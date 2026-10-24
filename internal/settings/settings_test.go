package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeSettings(t *testing.T, p string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultsApplyWhenNoFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	loaded, err := LoadSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Global, map[string]any{}) {
		t.Fatalf("global = %v", loaded.Global)
	}
	if !reflect.DeepEqual(loaded.Project, map[string]any{}) {
		t.Fatalf("project = %v", loaded.Project)
	}
	if !reflect.DeepEqual(loaded.Effective, DefaultSettings()) {
		t.Fatalf("effective = %+v", loaded.Effective)
	}
}

func TestProjectSupersedesGlobal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	globalPath := GlobalSettingsPath()
	projectPath := ProjectSettingsPath(root)
	writeSettings(t, globalPath, map[string]any{
		"baseSource": "issue:12", "gates": "auto", "provider": "opencode",
		"issues": "local", "issuesDir": "tickets",
	})
	writeSettings(t, projectPath, map[string]any{"gates": "prompt", "issuesDir": "issues/custom"})
	loaded, err := LoadSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	e := loaded.Effective
	if e.BaseSource != "issue:12" || e.Gates != "prompt" || e.Provider != "opencode" ||
		e.ArtifactDir != ".tomo/specs" || e.Issues != "local" || e.IssuesDir != "issues/custom" {
		t.Fatalf("effective = %+v", e)
	}
}

func TestInvalidGatesThrows(t *testing.T) {
	if _, err := MergeSettings(map[string]any{"gates": "aggressive"}); err == nil || !strings.Contains(err.Error(), "invalid gates") {
		t.Fatalf("want invalid gates, got %v", err)
	}
}

func TestPersonasMergePerKey(t *testing.T) {
	global := map[string]any{
		"personas": map[string]any{
			"map":    map[string]any{"plan": "security", "trace": "reviewer"},
			"custom": map[string]any{"lead": "personas/lead.md"},
		},
	}
	project := map[string]any{
		"personas": map[string]any{
			"map":    map[string]any{"plan": "architect"},
			"custom": map[string]any{"pm": "personas/pm.md"},
		},
	}
	eff, err := MergeSettings(global, project)
	if err != nil {
		t.Fatal(err)
	}
	want := Personas{
		Map:    map[string]string{"plan": "architect", "trace": "reviewer"},
		Custom: map[string]string{"lead": "personas/lead.md", "pm": "personas/pm.md"},
	}
	if !reflect.DeepEqual(eff.Personas, want) {
		t.Fatalf("personas = %+v, want %+v", eff.Personas, want)
	}
}

func TestPersonasDefaultsEmptyAndLoadLayers(t *testing.T) {
	d := DefaultSettings().Personas
	if !reflect.DeepEqual(d.Map, map[string]string{}) || !reflect.DeepEqual(d.Custom, map[string]string{}) {
		t.Fatalf("default personas = %+v", d)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	writeSettings(t, GlobalSettingsPath(), map[string]any{
		"personas": map[string]any{"map": map[string]any{"debt": "security"}},
	})
	loaded, err := LoadSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Effective.Personas.Map["debt"] != "security" {
		t.Fatalf("map.debt = %q", loaded.Effective.Personas.Map["debt"])
	}
	if !reflect.DeepEqual(loaded.Effective.Personas.Custom, map[string]string{}) {
		t.Fatalf("custom = %v", loaded.Effective.Personas.Custom)
	}
}

func TestInvalidIssuesFallsBackToGitHub(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	writeSettings(t, ProjectSettingsPath(root), map[string]any{"issues": "bogus"})
	loaded, err := LoadSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Effective.Issues != "github" {
		t.Fatalf("issues = %q", loaded.Effective.Issues)
	}
}

func TestUnreadableJSONThrows(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	p := ProjectSettingsPath(root)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettings(root); err == nil || !strings.Contains(err.Error(), "cannot parse settings") {
		t.Fatalf("want cannot parse settings, got %v", err)
	}
}

func TestGlobalPathHonorsXDGConfigHome(t *testing.T) {
	alt := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", alt)
	if !strings.HasPrefix(GlobalSettingsPath(), alt) {
		t.Fatalf("global path %q should start with %q", GlobalSettingsPath(), alt)
	}
}
