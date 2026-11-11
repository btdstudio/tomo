package cli

import (
	"io"
	"net/url"
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
	if _, err := icm.Scaffold(root, "", icm.Options{NoGit: true}); err != nil {
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

// realGit skips unless git is on PATH, and isolates it from the developer's
// global and system config, so a user-level setting such as url.*.insteadOf can't
// change what a test sees. global is the file `git config --global` reads and
// writes; empty means none. HOME and XDG_CONFIG_HOME also cover git versions
// older than 2.32, which ignore GIT_CONFIG_GLOBAL.
func realGit(t *testing.T, global string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	if global == "" {
		global = os.DevNull
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
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
	if _, err := icm.Scaffold(dir, "", icm.Options{NoGit: true}); err != nil {
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

// rawPlannedTicket is a ready ticket without the readiness mark, as if set to
// ready by hand. Use plannedTicket for one readied the way zumen does it.
const rawPlannedTicket = `---
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

// plannedTicket is rawPlannedTicket readied with kiso's MarkReady, so its
// ready_hash matches its content.
var plannedTicket = readied(rawPlannedTicket)

// readied returns body with its readiness mark set by MarkReady. It panics on a
// fixture kiso can't parse or ready, which is a bug in the test.
func readied(body string) string {
	tk, err := ticket.Parse([]byte(body))
	if err != nil {
		panic(err)
	}
	if err := tk.MarkReady(); err != nil {
		panic(err)
	}
	return string(tk.Bytes())
}

func writeTicket(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestChangeFromTicketSnapshotsAndSeeds(t *testing.T) {
	realGit(t, "")
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

// TestChangeFromOrigin pins what an import records as origin.workspace: the
// planning repo's configured remote, without credentials and untouched by
// insteadOf rewrites, or the folder's name when there is no remote to record.
func TestChangeFromOrigin(t *testing.T) {
	const plan = "https://github.com/org/plan.git"
	folder := func(dir string) string { return filepath.Base(dir) }
	for _, c := range []struct {
		name   string
		global bool // give git a writable global config
		setup  func(t *testing.T, dir string)
		want   func(dir string) string
	}{
		{"local insteadOf", false, func(t *testing.T, dir string) {
			gitCmd(t, dir, "init", "-q")
			gitCmd(t, dir, "remote", "add", "origin", plan)
			gitCmd(t, dir, "config", "url.git@github.com:.insteadOf", "https://github.com/")
		}, func(string) string { return plan }},
		{"global insteadOf", true, func(t *testing.T, dir string) {
			gitCmd(t, dir, "init", "-q")
			gitCmd(t, dir, "remote", "add", "origin", plan)
			gitCmd(t, dir, "config", "--global", "url.git@github.com:.insteadOf", "https://github.com/")
		}, func(string) string { return plan }},
		{"token", false, func(t *testing.T, dir string) {
			gitCmd(t, dir, "init", "-q")
			gitCmd(t, dir, "remote", "add", "origin", "https://token@github.com/org/plan.git")
		}, func(string) string { return plan }},
		{"no origin", false, func(t *testing.T, dir string) {
			gitCmd(t, dir, "init", "-q")
		}, folder},
		{"not a repo", false, func(t *testing.T, dir string) {
			t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
		}, folder},
		{"git missing", false, func(t *testing.T, dir string) {
			t.Setenv("PATH", t.TempDir())
		}, folder},
		{"scp-like", false, func(t *testing.T, dir string) {
			gitCmd(t, dir, "init", "-q")
			gitCmd(t, dir, "remote", "add", "origin", "git@github.com:org/plan.git")
		}, func(string) string { return "git@github.com:org/plan.git" }},
	} {
		t.Run(c.name, func(t *testing.T) {
			global := ""
			if c.global {
				global = filepath.Join(t.TempDir(), "gitconfig")
			}
			realGit(t, global)
			root := workspace(t)
			planning := t.TempDir()
			path := writeTicket(t, planning, "S-0014.md", plannedTicket)
			c.setup(t, planning)
			if code := runChange(root, []string{"new", "--from", path}); code != 0 {
				t.Fatalf("change new --from = %d", code)
			}
			snap, err := issues.Read(root, "00_issues", "S-0014")
			if err != nil {
				t.Fatal(err)
			}
			if want := c.want(planning); snap.Origin.Workspace != want || snap.Origin.Path != "S-0014.md" {
				t.Fatalf("origin = %+v, want workspace %q", snap.Origin, want)
			}
		})
	}
}

// TestChangeFromRefusals checks each refusal is for its own reason. Cases edit
// rawPlannedTicket and ready the result where it stays ready, so an edit isn't
// refused as "changed since readied" instead.
func TestChangeFromRefusals(t *testing.T) {
	root := workspace(t)
	dir := t.TempDir()
	raw := rawPlannedTicket
	for name, c := range map[string]struct{ file, body, reason string }{
		"wrong repo":     {"S-0014.md", readied(strings.Replace(raw, "repo: app", "repo: other", 1)), `lands in repo "other"`},
		"draft":          {"S-0014.md", strings.Replace(raw, "status: ready", "status: draft", 1), "only a ready or published ticket"},
		"no criteria":    {"S-0014.md", readied(strings.Replace(raw, "- [ ] AC1: Given a tag When pushed Then a release is cut\n", "", 1)), "requires at least one acceptance criterion"},
		"not a ticket":   {"S-0014.md", "just text", "must start with"},
		"name mismatch":  {"other.md", plannedTicket, "doesn't match id"},
		"epic":           {"E-0001.md", readied(strings.NewReplacer("id: S-0014", "id: E-0001", "type: story", "type: epic", "repo: app\n", "").Replace(raw)), `lands in repo ""`},
		"drifted frozen": {"S-0014.md", strings.Replace(raw, "status: ready", "status: published\nexternal:\n  github: o/r#1\npublished_hash: abc", 1), "changed since it was published"},
	} {
		path := writeTicket(t, dir, c.file, c.body)
		code, _, errOut := capture(t, func() int { return runChange(root, []string{"new", "--from", path}) })
		if code != 1 || !strings.Contains(errOut, c.reason) {
			t.Errorf("%s: code = %d, stderr = %q, want 1 and %q", name, code, errOut, c.reason)
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

// TestChangeFromRefusesUnreadiedTicket pins ADR-0001: a ready ticket imports only
// with a readiness mark that still matches its content.
func TestChangeFromRefusesUnreadiedTicket(t *testing.T) {
	for _, c := range []struct{ name, body, reason string }{
		{"set ready by hand", rawPlannedTicket, "not readied through the CLI"},
		{"body edited", strings.Replace(plannedTicket, "Why.", "Why not.", 1), "body changed since readied"},
		{"frontmatter edited", strings.Replace(plannedTicket, "title: Add release workflow", "title: Add a release workflow", 1), "frontmatter changed since readied"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.body == plannedTicket {
				t.Fatal("the edit didn't apply")
			}
			root := workspace(t)
			path := writeTicket(t, t.TempDir(), "S-0014.md", c.body)
			code, _, errOut := capture(t, func() int { return runChange(root, []string{"new", "--from", path}) })
			if code != 1 || !strings.Contains(errOut, c.reason) {
				t.Fatalf("code = %d, stderr = %q, want 1 and %q", code, errOut, c.reason)
			}
			if _, err := os.Stat(filepath.Join(root, "00_issues", "S-0014.md")); err == nil {
				t.Fatal("a refused import left a snapshot")
			}
			if _, err := os.Stat(state.ChangeDir(root, "2-add-release-workflow", "")); err == nil {
				t.Fatal("a refused import created a change")
			}
		})
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

// stripCases are stripCredentials' inputs, expected outputs, and the secrets each
// input carries, none of which may survive.
var stripCases = []struct {
	in, want string
	secrets  []string
}{
	// ssh keeps the username and loses the password (AC1, AC2).
	{"ssh://user:pw@host/org/plan.git", "ssh://user@host/org/plan.git", []string{"pw"}},
	{"git+ssh://user:pw@host/org/plan.git", "git+ssh://user@host/org/plan.git", []string{"pw"}},
	{"ssh+git://user:pw@host/org/plan.git", "ssh+git://user@host/org/plan.git", []string{"pw"}},
	{"SSH://user:pw@host/org/plan.git", "SSH://user@host/org/plan.git", []string{"pw"}},
	{"ssh://user:@host:2222/org/plan.git", "ssh://user@host:2222/org/plan.git", nil},
	{"ssh://:pw@host/org/plan.git", "ssh://host/org/plan.git", []string{"pw", "@"}},
	{"ssh://git@host/org/plan.git", "ssh://git@host/org/plan.git", nil},
	{"https://github.com/o/a@b.git", "https://github.com/o/a@b.git", nil},
	// Other schemes lose all userinfo (AC3).
	{"ftps://user:pw@host/org/plan.git", "ftps://host/org/plan.git", []string{"pw", "user"}},
	{"https://user:token@github.com/o/r.git", "https://github.com/o/r.git", []string{"token", "user"}},
	{"https://token@github.com/o/r.git", "https://github.com/o/r.git", []string{"token"}},
	{"https://@github.com/o/r.git", "https://github.com/o/r.git", nil},
	{"https://a@b:c@github.com/o/r.git", "https://github.com/o/r.git", []string{"a@b", "c@"}},
	// The query and fragment go, whatever the scheme (AC4).
	{"https://github.com/org/plan.git?token=abc#x", "https://github.com/org/plan.git", []string{"abc", "#x"}},
	{"https://github.com/org/plan.git?", "https://github.com/org/plan.git", nil},
	{"https://github.com/org/plan.git#", "https://github.com/org/plan.git", nil},
	{"https://github.com/org/plan.git#a?b", "https://github.com/org/plan.git", []string{"a?b"}},
	{"https://github.com?token=abc", "https://github.com", []string{"abc"}},
	{"ssh://git@host/org/plan.git?token=abc", "ssh://git@host/org/plan.git", []string{"abc"}},
	{"https://h/x?%zz", "https://h/x", []string{"%zz"}},
	{"https://u:pw@github.com/o/r.git?token=abc", "https://github.com/o/r.git", []string{"pw", "abc"}},
	// Non-URL remotes are unchanged (AC5).
	{"git@github.com:o/r.git", "git@github.com:o/r.git", nil},
	{"github.com:o/r.git", "github.com:o/r.git", nil},
	{"host:a://b", "host:a://b", nil},
	{"git@host:org/x://y?z", "git@host:org/x://y?z", nil},
	{"://u:pw@host/x", "://u:pw@host/x", nil},
	{"/srv/plan.git", "/srv/plan.git", nil},
	{"", "", nil},
	// A clean URL remote is byte-identical, escapes included (AC6).
	{"https://dev.azure.com/org/My%20Project/_git/plan", "https://dev.azure.com/org/My%20Project/_git/plan", nil},
	{"ssh://git@ssh.dev.azure.com:22/v3/org/My%20Project/plan", "ssh://git@ssh.dev.azure.com:22/v3/org/My%20Project/plan", nil},
	{"https://user:pw@dev.azure.com/org/My%20Project/_git/plan", "https://dev.azure.com/org/My%20Project/_git/plan", []string{"pw"}},
	// A URL remote Go can't parse records nothing if it might hold a secret (AC9).
	{"ssh://user:p#w@h/x", "", []string{"p#w"}},
	{"https://u:pw@h/%zz", "", []string{"pw"}},
	{"ssh://h:bad/x", "ssh://h:bad/x", nil},
}

func TestStripCredentials(t *testing.T) {
	for _, c := range stripCases {
		got := stripCredentials(c.in)
		if got != c.want {
			t.Errorf("stripCredentials(%q) = %q, want %q", c.in, got, c.want)
		}
		for _, s := range c.secrets {
			if strings.Contains(got, s) {
				t.Errorf("stripCredentials(%q) = %q, which keeps %q", c.in, got, s)
			}
		}
	}
}

// TestStripCredentialsOutputInvariants checks each stripped URL as net/url reads
// it: no password, a username only for ssh, no query or fragment, and the input's
// host and path.
func TestStripCredentialsOutputInvariants(t *testing.T) {
	for _, c := range stripCases {
		in, err := url.Parse(c.in)
		if err != nil || in.Scheme == "" || in.Opaque != "" {
			continue
		}
		got := stripCredentials(c.in)
		out, err := url.Parse(got)
		if err != nil {
			t.Errorf("stripCredentials(%q) = %q, which doesn't parse: %v", c.in, got, err)
			continue
		}
		ssh := out.Scheme == "ssh" || out.Scheme == "git+ssh" || out.Scheme == "ssh+git"
		if _, ok := out.User.Password(); ok || (out.User != nil && !ssh) {
			t.Errorf("stripCredentials(%q) = %q keeps userinfo %q", c.in, got, out.User)
		}
		if out.RawQuery != "" || out.ForceQuery || out.Fragment != "" || strings.ContainsAny(got, "?#") {
			t.Errorf("stripCredentials(%q) = %q keeps a query or fragment", c.in, got)
		}
		if out.Host != in.Host || out.EscapedPath() != in.EscapedPath() {
			t.Errorf("stripCredentials(%q) = %q: host/path %q %q, want %q %q", c.in, got, out.Host, out.EscapedPath(), in.Host, in.EscapedPath())
		}
	}
}

// capture runs f with stdout and stderr redirected, returning its exit code and
// what it printed to each.
func capture(t *testing.T, f func() int) (int, string, string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	code := f()
	os.Stdout, os.Stderr = origOut, origErr
	outW.Close()
	errW.Close()
	out, _ := io.ReadAll(outR)
	errOut, _ := io.ReadAll(errR)
	return code, string(out), string(errOut)
}

// writeProjectSettings replaces .tomo/settings.json with the given JSON object.
func writeProjectSettings(t *testing.T, root, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".tomo", "settings.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// bareWorkspace is a scaffolded root, outside any git repo, with a target repo
// and no change yet.
func bareWorkspace(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if _, err := icm.Scaffold(root, "", icm.Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	if code := runRepo(root, []string{"add", "app", "https://github.com/org/app"}); code != 0 {
		t.Fatalf("repo add = %d", code)
	}
	return root
}

func TestLocalWorkspaceTransitionsDontCommitOrWarn(t *testing.T) {
	root := bareWorkspace(t)
	writeProjectSettings(t, root, `{"vcs": "none"}`)
	steps := []struct {
		name string
		run  func() int
	}{
		{"change new", func() int { return runChange(root, []string{"new", "Auth flow"}) }},
		{"gate", func() int { return runGate(root, []string{"intake", "approved"}) }},
		{"reopen", func() int { return runReopen(root, []string{"intake", "--reason", "rethink"}) }},
		{"change close", func() int { return runChange(root, []string{"close", "1-auth-flow", "--reason", "dropped"}) }},
	}
	for _, s := range steps {
		code, _, errOut := capture(t, s.run)
		if code != 0 || errOut != "" {
			t.Fatalf("%s: code = %d, stderr = %q", s.name, code, errOut)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state.ChangeDir(root, "1-auth-flow", ""), ".gitkeep")); !os.IsNotExist(err) {
		t.Fatalf("change folder has a .gitkeep: %v", err)
	}
	if info, err := os.Stat(state.ChangeDir(root, "1-auth-flow", "")); err != nil || !info.IsDir() {
		t.Fatalf("change folder missing: %v", err)
	}
	log := readLog(t, root)
	if n := strings.Count(log, "1-auth-flow"); n != 4 {
		t.Fatalf("LOG.md has %d entries for the change, want 4:\n%s", n, log)
	}
	changes, err := state.ReadChanges(root)
	if err != nil || changes["1-auth-flow"] == nil {
		t.Fatalf("STATE.json: %v, %v", changes, err)
	}
}

func TestGitWorkspaceOutsideRepoWarnsWithHint(t *testing.T) {
	root := bareWorkspace(t)
	if code, _, _ := capture(t, func() int { return runChange(root, []string{"new", "Auth flow"}) }); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	code, _, errOut := capture(t, func() int { return runGate(root, []string{"intake", "approved"}) })
	if code != 0 {
		t.Fatalf("gate = %d, stderr = %q", code, errOut)
	}
	if !strings.Contains(errOut, "not in a git repo") || !strings.Contains(errOut, `set "vcs": "none"`) {
		t.Fatalf("stderr = %q", errOut)
	}
	if st, err := state.LoadState(root); err != nil || st.Gates["intake"] != "approved" {
		t.Fatalf("state = %+v, %v", st, err)
	}
}

func TestInvalidVCSFailsGateWithoutChangingState(t *testing.T) {
	root := workspace(t)
	before, err := os.ReadFile(state.StatePath(root))
	if err != nil {
		t.Fatal(err)
	}
	writeProjectSettings(t, root, `{"vcs": "svn"}`)
	code, _, errOut := capture(t, func() int { return runGate(root, []string{"intake", "approved"}) })
	if code == 0 || !strings.Contains(errOut, `invalid vcs "svn"`) {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	after, err := os.ReadFile(state.StatePath(root))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("STATE.json changed:\n%s", after)
	}
}

func TestStatusShowsVCS(t *testing.T) {
	root := bareWorkspace(t)
	code, out, _ := capture(t, func() int { return runStatus(root) })
	if code != 0 || out != "no active change\nvcs:      git\n" {
		t.Fatalf("no setting, none active: code = %d, out = %q", code, out)
	}
	writeProjectSettings(t, root, `{"vcs": "none"}`)
	code, out, _ = capture(t, func() int { return runStatus(root) })
	if code != 0 || out != "no active change\nvcs:      none\n" {
		t.Fatalf("none, none active: code = %d, out = %q", code, out)
	}
	if code := runChange(root, []string{"new", "Auth flow"}); code != 0 {
		t.Fatalf("change new = %d", code)
	}
	code, out, _ = capture(t, func() int { return runStatus(root) })
	if code != 0 || !strings.HasPrefix(out, "change:   1-auth-flow\n") || !strings.HasSuffix(out, "\nvcs:      none\n") {
		t.Fatalf("none, active: code = %d, out = %q", code, out)
	}
}

func TestSettingsShowsVCS(t *testing.T) {
	root := bareWorkspace(t)
	code, out, _ := capture(t, func() int { return runSettings(root) })
	if code != 0 || !strings.Contains(out, "gates:       block\nvcs:         git\n") {
		t.Fatalf("code = %d, out = %q", code, out)
	}
	writeProjectSettings(t, root, `{"vcs": "none"}`)
	if _, out, _ := capture(t, func() int { return runSettings(root) }); !strings.Contains(out, "vcs:         none\n") {
		t.Fatalf("out = %q", out)
	}
}

func TestInvalidVCSFailsStatusAndSettings(t *testing.T) {
	root := bareWorkspace(t)
	writeProjectSettings(t, root, `{"vcs": "svn"}`)
	for name, run := range map[string]func() int{
		"status":   func() int { return runStatus(root) },
		"settings": func() int { return runSettings(root) },
	} {
		code, _, errOut := capture(t, run)
		if code == 0 || !strings.Contains(errOut, `invalid vcs "svn"`) {
			t.Errorf("%s: code = %d, stderr = %q", name, code, errOut)
		}
	}
}
