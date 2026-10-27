package icm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScaffoldWritesTreeAndGitInit(t *testing.T) {
	dir := t.TempDir()
	rep, err := Scaffold(dir, Options{})
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
		"src/apps/.gitkeep",
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
	if _, err := Scaffold(dir, Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	ag := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(ag, []byte("user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(dir, Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(ag)
	if string(data) != "user edit\n" {
		t.Errorf("AGENTS.md clobbered: %q", data)
	}
}
