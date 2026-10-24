package render

import (
	"os"
	"path/filepath"
	"testing"
)

func goldenDir() string {
	return filepath.Join("..", "..", "testdata", "golden", "render")
}

// TestGoldenRenderByteIdentity proves the embedded store renders byte-identical
// to the captured golden fixtures (originally captured from the Node renderer).
func TestGoldenRenderByteIdentity(t *testing.T) {
	generated, err := GenerateAll()
	if err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(goldenDir())
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		seen[name] = true
		want, err := os.ReadFile(filepath.Join(goldenDir(), name))
		if err != nil {
			t.Fatal(err)
		}
		got, ok := generated[name]
		if !ok {
			t.Fatalf("%s: generated output missing for golden file", name)
		}
		if string(got) != string(want) {
			t.Fatalf("%s: byte mismatch\n--- got ---\n%q\n--- want ---\n%q", name, got, want)
		}
	}
	for name := range generated {
		if !seen[name] {
			t.Fatalf("%s: generated but not present in golden fixtures", name)
		}
	}
}
