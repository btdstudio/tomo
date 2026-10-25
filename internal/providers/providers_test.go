package providers

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRegistryExposesSixProviders(t *testing.T) {
	if !reflect.DeepEqual(ProviderIds(), []string{"opencode", "claude", "copilot", "antigravity", "codex", "pi"}) {
		t.Fatalf("provider ids = %v", ProviderIds())
	}
	for _, id := range ProviderIds() {
		p := PROVIDERS[id]
		if p.ID != id || p.Label == "" {
			t.Fatalf("%s: id/label", id)
		}
		if p.ProjectCommandDir == nil || p.GlobalCommandDir == nil || p.ProjectAgentDir == nil ||
			p.GlobalAgentDir == nil || p.CommandFilename == nil || p.Filename == nil || p.Cast == nil {
			t.Fatalf("%s: incomplete contract", id)
		}
	}
	if !IsProvider("antigravity") || IsProvider("gemini") {
		t.Fatal("isProvider mismatch")
	}
}

func TestProviderDirsAndFilenames(t *testing.T) {
	oc, cl, cp, ag, cx := PROVIDERS["opencode"], PROVIDERS["claude"], PROVIDERS["copilot"], PROVIDERS["antigravity"], PROVIDERS["codex"]
	assertSuffix(t, oc.ProjectCommandDir("/p"), filepath.Join("p", ".opencode", "command"))
	assertSuffix(t, cl.ProjectCommandDir("/p"), filepath.Join("p", ".claude", "commands", "tomo"))
	assertSuffix(t, cp.ProjectCommandDir("/p"), filepath.Join("p", ".github", "skills"))
	assertSuffix(t, ag.ProjectCommandDir("/p"), filepath.Join("p", ".agents", "skills"))
	assertSuffix(t, oc.ProjectAgentDir("/p"), filepath.Join("p", ".opencode", "agents"))
	assertSuffix(t, cl.ProjectAgentDir("/p"), filepath.Join("p", ".claude", "agents"))
	assertSuffix(t, cp.ProjectAgentDir("/p"), filepath.Join("p", ".github", "agents"))
	assertSuffix(t, ag.ProjectAgentDir("/p"), filepath.Join("p", ".agents", "agents"))
	assertSuffix(t, oc.GlobalCommandDir(), filepath.Join("opencode", "command"))
	assertSuffix(t, cl.GlobalCommandDir(), filepath.Join(".claude", "commands", "tomo"))
	assertSuffix(t, cp.GlobalCommandDir(), filepath.Join(".copilot", "skills"))
	assertSuffix(t, ag.GlobalCommandDir(), filepath.Join(".gemini", "antigravity-cli", "skills"))
	assertSuffix(t, oc.GlobalAgentDir(), filepath.Join("opencode", "agents"))
	assertSuffix(t, cl.GlobalAgentDir(), filepath.Join(".claude", "agents"))
	assertSuffix(t, cp.GlobalAgentDir(), filepath.Join(".copilot", "agents"))
	assertSuffix(t, ag.GlobalAgentDir(), filepath.Join(".gemini", "config", "agents"))
	assertSuffix(t, cx.ProjectCommandDir("/p"), filepath.Join("p", ".agents", "skills"))
	assertSuffix(t, cx.ProjectAgentDir("/p"), filepath.Join("p", ".codex", "agents"))
	assertSuffix(t, cx.GlobalAgentDir(), filepath.Join(".codex", "agents"))

	if oc.CommandFilename("requirements") != "tomo-requirements.md" {
		t.Fatal("opencode command filename")
	}
	if cl.CommandFilename("requirements") != "requirements.md" {
		t.Fatal("claude command filename")
	}
	if cp.Filename("coder") != "coder.agent.md" {
		t.Fatal("copilot agent filename")
	}
	if cp.CommandFilename("requirements") != "tomo-requirements/SKILL.md" {
		t.Fatal("copilot command filename")
	}
	if ag.Filename("coder") != "coder/agent.md" {
		t.Fatal("antigravity agent filename")
	}
	if ag.CommandFilename("requirements") != "tomo-requirements/SKILL.md" {
		t.Fatal("antigravity command filename")
	}
	if cx.Filename("coder") != "coder.toml" {
		t.Fatal("codex agent filename")
	}
	if cx.CommandFilename("requirements") != "tomo-requirements/SKILL.md" {
		t.Fatal("codex command filename")
	}
}

func assertSuffix(t *testing.T, got, suffix string) {
	t.Helper()
	if !strings.HasSuffix(got, suffix) {
		t.Fatalf("%q should end with %q", got, suffix)
	}
}

func TestParseProviderSpec(t *testing.T) {
	all := []string{"opencode", "claude", "copilot", "antigravity", "codex", "pi"}
	cases := map[string][]string{
		"":                 all,
		"all":              all,
		"opencode":         {"opencode"},
		"claude":           {"claude"},
		"copilot":          {"copilot"},
		"opencode, claude": {"opencode", "claude"},
		"claude,claude":    {"claude", "claude"},
	}
	for in, want := range cases {
		got, err := ParseProviderSpec(in)
		if err != nil {
			t.Fatalf("parseProviderSpec(%q): %v", in, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("parseProviderSpec(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseProviderSpec("gemini"); err == nil || !strings.Contains(err.Error(), `unknown provider "gemini"`) {
		t.Fatalf("want unknown provider error, got %v", err)
	}
}

func TestSeedSettingsProvider(t *testing.T) {
	root := t.TempDir()
	changed, p, err := SeedSettingsProvider(filepath.Join(root, "a"), []string{"opencode"})
	if err != nil || !changed {
		t.Fatalf("seed: %v %v", changed, err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "{\n  \"provider\": \"opencode\"\n}\n" {
		t.Fatalf("seeded settings = %q", b)
	}

	// multi-provider leaves tag alone
	os.WriteFile(p, []byte("{\n  \"provider\": \"opencode\",\n  \"gates\": \"auto\"\n}\n"), 0o644)
	changed, _, _ = SeedSettingsProvider(filepath.Join(root, "a"), []string{"opencode", "claude"})
	if changed {
		t.Fatal("multi-provider should not change")
	}

	// same single provider is a no-op
	changed, _, _ = SeedSettingsProvider(filepath.Join(root, "a"), []string{"opencode"})
	if changed {
		t.Fatal("same provider should be no-op")
	}

	changed, p2, _ := SeedSettingsProvider(filepath.Join(root, "b"), []string{"copilot"})
	if !changed {
		t.Fatal("copilot seed should change")
	}
	b2, _ := os.ReadFile(p2)
	if string(b2) != "{\n  \"provider\": \"copilot\"\n}\n" {
		t.Fatalf("copilot seeded settings = %q", b2)
	}
}
