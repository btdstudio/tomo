package icm

import (
	"strings"
	"testing"
)

func TestAddUnknownType(t *testing.T) {
	dir := t.TempDir()
	if _, err := Scaffold(dir, Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(dir, "bogus", "demo"); err == nil || !strings.Contains(err.Error(), "unknown app type") {
		t.Fatalf("want unknown app type, got %v", err)
	}
}
