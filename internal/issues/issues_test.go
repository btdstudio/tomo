package issues

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/btdstudio/kiso/ticket"
)

func TestNewCreatesValidNativeTicket(t *testing.T) {
	root := t.TempDir()
	created, err := New(root, "", NewOptions{Title: "Build: the thing", Repo: "app", Labels: []string{"docs", "ui"}, Body: "Wire up the store."})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "N-0001" || created.Status != ticket.StatusDraft || created.Type != ticket.Task {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasSuffix(created.Path, filepath.Join("00_issues", "N-0001.md")) {
		t.Fatalf("path = %q", created.Path)
	}
	read, err := Read(root, "", "N-0001")
	if err != nil {
		t.Fatal(err)
	}
	if read.Title != "Build: the thing" || !reflect.DeepEqual(read.Labels, []string{"docs", "ui"}) || !strings.Contains(read.Body, "Wire up the store.") {
		t.Fatalf("read = %+v", read)
	}
	if problems := ticket.ValidateFile(read); len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

func TestIDsSkipImportedTickets(t *testing.T) {
	root := t.TempDir()
	imported, err := ticket.New(ticket.Ticket{ID: "S-0014", Type: ticket.Story, Title: "x", Repo: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Import(root, "", imported, ticket.Origin{Workspace: "w", Path: "p"}); err != nil {
		t.Fatal(err)
	}
	created, err := New(root, "", NewOptions{Title: "y", Repo: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "N-0015" {
		t.Fatalf("id = %s", created.ID)
	}
}

func TestReadAcceptsANumberAndReportsMissing(t *testing.T) {
	root := t.TempDir()
	if _, err := New(root, "", NewOptions{Title: "x", Repo: "app"}); err != nil {
		t.Fatal(err)
	}
	if got, err := Read(root, "", "1"); err != nil || got.ID != "N-0001" {
		t.Fatalf("got = %+v, %v", got, err)
	}
	if _, err := Read(root, "", "N-0009"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestCustomDirAndEmptyStore(t *testing.T) {
	root := t.TempDir()
	if all, err := List(root, "custom"); err != nil || len(all) != 0 {
		t.Fatalf("empty = %v, %v", all, err)
	}
	created, err := New(root, "custom", NewOptions{Title: "x", Repo: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(created.Path, filepath.Join("custom", "N-0001.md")) {
		t.Fatalf("path = %q", created.Path)
	}
}

func TestListReportsUnreadableFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Dir(root, ""), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(root, ""), "S-0001.md"), []byte("not a ticket"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := List(root, ""); err == nil {
		t.Fatal("want an error")
	}
}

func TestImportRefreshesSameOriginAndRefusesAnother(t *testing.T) {
	root := t.TempDir()
	mk := func(title string) *ticket.Ticket {
		tk, err := ticket.New(ticket.Ticket{ID: "S-0001", Type: ticket.Story, Title: title, Repo: "app"})
		if err != nil {
			t.Fatal(err)
		}
		return tk
	}
	origin := ticket.Origin{Workspace: "git@github.com:o/plan.git", Path: "specs/1-x/tickets/S-0001.md"}
	if _, err := Import(root, "", mk("first"), origin); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(root, "", mk("second"), origin); err != nil {
		t.Fatal(err)
	}
	got, err := Read(root, "", "S-0001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "second" || got.Origin != origin {
		t.Fatalf("got = %+v", got)
	}
	if _, err := Import(root, "", mk("third"), ticket.Origin{Workspace: "elsewhere", Path: "p"}); err == nil || !strings.Contains(err.Error(), "not replacing") {
		t.Fatalf("err = %v", err)
	}
	if got, _ := Read(root, "", "S-0001"); got.Title != "second" {
		t.Fatalf("overwritten: %+v", got)
	}
}

func TestMigrateConvertsOldIssues(t *testing.T) {
	root := t.TempDir()
	dir := Dir(root, "")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"1.md":     "---\nid: 1\nstatus: open\ntitle: Fix: login\nlabels: [docs, ui]\n---\n\nThe body.\n",
		"2.md":     "---\nid: 2\nstatus: closed\ntitle: Done\nlabels: []\n---\n\n",
		"12.md":    "No frontmatter at all\n",
		".gitkeep": "",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !HasOldIssues(root, "") {
		t.Fatal("HasOldIssues = false")
	}
	moved, err := Migrate(root, "", "app")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"1.md": "N-0001.md", "2.md": "N-0002.md", "12.md": "N-0012.md"}
	if !reflect.DeepEqual(moved, want) {
		t.Fatalf("moved = %v", moved)
	}
	if HasOldIssues(root, "") {
		t.Fatal("old files remain")
	}
	all, err := List(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("all = %d", len(all))
	}
	for _, tk := range all {
		if problems := ticket.ValidateFile(tk); len(problems) != 0 {
			t.Errorf("%s: %v", tk.ID, problems)
		}
	}
	first := all[0]
	if first.Title != "Fix: login" || first.Status != ticket.StatusDraft || !reflect.DeepEqual(first.Labels, []string{"docs", "ui"}) || !strings.Contains(first.Body, "The body.") || first.Repo != "app" {
		t.Fatalf("first = %+v", first)
	}
	if all[1].Status != ticket.StatusClosed {
		t.Fatalf("second = %+v", all[1])
	}
	if all[2].Title != "Issue 12" || !strings.Contains(all[2].Body, "No frontmatter") {
		t.Fatalf("third = %+v", all[2])
	}
	if again, err := Migrate(root, "", "app"); err != nil || len(again) != 0 {
		t.Fatalf("second migration = %v, %v", again, err)
	}
}
