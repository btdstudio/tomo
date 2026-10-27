// Package icm implements Tomo's ICM engine: scaffold a folder-driven SDLC
// workspace, refresh its managed files, and add apps from external repos.
package icm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/btdstudio/tomo"
	"github.com/btdstudio/tomo/internal/version"
)

// WorkspaceDir is the per-project state directory (gitignored).
const WorkspaceDir = ".tomo"

// Manifest records the tomo version and a sha256 per managed file, so
// `tomo update` can tell which files the user has edited.
type Manifest struct {
	Version string            `json:"version"`
	Files   map[string]string `json:"files"` // relative path -> sha256 hex
}

// walkScaffold calls fn for each file in the embedded scaffold, passing the
// path relative to the scaffold root and the file's bytes.
func walkScaffold(fn func(rel string, data []byte) error) error {
	return fs.WalkDir(tomo.ScaffoldFS, "scaffold", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(tomo.ScaffoldFS, p)
		if err != nil {
			return err
		}
		return fn(strings.TrimPrefix(p, "scaffold/"), data)
	})
}

func readScaffoldFile(rel string) ([]byte, error) {
	return fs.ReadFile(tomo.ScaffoldFS, "scaffold/"+rel)
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// BuildManifest walks the embedded scaffold and hashes every file.
func BuildManifest() (Manifest, error) {
	m := Manifest{Version: version.String(), Files: map[string]string{}}
	err := walkScaffold(func(rel string, data []byte) error {
		m.Files[rel] = hashBytes(data)
		return nil
	})
	if err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func manifestPath(dir string) string {
	return filepath.Join(dir, WorkspaceDir, "manifest.json")
}

// ReadManifest reads .tomo/manifest.json.
func ReadManifest(dir string) (Manifest, error) {
	data, err := os.ReadFile(manifestPath(dir))
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	if m.Files == nil {
		m.Files = map[string]string{}
	}
	return m, nil
}

// WriteManifest writes .tomo/manifest.json.
func WriteManifest(dir string, m Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(manifestPath(dir)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(manifestPath(dir), data, 0o644)
}
