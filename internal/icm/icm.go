// Package icm configures kiso's scaffold engine for tomo — the embedded SDLC
// workspace and its generated seeds — and adds apps from external repos.
package icm

import (
	"encoding/json"
	"io/fs"

	"github.com/btdstudio/kiso/scaffold"
	"github.com/btdstudio/tomo"
	"github.com/btdstudio/tomo/internal/settings"
)

// WorkspaceDir is the per-project state directory (gitignored).
const WorkspaceDir = ".tomo"

type (
	Manifest     = scaffold.Manifest
	Options      = scaffold.Options
	Report       = scaffold.Report
	UpdateReport = scaffold.UpdateReport
)

var engine = scaffold.Engine{
	Tool:   "tomo",
	FS:     mustSub(tomo.ScaffoldFS, "scaffold"),
	Marker: "00_issues",
	Seeds:  seeds,
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// seeds generates the .tomo/ settings, state and log. They are produced by the
// binary — not copied from the scaffold — so `tomo update` never touches the
// user's workspace state.
func seeds() ([]scaffold.Seed, error) {
	settingsData, err := json.MarshalIndent(settings.DefaultSettings(), "", "  ")
	if err != nil {
		return nil, err
	}
	stateData, err := json.MarshalIndent(map[string]any{
		"change":    "",
		"updatedAt": "",
		"changes":   map[string]any{},
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	return []scaffold.Seed{
		{Rel: ".tomo/settings.json", Data: append(settingsData, '\n')},
		{Rel: ".tomo/state.json", Data: append(stateData, '\n')},
		{Rel: ".tomo/log.md", Data: []byte{}},
	}, nil
}

// Scaffold writes the embedded tree into dir; see scaffold.Engine.Scaffold.
func Scaffold(dir string, opts Options) (Report, error) { return engine.Scaffold(dir, opts) }

// Update refreshes managed files; see scaffold.Engine.Update.
func Update(dir string) (UpdateReport, error) { return engine.Update(dir) }

// Check validates the embedded scaffold; see scaffold.Engine.Check.
func Check() []string { return engine.Check() }

// BuildManifest hashes the embedded scaffold.
func BuildManifest() (Manifest, error) { return engine.BuildManifest() }

// ReadManifest reads .tomo/manifest.json.
func ReadManifest(dir string) (Manifest, error) { return engine.ReadManifest(dir) }

// WriteManifest writes .tomo/manifest.json.
func WriteManifest(dir string, m Manifest) error { return engine.WriteManifest(dir, m) }
