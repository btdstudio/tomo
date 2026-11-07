package icm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/btdstudio/kiso/repos"
	"github.com/btdstudio/tomo/internal/settings"
)

// AddReport describes what Add did.
type AddReport struct {
	Path   string
	Source string
}

// AddOptions configure Add.
type AddOptions struct {
	Path   string   // where the app goes; its last element is the folder the tool creates
	Remote string   // the remote to register and set as origin
	Args   []string // passed through to the tool, e.g. kumi's Go module path
}

// appScaffold maps an app type to the subcommand its tool uses to scaffold a
// new project into a directory.
var appScaffold = map[string]string{
	"kumi":  "new",
	"hanga": "init",
}

// installTool installs an app's scaffold tool. Tests replace it.
var installTool = GoInstall

// Add installs the app's scaffold tool, runs it to create the app outside the
// workspace, makes sure the app is a git repo with the given origin, and registers
// it as this workspace's target repo.
func Add(dir, appType, name string, opts AddOptions) (AddReport, error) {
	rep := AddReport{}
	loaded, err := settings.LoadSettings(dir)
	if err != nil {
		return rep, err
	}
	installPath, ok := loaded.Effective.AppSources[appType]
	if !ok || installPath == "" {
		return rep, fmt.Errorf("tomo add: unknown app type %q (known: kumi, hanga)", appType)
	}
	sub, ok := appScaffold[appType]
	if !ok {
		return rep, fmt.Errorf("tomo add: unknown app type %q", appType)
	}
	if opts.Path == "" || opts.Remote == "" {
		return rep, fmt.Errorf("tomo add: --path and --remote are required")
	}
	reg, err := repos.Load(dir, "tomo")
	if err != nil {
		return rep, err
	}
	if err := refuseSecondTarget(reg); err != nil {
		return rep, err
	}
	if err := reg.Add(name, opts.Remote, ""); err != nil {
		return rep, err
	}
	if err := reg.SetPath(name, opts.Path); err != nil {
		return rep, err
	}
	dest, err := reg.Resolve(name)
	if err != nil {
		return rep, err
	}
	if _, err := os.Stat(dest); err == nil {
		return rep, fmt.Errorf("tomo add: %s already exists — to use existing code, run `tomo repo add`", dest)
	}

	bin, err := installTool(installPath)
	if err != nil {
		return rep, fmt.Errorf("tomo add: install %s: %w", installPath, err)
	}
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return rep, err
	}
	args := append([]string{sub, filepath.Base(dest)}, opts.Args...)
	if err := runCmd(parent, bin, args...); err != nil {
		return rep, fmt.Errorf("tomo add: %s %s: %w", appType, sub, err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); os.IsNotExist(err) {
		if err := runCmd(dest, "git", "init", "--quiet"); err != nil {
			return rep, fmt.Errorf("tomo add: git init: %w", err)
		}
	}
	if err := runCmd(dest, "git", "remote", "add", "origin", opts.Remote); err != nil {
		return rep, fmt.Errorf("tomo add: setting origin: %w", err)
	}
	if err := reg.Save(); err != nil {
		return rep, err
	}
	rep.Path = dest
	rep.Source = installPath
	return rep, nil
}

func refuseSecondTarget(reg repos.Registry) error {
	if names := reg.Names(); len(names) > 0 {
		return fmt.Errorf("tomo: this workspace already targets %q, and a tomo workspace targets exactly one repo", names[0])
	}
	return nil
}

// RefuseSecondTarget returns an error if the workspace already has a target repo.
func RefuseSecondTarget(root string) error {
	reg, err := repos.Load(root, "tomo")
	if err != nil {
		return err
	}
	return refuseSecondTarget(reg)
}

// TargetName returns the workspace's one registered repo. It doesn't need the repo
// to be on this machine.
func TargetName(root string) (string, error) {
	reg, err := repos.Load(root, "tomo")
	if err != nil {
		return "", err
	}
	names := reg.Names()
	switch len(names) {
	case 0:
		return "", fmt.Errorf("tomo: no target repo — run `tomo add` for a new app, or `tomo repo add` for existing code")
	case 1:
		return names[0], nil
	}
	return "", fmt.Errorf("tomo: a tomo workspace targets exactly one repo, but REPOS.json has %s", strings.Join(names, ", "))
}

// Target returns the workspace's one registered repo and its path on this machine.
func Target(root string) (name, path string, err error) {
	name, err = TargetName(root)
	if err != nil {
		return "", "", err
	}
	reg, err := repos.Load(root, "tomo")
	if err != nil {
		return "", "", err
	}
	path, err = reg.Resolve(name)
	if err != nil {
		return "", "", err
	}
	return name, path, nil
}

// OldAppsDir reports whether the workspace still has code under src/apps/ and no
// target repo, from before code moved out of the workspace.
func OldAppsDir(root string) bool {
	reg, err := repos.Load(root, "tomo")
	if err != nil || len(reg.Repos) > 0 {
		return false
	}
	entries, err := os.ReadDir(filepath.Join(root, "src", "apps"))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name() != ".gitkeep" && e.Name() != ".DS_Store" {
			return true
		}
	}
	return false
}

// GoToolBin returns the GOPATH/bin path for a tool import path (e.g.
// github.com/btdstudio/kumi/cmd/kumi -> <GOPATH>/bin/kumi).
func GoToolBin(importPath string) (string, error) {
	out, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		return "", err
	}
	gopath := strings.TrimSpace(string(out))
	if gopath == "" {
		return "", fmt.Errorf("empty GOPATH")
	}
	base := importPath
	if i := strings.Index(base, "@"); i >= 0 {
		base = base[:i]
	}
	return filepath.Join(gopath, "bin", filepath.Base(base)), nil
}

// GoInstall installs a Go tool (import path) via `go install …@latest` and
// returns the path to the installed binary.
func GoInstall(importPath string) (string, error) {
	full := importPath
	if !strings.Contains(importPath, "@") {
		full += "@latest"
	}
	if err := runCmd("", "go", "install", full); err != nil {
		return "", err
	}
	return GoToolBin(importPath)
}

// runCmd runs a command in dir, streaming its output.
func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
