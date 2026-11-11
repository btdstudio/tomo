package icm

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/btdstudio/kiso/scaffold"
)

func TestScaffoldWritesTreeAndGitInit(t *testing.T) {
	dir := t.TempDir()
	rep, err := Scaffold(dir, "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.GitInit {
		t.Error("expected git init on empty dir")
	}
	for _, rel := range []string{
		"AGENTS.md",
		"CLAUDE.md",
		".gitignore",
		".tomo/settings.json",
		"00_issues/.gitkeep",
		"01_planning/personas/analyst.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing %q: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".tomo", "manifest.json")); err != nil {
		t.Errorf("missing manifest: %v", err)
	}
}

func TestScaffoldIdempotentSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	if _, err := Scaffold(dir, "", Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	ag := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(ag, []byte("user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(dir, "", Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(ag)
	if string(data) != "user edit\n" {
		t.Errorf("AGENTS.md clobbered: %q", data)
	}
}

// gitFiles lists the git files under dir, as slash paths relative to it.
func gitFiles(t *testing.T, dir string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() && d.Name() == ".git" {
			found = append(found, rel)
			return filepath.SkipDir
		}
		if scaffold.IsGitFile(rel) {
			found = append(found, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestScaffoldLocalWorkspace(t *testing.T) {
	dir := t.TempDir()
	if _, err := Scaffold(dir, "none", Options{}); err != nil {
		t.Fatal(err)
	}
	// A local workspace keeps only .gitignore, so local files stay out of git.
	if found := gitFiles(t, dir); !slices.Equal(found, []string{".gitignore"}) {
		t.Fatalf("git files in a local workspace = %v, want only .gitignore", found)
	}
	for _, kept := range []string{"00_issues", "01_planning/specs", "03_testing/reports", "04_refactoring/reviews"} {
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(kept))); err != nil || !info.IsDir() {
			t.Errorf("%s missing: %v", kept, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, ".tomo", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(data, &s); err != nil || s["vcs"] != "none" {
		t.Fatalf("settings = %s, %v", data, err)
	}
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	for rel := range m.Files {
		if scaffold.IsGitFile(rel) && rel != ".gitignore" {
			t.Errorf("manifest lists git file %s", rel)
		}
	}
	if _, ok := m.Files[".gitignore"]; !ok {
		t.Errorf("manifest doesn't list .gitignore")
	}
}

// AC12: the scaffolded AGENTS.md makes workspace commits conditional on git and
// keeps the target-repo commit text.
func TestScaffoldAgentsWordingIsNeutralAboutCommits(t *testing.T) {
	data, err := fs.ReadFile(engine.FS, "AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"In a git workspace, `tomo change new`, `gate`, `reopen`",
		"With `vcs: none`\n  they only write the files; nothing is committed.",
		"`tomo commit` and `tomo pr` act on the target repo, which is always git.",
		"commit what's staged in the target repo",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("AGENTS.md missing %q", want)
		}
	}
	for _, banned := range []string{"(committed)", "Gates commit this workspace's", ".gitignore            # git workspaces only"} {
		if strings.Contains(text, banned) {
			t.Errorf("AGENTS.md still says %q", banned)
		}
	}
}

// designTemplate is the scaffold's design template.
const designTemplate = "01_planning/templates/design.md"

// TestDesignTemplateHasOptionalDiagram pins the optional Diagrams section: where it
// sits, that it says it's optional, and that its Mermaid renders as written.
func TestDesignTemplateHasOptionalDiagram(t *testing.T) {
	data, err := fs.ReadFile(engine.FS, designTemplate)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	var headings []string
	diagrams := -1
	for i, line := range lines {
		if h, ok := strings.CutPrefix(line, "## "); ok {
			headings = append(headings, h)
			if h == "Diagrams" {
				diagrams = i
			}
		}
	}
	want := []string{"Overview", "Context", "Design decisions", "Architecture / components", "Data flow / behavior", "Diagrams", "Interfaces", "Out of scope (deferred)", "Open items / risks", "Notes"}
	if !slices.Equal(headings, want) {
		t.Fatalf("headings = %q, want %q", headings, want)
	}
	next := ""
	for _, line := range lines[diagrams+1:] {
		if strings.TrimSpace(line) != "" {
			next = line
			break
		}
	}
	if !strings.HasPrefix(next, "<Optional") {
		t.Errorf("the Diagrams section doesn't open with an <Optional …> line: %q", next)
	}
	if n := strings.Count(string(data), "```mermaid"); n != 1 {
		t.Fatalf("%d mermaid fences, want 1", n)
	}
	_, rest, _ := strings.Cut(string(data), "```mermaid\n")
	fence, _, ok := strings.Cut(rest, "```")
	if !ok {
		t.Fatal("the mermaid fence isn't closed")
	}
	if first, _, _ := strings.Cut(fence, "\n"); first != "sequenceDiagram" {
		t.Errorf("the fence starts with %q, want sequenceDiagram", first)
	}
	if !strings.Contains(fence, "->>") {
		t.Error("the diagram has no message arrow")
	}
	if bare := strings.NewReplacer("-->>", "", "->>", "").Replace(fence); strings.ContainsAny(bare, "<>") {
		t.Errorf("the diagram has a <…> placeholder, which isn't valid Mermaid:\n%s", fence)
	}
}
