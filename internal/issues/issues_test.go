package issues

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestWriteNewIssueCreatesControlledFrontmatter(t *testing.T) {
	root := t.TempDir()
	created, err := WriteNewIssue(root, "", NewIssueOptions{Title: "Build the thing", Labels: []string{"docs", "ui"}, Body: "Wire up the store."})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != 1 || created.Status != "open" || created.Title != "Build the thing" ||
		!reflect.DeepEqual(created.Labels, []string{"docs", "ui"}) || created.Body != "Wire up the store." {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasSuffix(created.Path, filepath.Join("issues", "1.md")) {
		t.Fatalf("path = %q", created.Path)
	}
	saved, _ := os.ReadFile(created.Path)
	want := "---\nid: 1\nstatus: open\ntitle: Build the thing\nlabels: [docs, ui]\n---\n\nWire up the store.\n"
	if string(saved) != want {
		t.Fatalf("saved = %q, want %q", saved, want)
	}
}

func TestIDsAreSequential(t *testing.T) {
	root := t.TempDir()
	a, _ := WriteNewIssue(root, "", NewIssueOptions{Title: "a"})
	b, _ := WriteNewIssue(root, "", NewIssueOptions{Title: "b"})
	c, _ := WriteNewIssue(root, "", NewIssueOptions{Title: "c"})
	if a.ID != 1 || b.ID != 2 || c.ID != 3 {
		t.Fatalf("ids = %d,%d,%d", a.ID, b.ID, c.ID)
	}
}

func TestListIssuesSortedSkippingNonNumeric(t *testing.T) {
	root := t.TempDir()
	WriteNewIssue(root, "", NewIssueOptions{Title: "first"})
	WriteNewIssue(root, "", NewIssueOptions{Title: "second"})
	os.WriteFile(filepath.Join(root, "issues", "README.md"), []byte("not an issue\n"), 0o644)
	list := ListIssues(root, "")
	if len(list) != 2 || list[0].ID != 1 || list[1].ID != 2 || list[0].Title != "first" || list[1].Title != "second" {
		t.Fatalf("list = %+v", list)
	}
}

func TestReadIssueAndMissing(t *testing.T) {
	root := t.TempDir()
	created, _ := WriteNewIssue(root, "", NewIssueOptions{Title: "T", Body: "Body"})
	got, err := ReadIssue(root, "", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "T" || got.Body != "Body" {
		t.Fatalf("got = %+v", got)
	}
	if _, err := ReadIssue(root, "", 99); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestIssuesDirRelocatesStore(t *testing.T) {
	root := t.TempDir()
	created, _ := WriteNewIssue(root, "tickets", NewIssueOptions{Title: "x"})
	if !strings.HasSuffix(created.Path, filepath.Join("tickets", "1.md")) {
		t.Fatalf("path = %q", created.Path)
	}
	if LocalIssuesDir(root, "tickets") != filepath.Join(root, "tickets") {
		t.Fatalf("local dir = %q", LocalIssuesDir(root, "tickets"))
	}
}

func TestEmptyStore(t *testing.T) {
	root := t.TempDir()
	if len(ListIssues(root, "")) != 0 || NextId(root, "") != 1 {
		t.Fatalf("empty store list/next = %v / %d", ListIssues(root, ""), NextId(root, ""))
	}
}

func TestWriteIssueRefusesToClobber(t *testing.T) {
	root := t.TempDir()
	dir := LocalIssuesDir(root, "")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "5.md"), []byte("existing content"), 0o644)
	_, err := WriteIssue(root, "", Issue{ID: 5, Status: OPEN, Title: "x"})
	if err == nil || !strings.Contains(err.Error(), "not overwriting") {
		t.Fatalf("want not overwriting, got %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "5.md")); string(b) != "existing content" {
		t.Fatalf("5.md clobbered")
	}
}

func TestParseIssueRoundTripsClosedAndEmptyLabels(t *testing.T) {
	root := t.TempDir()
	created, _ := WriteNewIssue(root, "", NewIssueOptions{Title: "long", Labels: []string{}})
	raw := "---\nid: " + strconv.Itoa(created.ID) + "\nstatus: closed\ntitle: long\nlabels: []\n---\n"
	os.WriteFile(created.Path, []byte(raw), 0o644)
	got, err := ReadIssue(root, "", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "closed" || !reflect.DeepEqual(got.Labels, []string{}) {
		t.Fatalf("got status/labels = %q / %v", got.Status, got.Labels)
	}
}
