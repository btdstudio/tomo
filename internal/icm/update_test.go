package icm

import (
	"os"
	"path/filepath"
	"slices"
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
	if found := gitFiles(t, dir); !slices.Equal(found, []string{".gitignore"}) {
		t.Fatalf("git files after update = %v, want only .gitignore", found)
	}
}

// A local workspace made by tomo v0.9.0 has no .gitignore, on disk or in its
// manifest. The first update creates it.
func TestUpdateLocalWorkspaceCreatesMissingGitignore(t *testing.T) {
	dir := t.TempDir()
	if _, err := Scaffold(dir, "none", Options{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	delete(m.Files, ".gitignore")
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir, "none")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(rep.Updated, ".gitignore") {
		t.Errorf(".gitignore not in Updated: %+v", rep)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err != nil {
		t.Fatalf(".gitignore not created: %v", err)
	}
}

func TestUpdateLocalWorkspaceSkipsEditedGitignore(t *testing.T) {
	dir := t.TempDir()
	if _, err := Scaffold(dir, "none", Options{}); err != nil {
		t.Fatal(err)
	}
	ignore := filepath.Join(dir, ".gitignore")
	edited := []byte("# mine\n.tomo/active\n")
	if err := os.WriteFile(ignore, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir, "none")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(rep.Skipped, ".gitignore") {
		t.Errorf(".gitignore not in Skipped: %+v", rep)
	}
	if data, _ := os.ReadFile(ignore); string(data) != string(edited) {
		t.Errorf(".gitignore overwritten: %q", data)
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
