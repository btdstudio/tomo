package icm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdatePreservesUserEdits(t *testing.T) {
	dir := t.TempDir()
	if _, err := Scaffold(dir, Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	ag := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(ag, []byte("user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Update(dir)
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
