package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/btdstudio/kiso/scaffold"
)

// assertLocal checks dir is a local workspace: no git files, the kept folders,
// "vcs": "none" in its settings, and no git file in the manifest.
func assertLocal(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if d.Name() == ".git" || scaffold.IsGitFile(filepath.ToSlash(rel)) {
			t.Errorf("git file in a local workspace: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{"00_issues", "01_planning/specs", "03_testing/reports", "04_refactoring/reviews"} {
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(kept))); err != nil || !info.IsDir() {
			t.Errorf("%s missing: %v", kept, err)
		}
	}
	var s map[string]any
	data, err := os.ReadFile(filepath.Join(dir, ".tomo", "settings.json"))
	if err != nil || json.Unmarshal(data, &s) != nil || s["vcs"] != "none" {
		t.Errorf("settings = %s, %v", data, err)
	}
	var m struct{ Files map[string]string }
	data, err = os.ReadFile(filepath.Join(dir, ".tomo", "manifest.json"))
	if err != nil || json.Unmarshal(data, &m) != nil {
		t.Fatalf("manifest = %s, %v", data, err)
	}
	for rel := range m.Files {
		if scaffold.IsGitFile(rel) {
			t.Errorf("manifest lists git file %s", rel)
		}
	}
}

// AC5: tomo init --no-git makes a local workspace.
func TestInitNoGitMakesLocalWorkspace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := filepath.Join(t.TempDir(), "ws")
	if code := run([]string{"init", "--no-git", dir}); code != 0 {
		t.Fatalf("init = %d", code)
	}
	assertLocal(t, dir)
}

// AC6: a global vcs: none applies to init without --no-git.
func TestInitHonorsGlobalVCSNone(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if err := os.MkdirAll(filepath.Join(cfg, "tomo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "tomo", "settings.json"), []byte(`{"vcs": "none"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "ws")
	if code := run([]string{"init", dir}); code != 0 {
		t.Fatalf("init = %d", code)
	}
	assertLocal(t, dir)
}

// AC7: tomo update in a local workspace creates no git file.
func TestUpdateLocalWorkspaceCreatesNoGitFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := filepath.Join(t.TempDir(), "ws")
	if code := run([]string{"init", "--no-git", dir}); code != 0 {
		t.Fatalf("init = %d", code)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })
	if code := run([]string{"update"}); code != 0 {
		t.Fatalf("update = %d", code)
	}
	assertLocal(t, dir)
}
