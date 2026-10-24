package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/btdstudio/tomo/internal/issues"
	"github.com/btdstudio/tomo/internal/personas"
	"github.com/btdstudio/tomo/internal/state"
)

func TestStartChangeAndSetGate(t *testing.T) {
	root := t.TempDir()
	change, err := StartChange(root, "Local Issue Store!")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(change, "1-local-issue-store\n") {
		t.Fatalf("change = %q", change)
	}
	if err := SetGate(root, "requirements", "draft"); err != nil {
		t.Fatal(err)
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Change != "1-local-issue-store" || st.Phase != "requirements" || st.Status != "draft" {
		t.Fatalf("state = %+v", st)
	}
}

func TestPersonaJSON(t *testing.T) {
	root := t.TempDir()
	out, err := Persona(root, "requirements")
	if err != nil {
		t.Fatal(err)
	}
	var got personas.Resolved
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	want := personas.Resolved{ID: "analyst", Mode: "lens", Source: "default", File: ".tomo/personas/analyst.md"}
	if got != want {
		t.Fatalf("persona = %+v, want %+v", got, want)
	}
}

func TestIssueNewListShow(t *testing.T) {
	root := t.TempDir()
	out, err := IssueNew(root, "", issues.NewIssueOptions{Title: "First", Labels: []string{"docs"}, Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	var created issues.Issue
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != 1 || created.Title != "First" {
		t.Fatalf("created = %+v", created)
	}
	list, err := IssueList(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list, `"id": 1`) {
		t.Fatalf("list = %s", list)
	}
	show, err := IssueShow(root, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(show, `"title": "First"`) {
		t.Fatalf("show = %s", show)
	}
}

func TestSlug(t *testing.T) {
	if Slug("  Foo Bar  ") != "foo-bar\n" {
		t.Fatalf("slug = %q", Slug("  Foo Bar  "))
	}
}

func TestStep(t *testing.T) {
	root := t.TempDir()
	if _, err := StartChange(root, "My Feature"); err != nil {
		t.Fatal(err)
	}
	if err := SetGate(root, "requirements", "await-approval"); err != nil {
		t.Fatal(err)
	}

	out, err := Step(root, "requirements", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<!-- tomo context -->",
		"change: 1-my-feature",
		"phase: requirements (await-approval)",
		"persona: analyst (mode=lens, file=.tomo/personas/analyst.md)",
		"settings: gates=block issues=github issuesDir=issues",
		"# Tomo requirements (phase 1)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("step output missing %q\n%s", want, out)
		}
	}

	// --print dumps only the raw canonical body (no context block).
	raw, err := Step(root, "requirements", true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "<!-- tomo context -->") {
		t.Fatalf("--print must not include the context block")
	}
	if !strings.Contains(raw, "# Tomo requirements (phase 1)") {
		t.Fatalf("--print missing body")
	}

	// Unknown command errors.
	if _, err := Step(root, "bogus", false); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("want unknown command, got %v", err)
	}
}
