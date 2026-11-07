package cli

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/kiso/ticket"
	"github.com/btdstudio/tomo/internal/icm"
	"github.com/btdstudio/tomo/internal/issues"
	"github.com/btdstudio/tomo/internal/state"
)

// workspace returns a scaffolded root with one change at intake.
func workspace(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if _, err := icm.Scaffold(root, icm.Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	if code := runRepo(root, []string{"add", "app", "https://github.com/org/app"}); code != 0 {
		t.Fatalf("repo add = %d", code)
	}
	if code := runChange(root, []string{"new", "Auth flow"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	return root
}

func listIssues(t *testing.T, root string) []*ticket.Ticket {
	t.Helper()
	all, err := issues.List(root, "00_issues")
	if err != nil {
		t.Fatal(err)
	}
	return all
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
	if st.Name != "1-auth-flow" || st.Phase != "intake" || st.Status != "draft" || st.Sources[0] != "00_issues/N-0001.md" {
		t.Fatalf("st = %+v", st)
	}
	if got := listIssues(t, root); len(got) != 1 || got[0].Title != "Auth flow" || got[0].Repo != "app" {
		t.Fatalf("issues = %+v", got)
	}
}

func TestChangeNewFromExistingIssue(t *testing.T) {
	root := workspace(t)
	if code := runChange(root, []string{"new", "Second", "--issue", "1"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	if got := listIssues(t, root); len(got) != 1 {
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
		{"intake", "approved"}, {"requirements", "approved"}, {"design", "approved"}, {"plan", "approved"},
		{"implement", "in-progress"}, {"implement", "complete"},
		{"test", "approved"}, {"review", "approved"}, {"security", "approved"},
		{"ship", "in-progress"}, {"ship", "complete"},
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

func TestFailedReviewLoopsBackToImplement(t *testing.T) {
	root := workspace(t)
	for _, c := range [][]string{
		{"intake", "approved"}, {"requirements", "approved"}, {"design", "approved"}, {"plan", "approved"},
		{"implement", "in-progress"},
	} {
		if code := runGate(root, c); code != 0 {
			t.Fatalf("gate %v = %d", c, code)
		}
	}
	if code := runGate(root, []string{"test", "approved"}); code != 1 {
		t.Fatalf("test during implement = %d, want 1", code)
	}
	for _, c := range [][]string{
		{"implement", "complete"}, {"test", "approved"}, {"review", "approved"},
		{"security", "skipped", "--reason", "docs only"},
	} {
		if code := runGate(root, c); code != 0 {
			t.Fatalf("gate %v = %d", c, code)
		}
	}
	if code := runReopen(root, []string{"implement", "--reason", "critical finding"}); code != 0 {
		t.Fatalf("reopen = %d", code)
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Gates["implement"] != "in-progress" || st.Gates["test"] != "" || st.Gates["review"] != "" || st.Gates["security"] != "skipped" || st.Gates["plan"] != "approved" {
		t.Fatalf("gates = %+v", st.Gates)
	}
	if code := runGate(root, []string{"review", "approved"}); code != 1 {
		t.Fatalf("review before re-testing = %d, want 1", code)
	}
}

func TestSecurityCanBeSkippedButNotOtherPhases(t *testing.T) {
	root := workspace(t)
	if code := runGate(root, []string{"review", "skipped", "--reason", "r", "--force"}); code != 1 {
		t.Fatalf("skip review = %d, want 1", code)
	}
	if code := runGate(root, []string{"security", "skipped", "--force"}); code != 1 {
		t.Fatalf("skip without reason = %d, want 1", code)
	}
}

func TestGateForceOverridesAndLogs(t *testing.T) {
	root := workspace(t)
	if code := runGate(root, []string{"plan", "approved", "--force"}); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if log := readLog(t, root); !strings.Contains(log, "override: tomo: plan requires intake") {
		t.Fatalf("override not logged: %s", log)
	}
}

func TestReopenAndClose(t *testing.T) {
	root := workspace(t)
	for _, phase := range []string{"intake", "requirements", "design"} {
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
	workspace := t.TempDir()
	root := t.TempDir()
	gitCmd(t, root, "init", "-q")
	if code := runCommit(workspace, []string{"-m", "x"}); code != 1 {
		t.Fatalf("no target = %d, want 1", code)
	}
	if code := runRepo(workspace, []string{"add", "app", "https://github.com/org/app", "--path", root}); code != 0 {
		t.Fatalf("repo add = %d", code)
	}
	for _, f := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code := runCommit(workspace, []string{"-m", "feat: a"}); code != 1 {
		t.Fatalf("nothing staged = %d, want 1", code)
	}
	gitCmd(t, root, "add", "a.txt")
	if code := runCommit(workspace, []string{"-m", "feat: a"}); code != 0 {
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	if _, err := icm.Scaffold(dir, icm.Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	if got := runIssues(dir, []string{"new", "--title", "Fix login"}); got != 1 {
		t.Fatalf("without a target = %d, want 1", got)
	}
	if code := runRepo(dir, []string{"add", "app", "https://github.com/org/app"}); code != 0 {
		t.Fatal("repo add")
	}
	if got := runIssues(dir, []string{"new", "--title", "Fix login"}); got != 0 {
		t.Fatalf("runIssues new exit %d", got)
	}
	all := listIssues(t, dir)
	if len(all) != 1 || all[0].ID != "N-0001" || all[0].Title != "Fix login" {
		t.Fatalf("issues = %+v", all)
	}
	if got := runIssues(dir, []string{"show", "1"}); got != 0 {
		t.Fatalf("show = %d", got)
	}
	if got := runIssues(dir, []string{"show", "N-0009"}); got != 1 {
		t.Fatalf("show missing = %d", got)
	}
}

func TestRepoAddKeepsWorkspaceToOneTarget(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if code := runEnv(root); code != 1 {
		t.Fatalf("env with no target = %d, want 1", code)
	}
	if code := runRepo(root, []string{"add", "app", "https://github.com/org/app"}); code != 0 {
		t.Fatalf("first = %d", code)
	}
	if code := runRepo(root, []string{"add", "other", "https://github.com/org/other"}); code != 1 {
		t.Fatalf("second = %d, want 1", code)
	}
}

func TestEnvPrintsFlagsForRegisteredRepos(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	target := t.TempDir()
	if code := runRepo(root, []string{"add", "app", "https://github.com/org/app", "--path", target}); code != 0 {
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

const plannedTicket = `---
schema: 1
id: S-0014                 # planned in zumen
type: story
title: Add release workflow
parent: F-0003
repo: app
depends_on: [S-0011]
change: 3-release
spec: specs/3-release/functional-spec.md
adrs: [ADR-0004]
status: ready
---

## Context
Why.

## Acceptance criteria
- [ ] AC1: Given a tag When pushed Then a release is cut
`

func writeTicket(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestChangeFromTicketSnapshotsAndSeeds(t *testing.T) {
	root := workspace(t)
	planning := t.TempDir()
	gitCmd(t, planning, "init", "-q")
	gitCmd(t, planning, "remote", "add", "origin", "https://x-access-token:secret@github.com/org/plan.git")
	path := writeTicket(t, planning, "S-0014.md", plannedTicket)
	if code := runChange(root, []string{"new", "--from", path}); code != 0 {
		t.Fatalf("change new --from = %d", code)
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != "2-add-release-workflow" || st.Phase != "intake" || st.Sources[0] != "00_issues/S-0014.md" {
		t.Fatalf("st = %+v", st)
	}
	snap, err := issues.Read(root, "00_issues", "S-0014")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Origin.Workspace != "https://github.com/org/plan.git" || snap.Origin.Path != "S-0014.md" {
		t.Fatalf("origin = %+v", snap.Origin)
	}
	raw, _ := os.ReadFile(snap.Path)
	if strings.Contains(string(raw), "secret") {
		t.Fatalf("committed a credential:\n%s", raw)
	}
	if !strings.Contains(string(raw), "# planned in zumen") {
		t.Fatalf("snapshot rewrote unchanged fields:\n%s", raw)
	}
	if problems := ticket.ValidateFile(snap); len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
	if got := branchName(snap); got != "s-0014-add-release-workflow" {
		t.Fatalf("branch = %q", got)
	}
}

func TestChangeFromRefusals(t *testing.T) {
	root := workspace(t)
	dir := t.TempDir()
	for name, c := range map[string]struct{ file, body string }{
		"wrong repo":     {"S-0014.md", strings.Replace(plannedTicket, "repo: app", "repo: other", 1)},
		"draft":          {"S-0014.md", strings.Replace(plannedTicket, "status: ready", "status: draft", 1)},
		"no criteria":    {"S-0014.md", strings.Replace(plannedTicket, "- [ ] AC1: Given a tag When pushed Then a release is cut\n", "", 1)},
		"not a ticket":   {"S-0014.md", "just text"},
		"name mismatch":  {"other.md", plannedTicket},
		"epic":           {"E-0001.md", strings.NewReplacer("id: S-0014", "id: E-0001", "type: story", "type: epic", "repo: app\n", "").Replace(plannedTicket)},
		"drifted frozen": {"S-0014.md", strings.Replace(plannedTicket, "status: ready", "status: published\nexternal:\n  github: o/r#1\npublished_hash: abc", 1)},
	} {
		path := writeTicket(t, dir, c.file, c.body)
		if code := runChange(root, []string{"new", "--from", path}); code != 1 {
			t.Errorf("%s: code = %d, want 1", name, code)
		}
		os.Remove(path)
	}
	if code := runChange(root, []string{"new", "--from", filepath.Join(dir, "missing.md")}); code != 1 {
		t.Errorf("missing file: want 1")
	}
	if got := listIssues(t, root); len(got) != 1 {
		t.Fatalf("a refused import left a ticket: %+v", got)
	}
	if _, err := os.Stat(state.ChangeDir(root, "2-add-release-workflow", "")); err == nil {
		t.Fatal("a refused import created a change")
	}
}

func TestChangeFromRefusesSameIDFromElsewhere(t *testing.T) {
	root := workspace(t)
	first := writeTicket(t, t.TempDir(), "S-0014.md", plannedTicket)
	if code := runChange(root, []string{"new", "--from", first}); code != 0 {
		t.Fatal("first import")
	}
	second := writeTicket(t, t.TempDir(), "S-0014.md", plannedTicket)
	if code := runChange(root, []string{"new", "--from", second}); code != 1 {
		t.Fatalf("second origin = %d, want 1", code)
	}
}

func TestPublishedTicketImportsFrozenAndPRLinksIt(t *testing.T) {
	root := workspace(t)
	tk, err := ticket.Parse([]byte(plannedTicket))
	if err != nil {
		t.Fatal(err)
	}
	if err := tk.MarkPublished("github", "org/app#7"); err != nil {
		t.Fatal(err)
	}
	path := writeTicket(t, t.TempDir(), "S-0014.md", string(tk.Bytes()))
	if code := runChange(root, []string{"new", "--from", path}); code != 0 {
		t.Fatal("import")
	}
	st, err := state.LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	snap := changeTicket(root, st)
	if snap == nil || !snap.Frozen() || snap.Drifted() {
		t.Fatalf("snapshot = %+v", snap)
	}
	body := prBody(snap)
	for _, want := range []string{"Implements S-0014: Add release workflow", "**github:** org/app#7", "**Spec:** `specs/3-release/functional-spec.md`", "ADR-0004"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
	args := prArgs(root, t.TempDir(), nil)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--title [S-0014] Add release workflow") || !strings.Contains(joined, "--body") {
		t.Fatalf("args = %v", args)
	}
	if args := prArgs(root, t.TempDir(), []string{"My title"}); !strings.Contains(strings.Join(args, " "), "--title My title") {
		t.Fatalf("args = %v", args)
	}
}

func TestStripCredentials(t *testing.T) {
	for in, want := range map[string]string{
		"https://user:token@github.com/o/r.git": "https://github.com/o/r.git",
		"https://token@github.com/o/r.git":      "https://github.com/o/r.git",
		"git@github.com:o/r.git":                "git@github.com:o/r.git",
		"":                                      "",
	} {
		if got := stripCredentials(in); got != want {
			t.Errorf("stripCredentials(%q) = %q", in, got)
		}
	}
}
