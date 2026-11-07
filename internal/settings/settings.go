// Package settings defines tomo's settings shape and merges it through kiso:
// effective settings = defaults ← global (~/.config/tomo/settings.json) ←
// project (.tomo/settings.json).
package settings

import (
	core "github.com/btdstudio/kiso/settings"
	"github.com/btdstudio/tomo/internal/state"
)

// Settings is the effective settings shape.
type Settings struct {
	BaseSource  string            `json:"baseSource"`
	Gates       string            `json:"gates"`
	ArtifactDir string            `json:"artifactDir"`
	IssuesDir   string            `json:"issuesDir"`
	SyncFile    string            `json:"syncFile"`
	AppSources  map[string]string `json:"appSources"`
}

// Loaded is the result of LoadSettings.
type Loaded = core.Loaded[Settings]

var spec = core.Spec[Settings]{
	Tool:     "tomo",
	Defaults: DefaultSettings,
	Apply:    apply,
	Validate: func(s *Settings) error { return core.CheckGates("tomo", s.Gates) },
}

func DefaultSettings() Settings {
	return Settings{
		BaseSource:  "", // default base requirement source: file | "issue:N" | "url:..."
		Gates:       "block",
		ArtifactDir: state.DefaultArtifactDir,
		IssuesDir:   "00_issues",
		SyncFile:    "01_planning/sync.md",
		AppSources:  map[string]string{"kumi": "github.com/btdstudio/kumi/cmd/kumi", "hanga": "github.com/btdstudio/hanga/cmd/hanga"},
	}
}

func apply(s *Settings, key string, value any) {
	if key == "appSources" {
		s.AppSources = mergeStringMap(s.AppSources, appSourcesFromRaw(value))
		return
	}
	val, ok := value.(string)
	if !ok {
		return
	}
	switch key {
	case "baseSource":
		s.BaseSource = val
	case "gates":
		s.Gates = val
	case "artifactDir":
		s.ArtifactDir = val
	case "issuesDir":
		s.IssuesDir = val
	case "syncFile":
		s.SyncFile = val
	}
}

func mergeStringMap(base, next map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range next {
		out[k] = v
	}
	return out
}

func appSourcesFromRaw(v any) map[string]string {
	out := map[string]string{}
	m, ok := v.(map[string]any)
	if !ok {
		return out
	}
	for k, vv := range m {
		if s, ok := vv.(string); ok {
			out[k] = s
		}
	}
	return out
}

func GlobalSettingsPath() string { return spec.GlobalPath() }

func ProjectSettingsPath(root string) string { return spec.ProjectPath(root) }

// ReadIfExists parses a JSON object file, returning an empty map when absent.
func ReadIfExists(file string) (map[string]any, error) { return spec.ReadIfExists(file) }

// MergeSettings overlays the raw layers (global then project) over defaults.
func MergeSettings(layers ...map[string]any) (Settings, error) { return spec.Merge(layers...) }

func LoadSettings(root string) (Loaded, error) { return spec.Load(root) }
