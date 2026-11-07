package icm

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/btdstudio/kiso/repos"
)

func scaffolded(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	if _, err := Scaffold(dir, Options{NoGit: true}); err != nil {
		t.Fatal(err)
	}
	return dir
}

// fakeTool installs a stand-in generator that creates the folder it's told to,
// recording its arguments.
func fakeTool(t *testing.T) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "gen")
	script := "#!/bin/sh\nmkdir \"$2\" && echo \"$@\" > \"$2/args.txt\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := installTool
	installTool = func(string) (string, error) { return bin, nil }
	t.Cleanup(func() { installTool = orig })
}

func TestAddUnknownType(t *testing.T) {
	dir := scaffolded(t)
	if _, err := Add(dir, "bogus", "demo", AddOptions{Path: "/x", Remote: "https://github.com/o/x"}); err == nil || !strings.Contains(err.Error(), "unknown app type") {
		t.Fatalf("want unknown app type, got %v", err)
	}
}

func TestAddScaffoldsOutsideAndRegistersTarget(t *testing.T) {
	fakeTool(t)
	dir := scaffolded(t)
	dest := filepath.Join(t.TempDir(), "code", "my-folder")
	remote := "git@github.com:org/demo.git"
	rep, err := Add(dir, "kumi", "demo", AddOptions{Path: dest, Remote: remote, Args: []string{"github.com/org/demo"}})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Path != dest {
		t.Fatalf("path = %q", rep.Path)
	}
	args, err := os.ReadFile(filepath.Join(dest, "args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(args)); got != "new my-folder github.com/org/demo" {
		t.Fatalf("tool args = %q", got)
	}
	out, err := exec.Command("git", "-C", dest, "remote", "get-url", "origin").Output()
	if err != nil || strings.TrimSpace(string(out)) != remote {
		t.Fatalf("origin = %q, %v", out, err)
	}
	name, path, err := Target(dir)
	if err != nil || name != "demo" || path != dest {
		t.Fatalf("target = %q, %q, %v", name, path, err)
	}
	reg, err := repos.Load(dir, "tomo")
	if err != nil {
		t.Fatal(err)
	}
	if problems := reg.Doctor(); len(problems) != 0 {
		t.Fatalf("problems = %+v", problems)
	}
	if _, err := os.Stat(filepath.Join(dir, "src")); !os.IsNotExist(err) {
		t.Fatalf("workspace has src/: %v", err)
	}
}

func TestAddRefusals(t *testing.T) {
	fakeTool(t)
	dir := scaffolded(t)
	base := t.TempDir()
	remote := "https://github.com/org/demo"
	for name, c := range map[string]struct {
		opts AddOptions
		want string
	}{
		"no path":   {AddOptions{Remote: remote}, "--path and --remote"},
		"no remote": {AddOptions{Path: filepath.Join(base, "a")}, "--path and --remote"},
		"exists":    {AddOptions{Path: base, Remote: remote}, "already exists"},
		"token":     {AddOptions{Path: filepath.Join(base, "b"), Remote: "https://t@github.com/o/x"}, "credentials"},
	} {
		if _, err := Add(dir, "kumi", "demo", c.opts); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want %q, got %v", name, c.want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "REPOS.json")); err == nil {
		t.Fatal("a refused add wrote REPOS.json")
	}
	if _, err := Add(dir, "kumi", "demo", AddOptions{Path: filepath.Join(base, "c"), Remote: remote}); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(base, "d")
	if _, err := Add(dir, "hanga", "site", AddOptions{Path: second, Remote: remote}); err == nil || !strings.Contains(err.Error(), "exactly one repo") {
		t.Fatalf("second target err = %v", err)
	}
	if _, err := os.Stat(second); err == nil {
		t.Fatal("a refused add ran the generator")
	}
}

func TestAddFailedToolRegistersNothing(t *testing.T) {
	dir := scaffolded(t)
	bin := filepath.Join(t.TempDir(), "gen")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := installTool
	installTool = func(string) (string, error) { return bin, nil }
	defer func() { installTool = orig }()
	if _, err := Add(dir, "kumi", "demo", AddOptions{Path: filepath.Join(t.TempDir(), "a"), Remote: "https://github.com/o/x"}); err == nil {
		t.Fatal("want an error")
	}
	if _, _, err := Target(dir); err == nil || !strings.Contains(err.Error(), "no target repo") {
		t.Fatalf("err = %v", err)
	}
}

func TestTargetErrors(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Target(dir); err == nil || !strings.Contains(err.Error(), "no target repo") {
		t.Fatalf("none: %v", err)
	}
	body := `{"repos":{"a":{"remote":"https://github.com/o/a"},"b":{"remote":"https://github.com/o/b"}}}`
	if err := os.WriteFile(filepath.Join(dir, "REPOS.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Target(dir); err == nil || !strings.Contains(err.Error(), "a, b") {
		t.Fatalf("two: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "REPOS.json"), []byte(`{"repos":{"a":{"remote":"https://github.com/o/a"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Target(dir); err == nil || !strings.Contains(err.Error(), "no local path") {
		t.Fatalf("no path: %v", err)
	}
}

func TestOldAppsDir(t *testing.T) {
	dir := t.TempDir()
	if OldAppsDir(dir) {
		t.Fatal("no src/apps")
	}
	apps := filepath.Join(dir, "src", "apps")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apps, ".gitkeep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if OldAppsDir(dir) {
		t.Fatal("only .gitkeep")
	}
	if err := os.MkdirAll(filepath.Join(apps, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !OldAppsDir(dir) {
		t.Fatal("has an app")
	}
	if err := os.WriteFile(filepath.Join(dir, "REPOS.json"), []byte(`{"repos":{"a":{"remote":"https://github.com/o/a"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if OldAppsDir(dir) {
		t.Fatal("already has a target")
	}
}
