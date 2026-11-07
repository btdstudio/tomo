package cli

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/kiso/repos"
	"github.com/btdstudio/tomo/internal/icm"
	"github.com/btdstudio/tomo/internal/issues"
	"github.com/btdstudio/tomo/internal/state"
)

// workspace returns a scaffolded root with one change at requirements.
func workspace(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if _, err := icm.Scaffold(root, icm.Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	if code := runChange(root, []string{"new", "Auth flow"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	return root
}

func readLog(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(state.LogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func gitCmd(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func TestChangeNewCreatesIssueAndSeeds(t *testing.T) {
	root := workspace(t)
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != "1-auth-flow" || st.Phase != "requirements" || st.Status != "draft" || st.Sources[0] != "00_issues/1.md" {
		t.Fatalf("st = %+v", st)
	}
	if got := issues.ListIssues(root, "00_issues"); len(got) != 1 || got[0].Title != "Auth flow" {
		t.Fatalf("issues = %+v", got)
	}
}

func TestChangeNewFromExistingIssue(t *testing.T) {
	root := workspace(t)
	if code := runChange(root, []string{"new", "Second", "--issue", "1"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	if got := issues.ListIssues(root, "00_issues"); len(got) != 1 {
		t.Fatalf("made another issue: %+v", got)
	}
	if code := runChange(root, []string{"new", "Third", "--issue", "99"}); code != 1 {
		t.Fatalf("missing issue = %d, want 1", code)
	}
	if _, err := os.Stat(state.ChangeDir(root, "3-third", "")); err == nil {
		t.Fatal("refused change created a folder")
	}
}

func TestChangeNewLeavesNothingBehindOnCorruptState(t *testing.T) {
	root := workspace(t)
	if err := os.WriteFile(state.StatePath(root), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runChange(root, []string{"new", "Second"}); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if _, err := os.Stat(filepath.Join(root, "00_issues", "2.md")); err == nil {
		t.Fatal("wrote an orphan issue")
	}
}

func TestGateEnforcesLoopOrder(t *testing.T) {
	root := workspace(t)
	if code := runGate(root, []string{"design", "approved"}); code != 1 {
		t.Fatalf("out of order = %d, want 1", code)
	}
	for _, c := range [][]string{
		{"requirements", "approved"}, {"design", "approved"}, {"plan", "approved"},
		{"implement", "in-progress"}, {"implement", "complete"},
	} {
		if code := runGate(root, c); code != 0 {
			t.Fatalf("gate %v = %d", c, code)
		}
	}
	if log := readLog(t, root); strings.Contains(log, "override") {
		t.Fatalf("in-order run logged an override: %s", log)
	}
	if code := runGate(root, []string{"implement", "approved"}); code != 1 {
		t.Fatalf("gated status on run phase = %d, want 1", code)
	}
}

func TestGateForceOverridesAndLogs(t *testing.T) {
	root := workspace(t)
	if code := runGate(root, []string{"plan", "approved", "--force"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if log := readLog(t, root); !strings.Contains(log, "override: tomo: plan requires requirements") {
		t.Fatalf("override not logged: %s", log)
	}
}

func TestReopenAndClose(t *testing.T) {
	root := workspace(t)
	for _, phase := range []string{"requirements", "design"} {
		if code := runGate(root, []string{phase, "approved"}); code != 0 {
			t.Fatalf("gate %s = %d", phase, code)
		}
	}
	if code := runReopen(root, []string{"requirements"}); code != 1 {
		t.Fatalf("reopen without reason = %d, want 1", code)
	}
	if code := runReopen(root, []string{"requirements", "--reason", "scope changed"}); code != 0 {
		t.Fatalf("reopen = %d", code)
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Gates["requirements"] != "draft" || st.Gates["design"] != "" {
		t.Fatalf("gates = %+v", st.Gates)
	}
	if code := runChange(root, []string{"close", "1-auth-flow", "--reason", "dropped"}); code != 0 {
		t.Fatalf("close = %d", code)
	}
	if code := runSwitch(root, []string{"1-auth-flow"}); code != 0 {
		t.Fatalf("switch = %d", code)
	}
	if code := runGate(root, []string{"requirements", "approved", "--force"}); code != 1 {
		t.Fatalf("gate on closed = %d, want 1", code)
	}
}

func TestCommitOnlyTakesWhatIsStaged(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@example.com")
	root := t.TempDir()
	gitCmd(t, root, "init", "-q")
	for _, f := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code := runCommit(root, []string{"-m", "feat: a"}); code != 1 {
		t.Fatalf("nothing staged = %d, want 1", code)
	}
	gitCmd(t, root, "add", "a.txt")
	if code := runCommit(root, []string{"-m", "feat: a"}); code != 0 {
		t.Fatalf("commit = %d", code)
	}
	if files := gitCmd(t, root, "show", "--name-only", "--format=", "HEAD"); strings.TrimSpace(files) != "a.txt" {
		t.Fatalf("commit took = %q", files)
	}
	if status := gitCmd(t, root, "status", "--porcelain"); !strings.Contains(status, "?? b.txt") {
		t.Fatalf("status = %q", status)
	}
}

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

func TestEnvPrintsFlagsForRegisteredRepos(t *testing.T) {
	root := workspace(t)
	target := t.TempDir()
	if code := repos.Run(root, "tomo", []string{"add", "app", "https://github.com/org/app", "--path", target}, os.Stdout, os.Stderr); code != 0 {
		t.Fatalf("repo add = %d", code)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	code := runEnv(root)
	os.Stdout = orig
	w.Close()
	out, _ := io.ReadAll(r)
	if code != 0 || strings.TrimSpace(string(out)) != "--add-dir "+target {
		t.Fatalf("code = %d, out = %q", code, out)
	}
}
