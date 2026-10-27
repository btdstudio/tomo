package icm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/btdstudio/tomo/internal/settings"
)

// AddReport describes what Add did.
type AddReport struct {
	Path   string
	Source string
}

// appScaffold maps an app type to the subcommand its tool uses to scaffold a
// new project into a directory.
var appScaffold = map[string]string{
	"kumi":    "new",
	"hachigo": "init",
}

// Add installs the app's scaffold tool and runs it to create the app under
// src/apps/<name>/. extraArgs (e.g. the Go module path for kumi) are passed
// through to the tool.
func Add(dir, appType, name string, extraArgs ...string) (AddReport, error) {
	rep := AddReport{}
	loaded, err := settings.LoadSettings(dir)
	if err != nil {
		return rep, err
	}
	installPath, ok := loaded.Effective.AppSources[appType]
	if !ok || installPath == "" {
		return rep, fmt.Errorf("tomo add: unknown app type %q (known: kumi, hachigo)", appType)
	}
	sub, ok := appScaffold[appType]
	if !ok {
		return rep, fmt.Errorf("tomo add: unknown app type %q", appType)
	}

	bin, err := GoInstall(installPath)
	if err != nil {
		return rep, fmt.Errorf("tomo add: install %s: %w", installPath, err)
	}

	appsDir := filepath.Join(dir, "src", "apps")
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		return rep, err
	}
	args := append([]string{sub, name}, extraArgs...)
	if err := runCmd(appsDir, bin, args...); err != nil {
		return rep, fmt.Errorf("tomo add: %s %s: %w", appType, sub, err)
	}

	rep.Path = filepath.Join(appsDir, name)
	rep.Source = installPath
	return rep, nil
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
