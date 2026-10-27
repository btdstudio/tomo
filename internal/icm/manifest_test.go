package icm

import "testing"

func TestBuildManifestIncludesScaffold(t *testing.T) {
	m, err := BuildManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"AGENTS.md",
		".gitignore",
		"00_issues/.gitkeep",
		"01_planning/personas/analyst.md",
		"src/apps/.gitkeep",
	} {
		if _, ok := m.Files[rel]; !ok {
			t.Errorf("manifest missing %q", rel)
		}
	}
}

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m, err := BuildManifest()
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	got, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != m.Version {
		t.Errorf("version = %q, want %q", got.Version, m.Version)
	}
	for rel, h := range m.Files {
		if got.Files[rel] != h {
			t.Errorf("hash %q = %q, want %q", rel, got.Files[rel], h)
		}
	}
}
