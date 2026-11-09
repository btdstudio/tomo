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
	for _, banned := range []string{"(committed)", "Gates commit this workspace's"} {
		if strings.Contains(text, banned) {
			t.Errorf("AGENTS.md still says %q", banned)
		}
	}
}
