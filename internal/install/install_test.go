package install

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/btdstudio/tomo/internal/providers"
	"github.com/btdstudio/tomo/internal/render"
)

func fakeBinary(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "tomo")
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho tomo\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func expectedProject(t *testing.T, project string, ids []string) []string {
	var list []string
	for _, cmd := range mustCommands(t) {
		for _, id := range ids {
			prov := providers.PROVIDERS[id]
			list = append(list, filepath.Join(prov.ProjectCommandDir(project), prov.CommandFilename(cmd.Name)))
		}
	}
	list = append(list, filepath.Join(project, ".tomo", "templates", "requirements.md"))
	list = append(list, filepath.Join(project, ".tomo", "templates", "design.md"))
	for _, p := range mustPersonas(t) {
		for _, id := range ids {
			prov := providers.PROVIDERS[id]
			list = append(list, filepath.Join(prov.ProjectAgentDir(project), prov.Filename(p.Name)))
		}
		list = append(list, filepath.Join(project, ".tomo", "personas", p.Name+".md"))
	}
	list = append(list, filepath.Join(project, ".tomo", "bin", "tomo"))
	if len(ids) == 1 {
		list = append(list, filepath.Join(project, ".tomo", "settings.json"))
	}
	return list
}

func mustCommands(t *testing.T) []providers.Command {
	t.Helper()
	c, err := render.ReadCommands()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustPersonas(t *testing.T) []providers.Persona {
	t.Helper()
	p, err := render.ReadPersonas()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func sortStrings(s []string) []string {
	out := append([]string{}, s...)
	sort.Strings(out)
	return out
}

func TestPerProjectInstallAllProviders(t *testing.T) {
	project := t.TempDir()
	ids := providers.ProviderIds()
	written, err := Install(Options{Project: project, SelfPath: fakeBinary(t)})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sortStrings(written), sortStrings(expectedProject(t, project, ids))) {
		t.Fatalf("written mismatch:\n got %v\nwant %v", written, expectedProject(t, project, ids))
	}

	// Wrappers equal the in-memory rendered output.
	all, err := render.GenerateAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, cmd := range mustCommands(t) {
		for _, id := range ids {
			prov := providers.PROVIDERS[id]
			dest := filepath.Join(prov.ProjectCommandDir(project), prov.CommandFilename(cmd.Name))
			want := all[render.ProviderCommandName(cmd.Name, id)]
			got, _ := os.ReadFile(dest)
			if string(got) != want {
				t.Fatalf("%s: wrapper mismatch", dest)
			}
		}
	}

	// No runtime .tomo/lib (replaced by the binary).
	if _, err := os.Stat(filepath.Join(project, ".tomo", "lib")); err == nil {
		t.Fatal(".tomo/lib should not be seeded (binary replaces it)")
	}
	// Binary placed and executable.
	if info, err := os.Stat(filepath.Join(project, ".tomo", "bin", "tomo")); err != nil || info.Mode()&0o111 == 0 {
		t.Fatal("binary not placed/executable")
	}
}

func TestSingleProviderInstall(t *testing.T) {
	for _, id := range providers.ProviderIds() {
		project := t.TempDir()
		written, err := Install(Options{Project: project, Providers: []string{id}, SelfPath: fakeBinary(t)})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(sortStrings(written), sortStrings(expectedProject(t, project, []string{id}))) {
			t.Fatalf("%s: written mismatch", id)
		}
		prov := providers.PROVIDERS[id]
		// Own wrapper and persona cast exist.
		if _, err := os.Stat(filepath.Join(prov.ProjectAgentDir(project), prov.Filename("coder"))); err != nil {
			t.Fatalf("%s: own persona cast missing", id)
		}
		// Other providers' dirs not created.
		for _, other := range providers.ProviderIds() {
			if other == id {
				continue
			}
			op := providers.PROVIDERS[other]
			if op.ProjectAgentDir(project) != prov.ProjectAgentDir(project) {
				if _, err := os.Stat(filepath.Join(op.ProjectAgentDir(project), op.Filename("coder"))); err == nil {
					t.Fatalf("%s install must not materialize %s persona casts", id, other)
				}
			}
		}
		// Provider tag seeded.
		b, _ := os.ReadFile(filepath.Join(project, ".tomo", "settings.json"))
		if string(b) != "{\n  \"provider\": \""+id+"\"\n}\n" {
			t.Fatalf("%s: settings = %q", id, b)
		}
	}
}

func TestSeedingProviderKeepsExistingKeys(t *testing.T) {
	project := t.TempDir()
	sp := filepath.Join(project, ".tomo", "settings.json")
	os.MkdirAll(filepath.Dir(sp), 0o755)
	os.WriteFile(sp, []byte("{\n  \"gates\": \"prompt\",\n  \"personas\": {\n    \"map\": {\n      \"plan\": \"security\"\n    }\n  }\n}\n"), 0o644)
	if _, err := Install(Options{Project: project, Providers: []string{"claude"}, SelfPath: fakeBinary(t)}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(sp)
	s := string(b)
	if !strings.Contains(s, `"provider": "claude"`) || !strings.Contains(s, `"gates": "prompt"`) || !strings.Contains(s, `"plan": "security"`) {
		t.Fatalf("seeded settings = %s", s)
	}
}

func TestReinstallIdempotent(t *testing.T) {
	project := t.TempDir()
	bin := fakeBinary(t)
	if _, err := Install(Options{Project: project, SelfPath: bin}); err != nil {
		t.Fatal(err)
	}
	before := snapshot(project)
	if _, err := Install(Options{Project: project, SelfPath: bin}); err != nil {
		t.Fatal(err)
	}
	after := snapshot(project)
	if !reflect.DeepEqual(after, before) {
		t.Fatal("reinstall must not change installed files")
	}
}

func snapshot(project string) map[string]string {
	out := map[string]string{}
	filepath.Walk(project, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		b, _ := os.ReadFile(p)
		out[strings.TrimPrefix(p, project)] = string(b)
		return nil
	})
	return out
}

func TestGlobalInstallCopies(t *testing.T) {
	g := t.TempDir()
	gd := map[string]string{
		"opencode":            filepath.Join(g, "opencode", "command"),
		"claude":              filepath.Join(g, "claude", "commands", "tomo"),
		"copilot":             filepath.Join(g, "copilot", "skills"),
		"antigravity":         filepath.Join(g, "agy", "skills"),
		"codex":               filepath.Join(g, "codex", "skills"),
		"pi":                  filepath.Join(g, "pi", "prompts"),
		"opencodeAgents":      filepath.Join(g, "opencode", "agents"),
		"claudeAgents":        filepath.Join(g, "claude", "agents"),
		"copilotAgents":       filepath.Join(g, "copilot", "agents"),
		"antigravityAgents":   filepath.Join(g, "agy", "agents"),
		"codexAgents":         filepath.Join(g, "codex", "agents"),
		"piAgents":            filepath.Join(g, "pi", "agents"),
		"antigravitySettings": filepath.Join(g, "agy", "settings.json"),
		"opencodeConfig":      filepath.Join(g, "opencode"),
		"tomo":                filepath.Join(g, "tomo"),
	}
	if _, err := Install(Options{Project: t.TempDir(), Global: true, GlobalDirs: gd, SelfPath: fakeBinary(t)}); err != nil {
		t.Fatal(err)
	}
	all, err := render.GenerateAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, cmd := range mustCommands(t) {
		for _, id := range providers.ProviderIds() {
			prov := providers.PROVIDERS[id]
			dst := filepath.Join(gd[id], prov.CommandFilename(cmd.Name))
			info, err := os.Lstat(dst)
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				t.Fatalf("%s: not a regular file (must be a copy, not a symlink)", dst)
			}
			got, _ := os.ReadFile(dst)
			if string(got) != all[render.ProviderCommandName(cmd.Name, id)] {
				t.Fatalf("%s: content mismatch", dst)
			}
		}
	}
	// opencode + antigravity grants seeded.
	ocConfig, _ := os.ReadFile(filepath.Join(g, "opencode", "opencode.json"))
	if !strings.Contains(string(ocConfig), providers.HomeRelative(filepath.Join(g, "tomo"))+"/**") {
		t.Fatalf("opencode grant not seeded: %s", ocConfig)
	}
	agySettings, _ := os.ReadFile(filepath.Join(g, "agy", "settings.json"))
	if !strings.Contains(string(agySettings), "read_file("+filepath.Join(g, "tomo")+")") {
		t.Fatalf("antigravity grant not seeded: %s", agySettings)
	}
}

