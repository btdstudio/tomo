package icm

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// localFiles are the machine-local files that a workspace's .gitignore keeps out
// of git, even in a local workspace (vcs: none).
var localFiles = []string{".tomo/active", ".tomo/paths.json", ".tomo/SLIP.md"}

// realGit skips unless git is on PATH, and isolates it from the developer's
// global and system config and from git's default excludes file
// ($XDG_CONFIG_HOME/git/ignore, or ~/.config/git/ignore), so nothing user-level
// can hide files from `git status`. HOME and XDG_CONFIG_HOME also cover git
// versions older than 2.32, which ignore GIT_CONFIG_GLOBAL.
func realGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func runGitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

// gitStatus lists the paths `git status` reports in dir, untracked files
// included one by one.
func gitStatus(t *testing.T, dir string) []string {
	t.Helper()
	var paths []string
	for _, line := range strings.Split(runGitIn(t, dir, "status", "--porcelain", "--untracked-files=all"), "\n") {
		if len(line) > 3 {
			paths = append(paths, line[3:])
		}
	}
	return paths
}

// localWorkspace scaffolds a local workspace in dir and writes its local files
// and a control file that git must see.
func localWorkspace(t *testing.T, dir string) {
	t.Helper()
	if _, err := Scaffold(dir, "none", Options{}); err != nil {
		t.Fatal(err)
	}
	for _, rel := range append([]string{"control.txt"}, localFiles...) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// assertIgnored checks that status lists the control file and none of the local
// files. Paths are compared by suffix, since status paths are relative to the repo.
func assertIgnored(t *testing.T, status []string) {
	t.Helper()
	seen := false
	for _, p := range status {
		if strings.HasSuffix(p, "control.txt") {
			seen = true
		}
		for _, rel := range localFiles {
			if strings.HasSuffix(p, rel) {
				t.Errorf("git status lists %s", p)
			}
		}
	}
	if !seen {
		t.Fatalf("git status doesn't list control.txt, so it proves nothing: %v", status)
	}
}

func TestLocalWorkspaceInsideRepoIgnoresLocalFiles(t *testing.T) {
	realGit(t)
	outer := t.TempDir()
	runGitIn(t, outer, "init", "--quiet")
	localWorkspace(t, filepath.Join(outer, "ws"))
	assertIgnored(t, gitStatus(t, outer))
}

func TestLocalWorkspaceAfterGitInitIgnoresLocalFiles(t *testing.T) {
	realGit(t)
	dir := t.TempDir()
	localWorkspace(t, dir)
	runGitIn(t, dir, "init", "--quiet")
	assertIgnored(t, gitStatus(t, dir))
}
