// Package settings owns Tomo's mergeable settings: global
// (~/.config/tomo/settings.json) then project (.tomo/settings.json); the
// project supersedes the global. Effective settings = defaults ← global ←
// project.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/btdstudio/tomo/internal/state"
)

var VALID_GATES = []string{"block", "prompt", "auto"}
var VALID_ISSUES = []string{"local"}

// Settings is the effective settings shape.
type Settings struct {
	BaseSource  string            `json:"baseSource"`
	Gates       string            `json:"gates"`
	ArtifactDir string            `json:"artifactDir"`
	Issues      string            `json:"issues"`
	IssuesDir   string            `json:"issuesDir"`
	AppSources  map[string]string `json:"appSources"`
}

func DefaultSettings() Settings {
	return Settings{
		BaseSource:  "", // default base requirement source: file | "issue:N" | "url:..."
		Gates:       "block",
		ArtifactDir: "01_planning/specs",
		Issues:      "local",
		IssuesDir:   "00_issues",
		AppSources:  map[string]string{"kumi": "github.com/btdstudio/kumi", "hachigo": "github.com/btdstudio/hachigo"},
	}
}

func GlobalSettingsPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			base = ""
		} else {
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "tomo", "settings.json")
}

func ProjectSettingsPath(root string) string {
	return filepath.Join(state.Workspace(root), "settings.json")
}

// ReadIfExists parses a JSON object file, returning an empty map when absent.
func ReadIfExists(file string) (map[string]any, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("tomo: cannot parse settings %s: %v", file, err)
	}
	m, ok := parsed.(map[string]any)
	if !ok || m == nil {
		return nil, fmt.Errorf("tomo: settings must be a JSON object: %s", file)
	}
	return m, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func stringField(s *Settings, key, val string) {
	switch key {
	case "baseSource":
		s.BaseSource = val
	case "gates":
		s.Gates = val
	case "artifactDir":
		s.ArtifactDir = val
	case "issues":
		s.Issues = val
	case "issuesDir":
		s.IssuesDir = val
	}
}

// MergeSettings overlays the raw layers (global then project) over defaults.
// Layers are raw JSON objects so a present-but-empty value still overrides,
// matching the JS Object.assign semantics.
func MergeSettings(layers ...map[string]any) (Settings, error) {
	out := DefaultSettings()
	for _, layer := range layers {
		for key, value := range layer {
			if value == nil {
				continue
			}
			if key == "appSources" {
				out.AppSources = mergeStringMap(out.AppSources, appSourcesFromRaw(value))
				continue
			}
			if s, ok := value.(string); ok {
				stringField(&out, key, s)
			}
		}
	}
	if !contains(VALID_GATES, out.Gates) {
		return Settings{}, fmt.Errorf("tomo: invalid gates %q (expected %s)", out.Gates, join(VALID_GATES, "|"))
	}
	if !contains(VALID_ISSUES, out.Issues) {
		fmt.Fprintf(os.Stderr, "tomo: invalid issues %q (expected %s); using \"local\"\n", out.Issues, join(VALID_ISSUES, "|"))
		out.Issues = "local"
	}
	return out, nil
}

func join(list []string, sep string) string {
	s := ""
	for i, v := range list {
		if i > 0 {
			s += sep
		}
		s += v
	}
	return s
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

// Loaded is the result of LoadSettings.
type Loaded struct {
	Global    map[string]any
	Project   map[string]any
	Effective Settings
}

func LoadSettings(root string) (Loaded, error) {
	global, err := ReadIfExists(GlobalSettingsPath())
	if err != nil {
		return Loaded{}, err
	}
	project, err := ReadIfExists(ProjectSettingsPath(root))
	if err != nil {
		return Loaded{}, err
	}
	effective, err := MergeSettings(global, project)
	if err != nil {
		return Loaded{}, err
	}
	return Loaded{Global: global, Project: project, Effective: effective}, nil
}
