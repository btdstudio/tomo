// Package settings defines tomo's settings shape and merges it through kiso:
// effective settings = defaults ← global (~/.config/tomo/settings.json) ←
// project (.tomo/settings.json).
package settings

import (
	"fmt"
	"os"
	"strings"

	core "github.com/btdstudio/kiso/settings"
	"github.com/btdstudio/tomo/internal/state"
)

var VALID_ISSUES = []string{"local"}
var VALID_PROJECT_TYPES = []string{"monorepo", "standard"}

// Settings is the effective settings shape.
type Settings struct {
	BaseSource  string            `json:"baseSource"`
	Gates       string            `json:"gates"`
	ArtifactDir string            `json:"artifactDir"`
	Issues      string            `json:"issues"`
	IssuesDir   string            `json:"issuesDir"`
	OutputDir   string            `json:"outputDir"`
	ProjectType string            `json:"projectType"`
	AppSources  map[string]string `json:"appSources"`
}

// Loaded is the result of LoadSettings.
type Loaded = core.Loaded[Settings]

var spec = core.Spec[Settings]{
	Tool:     "tomo",
	Defaults: DefaultSettings,
	Apply:    apply,
	Validate: validate,
}

func DefaultSettings() Settings {
	return Settings{
		BaseSource:  "", // default base requirement source: file | "issue:N" | "url:..."
		Gates:       "block",
		ArtifactDir: state.DefaultArtifactDir,
		Issues:      "local",
		IssuesDir:   "00_issues",
		OutputDir:   "src",      // default output directory for generated apps
		ProjectType: "monorepo", // monorepo: outputDir/apps/<name>; standard: outputDir is the app root
		AppSources:  map[string]string{"kumi": "github.com/btdstudio/kumi/cmd/kumi", "hachigo": "github.com/btdstudio/hachigo/cmd/hachigo"},
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
	case "issues":
		s.Issues = val
	case "issuesDir":
		s.IssuesDir = val
	case "outputDir":
		s.OutputDir = val
	case "projectType":
		s.ProjectType = val
	}
}

func validate(s *Settings) error {
	if err := core.CheckGates("tomo", s.Gates); err != nil {
		return err
	}
	if !contains(VALID_ISSUES, s.Issues) {
		fmt.Fprintf(os.Stderr, "tomo: invalid issues %q (expected %s); using \"local\"\n", s.Issues, strings.Join(VALID_ISSUES, "|"))
		s.Issues = "local"
	}
	if !contains(VALID_PROJECT_TYPES, s.ProjectType) {
		fmt.Fprintf(os.Stderr, "tomo: invalid projectType %q (expected %s); using \"monorepo\"\n", s.ProjectType, strings.Join(VALID_PROJECT_TYPES, "|"))
		s.ProjectType = "monorepo"
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
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
