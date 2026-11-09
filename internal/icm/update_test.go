package icm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/btdstudio/kiso/scaffold"
)

func TestUpdatePreservesUserEdits(t *testing.T) {
	dir := t.TempDir()
	if _, err := Scaffold(dir, "", Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	ag := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(ag, []byte("user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(ag); string(data) != "user edit\n" {
		t.Errorf("user edit clobbered: %q", data)
	}
	if !contains(rep.Skipped, "AGENTS.md") {
		t.Errorf("AGENTS.md not in skipped: %v", rep.Skipped)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestUpdateLocalWorkspaceSkipsGitFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := Scaffold(dir, "none", Options{}); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir, "none")
	if err != nil {
		t.Fatal(err)
	}
	for _, list := range [][]string{rep.Updated, rep.Skipped} {
		for _, rel := range list {
			if scaffold.IsGitFile(rel) {
				t.Errorf("update reported git file %s", rel)
			}
		}
	}
	if found := gitFiles(t, dir); len(found) != 0 {
		t.Fatalf("update wrote git files: %v", found)
	}
}

func TestUpdateGitWorkspaceRestoresGitignore(t *testing.T) {
	dir := t.TempDir()
	if _, err := Scaffold(dir, "", Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	ignore := filepath.Join(dir, ".gitignore")
	if err := os.Remove(ignore); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(dir, "git"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ignore); err != nil {
		t.Fatalf(".gitignore not restored: %v", err)
	}
}