func TestSeedAntigravityGrants(t *testing.T) {
	root := t.TempDir()
	sp := filepath.Join(root, "settings.json")
	tomoDir := filepath.Join(root, "tomo")
	first, err := SeedAntigravityGrants(sp, tomoDir)
	if err != nil || !first.Changed {
		t.Fatalf("first seed: %v %v", first.Changed, err)
	}
	second, _ := SeedAntigravityGrants(sp, tomoDir)
	if second.Changed {
		t.Fatal("repeat seed must be no-op")
	}
	b, _ := os.ReadFile(sp)
	want := "{\n  \"permissions\": {\n    \"allow\": [\n      \"read_file(" + tomoDir + ")\"\n    ]\n  }\n}\n"
	if string(b) != want {
		t.Fatalf("antigravity settings = %q, want %q", b, want)
	}
}

func TestSeedOpenCodeGrantsJSONC(t *testing.T) {
	root := t.TempDir()
	tomoDir := filepath.Join(t.TempDir(), ".config", "tomo")
	pattern := providers.HomeRelative(tomoDir) + "/**"

	// Created when absent, idempotent.
	fresh := filepath.Join(root, "fresh")
	a, err := SeedOpenCodeGrants(fresh, tomoDir)
	if err != nil || !a.Changed {
		t.Fatalf("fresh seed: %v %v", a.Changed, err)
	}
	if a.Pattern != pattern {
		t.Fatalf("pattern = %q, want %q", a.Pattern, pattern)
	}
	b, _ := SeedOpenCodeGrants(fresh, tomoDir)
	if b.Changed {
		t.Fatal("fresh re-seed must be no-op")
	}

	// Trailing commas tolerated; existing rules keep order, grant lands last.
	trailing := filepath.Join(root, "trailing")
	os.MkdirAll(trailing, 0o755)
	os.WriteFile(filepath.Join(trailing, "opencode.json"), []byte("{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"permission\": { \"external_directory\": \"ask\", \"bash\": \"allow\", },\n}\n"), 0o644)
	c, _ := SeedOpenCodeGrants(trailing, tomoDir)
	if !c.Changed {
		t.Fatal("trailing seed should change")
	}
	cfg, _ := os.ReadFile(filepath.Join(trailing, "opencode.json"))
	if !strings.Contains(string(cfg), `"*": "ask"`) || !strings.Contains(string(cfg), `"`+pattern+`": "allow"`) {
		t.Fatalf("trailing config = %s", cfg)
	}
	if !strings.Contains(string(cfg), `"bash": "allow"`) {
		t.Fatalf("bash rule lost: %s", cfg)
	}

	// Comments: never rewritten.
	commented := filepath.Join(root, "commented")
	os.MkdirAll(commented, 0o755)
	src := "{\n  // keep me\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n"
	os.WriteFile(filepath.Join(commented, "opencode.jsonc"), []byte(src), 0o644)
	d, _ := SeedOpenCodeGrants(commented, tomoDir)
	if d.Changed || !d.Manual || !strings.Contains(d.Snippet, pattern) {
		t.Fatalf("commented config should be manual: %+v", d)
	}
	if got, _ := os.ReadFile(filepath.Join(commented, "opencode.jsonc")); string(got) != src {
		t.Fatal("commented config must not be rewritten")
	}

	// Strings containing comment/comma lookalikes survive.
	v, _, err := ParseJsonc(`{"a": "x // y, }", "b": [1,],}`)
	if err != nil {
		t.Fatal(err)
	}
	if v["a"] != "x // y, }" {
		t.Fatalf("a = %v", v["a"])
	}
	if !reflect.DeepEqual(v["b"], []any{float64(1)}) {
		t.Fatalf("b = %v", v["b"])
	}

	// Block comments are stripped fully (no leaked closing '/').
	vb, _, err := ParseJsonc("{\n  /* block */\n  \"a\": 1\n}")
	if err != nil {
		t.Fatalf("block comment parse: %v", err)
	}
	if !reflect.DeepEqual(vb["a"], float64(1)) {
		t.Fatalf("block comment a = %v", vb["a"])
	}
}
