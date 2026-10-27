package cli

import (
	"testing"

	"github.com/btdstudio/tomo/internal/icm"
	"github.com/btdstudio/tomo/internal/issues"
)

func TestRunNoArgs(t *testing.T) {
	if Run(nil) != 1 {
		t.Error("expected exit 1 on no args")
	}
}

func TestRunIssuesNewAndList(t *testing.T) {
	dir := t.TempDir()
	if _, err := icm.Scaffold(dir, icm.Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	if got := runIssues(dir, []string{"new", "--title", "Fix login"}); got != 0 {
		t.Fatalf("runIssues new exit %d", got)
	}
	all := issues.ListIssues(dir, "00_issues")
	if len(all) != 1 || all[0].ID != 1 || all[0].Title != "Fix login" {
		t.Fatalf("issues = %+v", all)
	}
}
