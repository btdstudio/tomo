// Package issues owns the workspace's ticket store: one <ID>.md per ticket under
// 00_issues/, in the shared schema from kiso. A native ticket is created here, with
// an N- id and no origin. An imported one is a snapshot of a planned ticket.
package issues

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/btdstudio/kiso/ticket"
)

// DefaultDir is the store's location, relative to the workspace root.
const DefaultDir = "00_issues"

// Dir resolves the store directory against the workspace root.
func Dir(root, issuesDir string) string {
	if issuesDir == "" {
		issuesDir = DefaultDir
	}
	return filepath.Join(root, issuesDir)
}

// List returns every ticket in the store, sorted by id. An unreadable file is an
// error, so it can't look like a missing ticket.
func List(root, issuesDir string) ([]*ticket.Ticket, error) {
	tickets, problems, err := ticket.LoadDir(Dir(root, issuesDir))
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("tomo: unreadable ticket: %s", problems[0])
	}
	sort.Slice(tickets, func(i, j int) bool { return tickets[i].ID < tickets[j].ID })
	return tickets, nil
}

// NormalizeID accepts a full id, or the number of a native ticket: "1" and "N-0001"
// are the same ticket.
func NormalizeID(id string) string {
	if n, err := strconv.Atoi(id); err == nil {
		return fmt.Sprintf("%s-%04d", ticket.NativePrefix, n)
	}
	return id
}

// Read reads one ticket by id.
func Read(root, issuesDir, id string) (*ticket.Ticket, error) {
	path := filepath.Join(Dir(root, issuesDir), NormalizeID(id)+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tomo: ticket %s not found at %s", id, path)
	}
	t, err := ticket.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("tomo: %s: %v", path, err)
	}
	t.Path = path
	return t, nil
}

// NewOptions is the payload for New.
type NewOptions struct {
	Title  string
	Repo   string
	Labels []string
	Body   string
}

// New creates a native ticket with the next id. It starts as a draft.
func New(root, issuesDir string, opts NewOptions) (*ticket.Ticket, error) {
	all, err := List(root, issuesDir)
	if err != nil {
		return nil, err
	}
	id := ticket.NextNativeID(all)
	body := "\n" + strings.TrimRight(opts.Body, "\n") + "\n"
	t, err := ticket.New(ticket.Ticket{ID: id, Type: ticket.Task, Title: oneLine(opts.Title), Repo: opts.Repo, Labels: opts.Labels, Body: body})
	if err != nil {
		return nil, err
	}
	dir := Dir(root, issuesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	t.Path = filepath.Join(dir, id+".md")
	if err := os.WriteFile(t.Path, t.Bytes(), 0o644); err != nil {
		return nil, err
	}
	return t, nil
}

var lineBreakRe = regexp.MustCompile(`\s*\r?\n+\s*`)

func oneLine(s string) string { return strings.TrimSpace(lineBreakRe.ReplaceAllString(s, " ")) }

// Import copies a planned ticket into the store as a snapshot, recording its origin.
// The same ticket from the same origin refreshes the snapshot; the same id from
// anywhere else is refused.
func Import(root, issuesDir string, t *ticket.Ticket, origin ticket.Origin) (*ticket.Ticket, error) {
	dir := Dir(root, issuesDir)
	path := filepath.Join(dir, t.ID+".md")
	if existing, err := Read(root, issuesDir, t.ID); err == nil && existing.Origin != origin {
		return nil, fmt.Errorf("tomo: %s already exists here, from %q — not replacing it with one from %q", t.ID, existing.Origin.Workspace, origin.Workspace)
	}
	if err := t.SetOrigin(origin); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, t.Bytes(), 0o644); err != nil {
		return nil, err
	}
	t.Path = path
	return t, nil
}

var oldFileRe = regexp.MustCompile(`^(\d+)\.md$`)

// HasOldIssues reports whether the store still has issues in the old <n>.md format.
func HasOldIssues(root, issuesDir string) bool {
	entries, err := os.ReadDir(Dir(root, issuesDir))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if oldFileRe.MatchString(e.Name()) {
			return true
		}
	}
	return false
}

// Migrate converts old <n>.md issues to native tickets for the given repo: open
// becomes draft, because an old issue may have no acceptance criteria, and closed
// stays closed. It returns a map of old to new paths, relative to the store.
func Migrate(root, issuesDir, repo string) (map[string]string, error) {
	dir := Dir(root, issuesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	moved := map[string]string{}
	for _, e := range entries {
		m := oldFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		id := fmt.Sprintf("%s-%04d", ticket.NativePrefix, n)
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		fm, body := parseOld(string(data))
		status := ticket.StatusDraft
		if fm["status"] == "closed" {
			status = ticket.StatusClosed
		}
		title := fm["title"]
		if title == "" {
			title = "Issue " + m[1]
		}
		t, err := ticket.New(ticket.Ticket{ID: id, Type: ticket.Task, Title: title, Repo: repo, Status: status, Labels: oldLabels(fm["labels"]), Body: "\n" + body + "\n"})
		if err != nil {
			return nil, fmt.Errorf("tomo: migrating %s: %v", path, err)
		}
		target := filepath.Join(dir, id+".md")
		if _, err := os.Stat(target); err == nil {
			return nil, fmt.Errorf("tomo: migrating %s: %s already exists", path, target)
		}
		if err := os.WriteFile(target, t.Bytes(), 0o644); err != nil {
			return nil, err
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
		moved[e.Name()] = id + ".md"
	}
	return moved, nil
}

// parseOld reads the loose frontmatter of an old issue.
func parseOld(raw string) (map[string]string, string) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	fm := map[string]string{}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, strings.Trim(raw, "\n")
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return fm, strings.Trim(strings.Join(lines[i+1:], "\n"), "\n")
		}
		if key, value, ok := strings.Cut(lines[i], ":"); ok {
			fm[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
		}
	}
	return map[string]string{}, strings.Trim(raw, "\n")
}

func oldLabels(raw string) []string {
	raw = strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]")
	labels := []string{}
	for _, label := range strings.Split(raw, ",") {
		if label = strings.TrimSpace(label); label != "" {
			labels = append(labels, label)
		}
	}
	return labels
}
