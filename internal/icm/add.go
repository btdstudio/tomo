package icm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/btdstudio/tomo/internal/settings"
)

// AddReport describes what Add did.
type AddReport struct {
	Path   string
	Source string
}

// Add installs an app of the given type from its configured source repo into
// src/apps/<name>/.
func Add(dir, appType, name string) (AddReport, error) {
	rep := AddReport{}
	loaded, err := settings.LoadSettings(dir)
	if err != nil {
		return rep, err
	}
	src, ok := loaded.Effective.AppSources[appType]
	if !ok || src == "" {
		return rep, fmt.Errorf("tomo add: unknown app type %q (known: kumi, hachigo)", appType)
	}
	dest := filepath.Join(dir, "src", "apps", name)
	if _, err := os.Stat(dest); err == nil {
		return rep, fmt.Errorf("tomo add: %s already exists", dest)
	}
	tmp, err := os.MkdirTemp("", "tomo-add-*")
	if err != nil {
		return rep, err
	}
	defer os.RemoveAll(tmp)
	cloneURL := src
	if !strings.Contains(src, "://") && !strings.HasPrefix(src, "git@") {
		cloneURL = "https://" + src + ".git"
	}
	if err := runGit(tmp, "clone", "--depth", "1", cloneURL, tmp); err != nil {
		return rep, fmt.Errorf("tomo add: clone %s: %w", src, err)
	}
	if err := os.RemoveAll(filepath.Join(tmp, ".git")); err != nil {
		return rep, err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return rep, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return rep, err
	}
	rep.Path = dest
	rep.Source = src
	return rep, nil
}
