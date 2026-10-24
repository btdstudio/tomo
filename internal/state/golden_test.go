package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestGoldenStateByteIdentity proves the Go state serialization is byte-identical
// to the Node implementation captured in testdata/golden/state.json.
func TestGoldenStateByteIdentity(t *testing.T) {
	orig := Now
	defer func() { Now = orig }()
	Now = func() time.Time { return time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC) }

	root := t.TempDir()
	os.MkdirAll(filepath.Join(SpecsDir(root), "1-sample"), 0o755)
	st := DefaultState()
	st.Change = "1-sample"
	if err := SetGate(&st, "requirements", "await-approval"); err != nil {
		t.Fatal(err)
	}
	st.Sources = []string{"prd.md"}
	if err := SaveState(root, &st); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(StatePath(root))
	want := readGolden(t, "state.json")
	if string(got) != string(want) {
		t.Fatalf("state.json byte mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestGoldenStateMultiByteIdentity(t *testing.T) {
	orig := Now
	defer func() { Now = orig }()
	Now = func() time.Time { return time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC) }

	root := t.TempDir()
	os.MkdirAll(filepath.Join(SpecsDir(root), "1-alpha"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root), "2-beta"), 0o755)

	a := DefaultState()
	a.Change = "1-alpha"
	if err := SetGate(&a, "requirements", "approved"); err != nil {
		t.Fatal(err)
	}
	a.Sources = []string{"prd.md"}
	if err := SaveState(root, &a); err != nil {
		t.Fatal(err)
	}
	b, err := SwitchTo(root, "2-beta")
	if err != nil {
		t.Fatal(err)
	}
	if err := SetGate(&b, "design", "draft"); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(root, &b); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(StatePath(root))
	want := readGolden(t, "state-multi.json")
	if string(got) != string(want) {
		t.Fatalf("state-multi.json byte mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
