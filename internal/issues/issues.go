// Package issues owns Tomo's local issue store: one <id>.md file per issue
// under the settings `issuesDir` (default `issues`), with a controlled
// YAML-subset frontmatter this package both writes and reads: scalar `id`,
// `status` (open|closed), `title`, and an inline `labels` list. No YAML parser,
// no third-party dependencies. The GitHub path intentionally lives nowhere here
// — `gh` passthrough is described by the command bodies.
//
// This is a faithful port of src/lib/issues.js.
package issues

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const DEFAULT_ISSUES_DIR = "issues"
const OPEN = "open"
const CLOSED = "closed"

// Issue is one local issue.
type Issue struct {
	ID     int      `json:"id"`
	Status string   `json:"status"`
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
	Body   string   `json:"body"`
	Path   string   `json:"path"`
}

// LocalIssuesDir resolves the store directory against the project root.
func LocalIssuesDir(root, issuesDir string) string {
	if issuesDir == "" {
		issuesDir = DEFAULT_ISSUES_DIR
	}
	return filepath.Join(root, issuesDir)
}

// IssuePath is the absolute path to a single issue file inside dir.
func IssuePath(dir string, id int) string {
	return filepath.Join(dir, fmt.Sprintf("%d.md", id))
}

var crlfRe = regexp.MustCompile(`\r\n`)

// ParseFrontmatter splits the controlled frontmatter off the raw file text,
// returning keyed-lowercase fm plus the body.
func ParseFrontmatter(raw string) (fm map[string]string, body string) {
	raw = crlfRe.ReplaceAllString(raw, "\n")
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return map[string]string{}, raw
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end < 2 {
		return map[string]string{}, raw
	}
	fm = map[string]string{}
	for _, line := range lines[1:end] {
		i := strings.Index(line, ":")
		if i == -1 {
			continue
		}
		fm[strings.ToLower(strings.TrimSpace(line[:i]))] = strings.TrimSpace(line[i+1:])
	}
	body = strings.Join(lines[end+1:], "\n")
	body = strings.TrimPrefix(body, "\n")
	body = strings.TrimSuffix(body, "\n")
	return fm, body
}

// ParseIssue parses frontmatter into a plain issue object.
func ParseIssue(raw string, fallbackID int) Issue {
	fm, body := ParseFrontmatter(raw)
	labels := []string{}
	if l := fm["labels"]; l != "" {
		l = strings.TrimPrefix(strings.TrimSuffix(l, "]"), "[")
		for _, s := range strings.Split(l, ",") {
			if s = strings.TrimSpace(s); s != "" {
				labels = append(labels, s)
			}
		}
	}
	id, _ := strconv.Atoi(fm["id"])
	if id == 0 {
		id = fallbackID
	}
	status := OPEN
	if fm["status"] == CLOSED {
		status = CLOSED
	}
	return Issue{
		ID:     id,
		Status: status,
		Title:  fm["title"],
		Labels: labels,
		Body:   body,
	}
}

var titleWsRe = regexp.MustCompile(`\s*\r?\n+\s*`)

// ToFrontmatter serializes an issue to the controlled frontmatter + body text.
func ToFrontmatter(issue Issue) string {
	labels := "[]"
	if len(issue.Labels) > 0 {
		labels = "[" + strings.Join(issue.Labels, ", ") + "]"
	}
	title := strings.TrimSpace(titleWsRe.ReplaceAllString(issue.Title, " "))
	body := issue.Body
	return fmt.Sprintf("---\nid: %d\nstatus: %s\ntitle: %s\nlabels: %s\n---\n\n%s", issue.ID, issue.Status, title, labels, body)
}

var issueFileRe = regexp.MustCompile(`^(\d+)\.md$`)

// NextId is the highest numeric id present in the store + 1 (1 when empty).
func NextId(root, issuesDir string) int {
	dir := LocalIssuesDir(root, issuesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}
	max := 0
	for _, e := range entries {
		if m := issueFileRe.FindStringSubmatch(e.Name()); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > max {
				max = n
			}
		}
	}
	return max + 1
}

// ListIssues returns all issues in the store, sorted by id, skipping
// non-<id>.md files.
func ListIssues(root, issuesDir string) []Issue {
	dir := LocalIssuesDir(root, issuesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []Issue{}
	}
	var out []Issue
	for _, e := range entries {
		m := issueFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		id, _ := strconv.Atoi(m[1])
		p := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		issue := ParseIssue(string(raw), id)
		issue.Path = p
		out = append(out, issue)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ReadIssue reads one issue, erroring if the store has no such issue.
func ReadIssue(root, issuesDir string, id int) (Issue, error) {
	p := IssuePath(LocalIssuesDir(root, issuesDir), id)
	raw, err := os.ReadFile(p)
	if err != nil {
		return Issue{}, fmt.Errorf("tomo: local issue %d not found at %s", id, p)
	}
	issue := ParseIssue(string(raw), id)
	issue.Path = p
	return issue, nil
}

// WriteIssue writes one issue file, refusing to clobber an existing one.
func WriteIssue(root, issuesDir string, issue Issue) (Issue, error) {
	dir := LocalIssuesDir(root, issuesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Issue{}, err
	}
	p := IssuePath(dir, issue.ID)
	if _, err := os.Stat(p); err == nil {
		return Issue{}, fmt.Errorf("tomo: local issue %d already exists at %s; not overwriting", issue.ID, p)
	}
	if err := os.WriteFile(p, []byte(ToFrontmatter(issue)+"\n"), 0o644); err != nil {
		return Issue{}, err
	}
	raw, _ := os.ReadFile(p)
	written := ParseIssue(string(raw), issue.ID)
	written.Path = p
	return written, nil
}

// NewIssueOptions is the payload for WriteNewIssue.
type NewIssueOptions struct {
	Title  string
	Labels []string
	Body   string
}

// WriteNewIssue creates a new issue: next sequential id, status open. Never
// overwrites.
func WriteNewIssue(root, issuesDir string, opts NewIssueOptions) (Issue, error) {
	id := NextId(root, issuesDir)
	return WriteIssue(root, issuesDir, Issue{
		ID:     id,
		Status: OPEN,
		Title:  opts.Title,
		Labels: opts.Labels,
		Body:   opts.Body,
	})
}
