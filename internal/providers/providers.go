// Package providers is the single onboarding point for a supported AI coding
// harness: where its agent files live (project + user scope) and how a
// canonical persona profile is cast into that harness's agent format.
//
// This is a faithful port of src/lib/providers.js.
package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PersonaFM is the frontmatter of a canonical persona profile.
type PersonaFM struct {
	Name        string
	Description string
	Tools       string
	Focus       string
}

// Persona is a parsed canonical persona profile.
type Persona struct {
	Name string
	FM   PersonaFM
	Body string
}

// CommandFM is the frontmatter of a command contract.
type CommandFM struct {
	Name        string
	Description string
	Args        string
	Gate        string
}

// Command is a parsed command contract with its inlined body.
type Command struct {
	Name string
	FM   CommandFM
	Body string
}

// Personas that review rather than write: their casts deny edits.
var READONLY_PERSONAS = map[string]bool{"reviewer": true, "security": true}

func configBase() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return x
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func homeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// TomoConfigDir is the tomo config dir (global settings.json lives here).
func TomoConfigDir() string {
	return filepath.Join(configBase(), "tomo")
}

// HomeRelative returns the home-relative (`~/...`) form for permission
// patterns, falling back to the absolute path outside $HOME.
func HomeRelative(p string) string {
	home := homeDir()
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		rel, _ := filepath.Rel(home, p)
		return "~/" + filepath.ToSlash(rel)
	}
	return p
}

// ClaudeReadGrant is the Claude Code allowed-tools grant for the tomo config
// dir.
func ClaudeReadGrant() string {
	rel := HomeRelative(TomoConfigDir())
	if !strings.HasPrefix(rel, "~") {
		rel = "/" + rel
	}
	return "Read(" + rel + "/**)"
}

// Provider describes one harness: its dirs, filenames, and cast renderers.
type Provider struct {
	ID    string
	Label string

	ProjectCommandDir func(project string) string
	GlobalCommandDir  func() string
	ProjectAgentDir   func(project string) string
	GlobalAgentDir    func() string
	GlobalConfigDir   func() string // optional (opencode)
	SettingsPath      func() string // optional (antigravity)

	CommandFilename func(name string) string
	Filename        func(name string) string

	CommandCast func(cmd Command) string // optional; nil reuses the plain wrapper
	Cast        func(p Persona) string
}

var PROVIDERS = map[string]*Provider{
	"opencode": {
		ID:    "opencode",
		Label: "OpenCode",
		ProjectCommandDir: func(project string) string {
			return filepath.Join(project, ".opencode", "command")
		},
		GlobalCommandDir: func() string { return filepath.Join(configBase(), "opencode", "command") },
		ProjectAgentDir: func(project string) string {
			return filepath.Join(project, ".opencode", "agents")
		},
		GlobalAgentDir:  func() string { return filepath.Join(configBase(), "opencode", "agents") },
		GlobalConfigDir: func() string { return filepath.Join(configBase(), "opencode") },
		CommandFilename: func(name string) string { return "tomo-" + name + ".md" },
		Filename:        func(name string) string { return name + ".md" },
		Cast:            CastOpenCode,
	},
	"claude": {
		ID:    "claude",
		Label: "Claude Code",
		ProjectCommandDir: func(project string) string {
			return filepath.Join(project, ".claude", "commands", "tomo")
		},
		GlobalCommandDir: func() string { return filepath.Join(homeDir(), ".claude", "commands", "tomo") },
		ProjectAgentDir: func(project string) string {
			return filepath.Join(project, ".claude", "agents")
		},
		GlobalAgentDir:  func() string { return filepath.Join(homeDir(), ".claude", "agents") },
		CommandFilename: func(name string) string { return name + ".md" },
		Filename:        func(name string) string { return name + ".md" },
		CommandCast:     CastClaudeCommand,
		Cast:            CastClaude,
	},
	"copilot": {
		ID:    "copilot",
		Label: "GitHub Copilot",
		ProjectCommandDir: func(project string) string {
			return filepath.Join(project, ".github", "skills")
		},
		GlobalCommandDir: func() string { return filepath.Join(homeDir(), ".copilot", "skills") },
		ProjectAgentDir: func(project string) string {
			return filepath.Join(project, ".github", "agents")
		},
		GlobalAgentDir:  func() string { return filepath.Join(homeDir(), ".copilot", "agents") },
		CommandFilename: func(name string) string { return "tomo-" + name + "/SKILL.md" },
		Filename:        func(name string) string { return name + ".agent.md" },
		CommandCast:     CastCopilotCommand,
		Cast:            CastCopilot,
	},
	"antigravity": {
		ID:    "antigravity",
		Label: "Google Antigravity",
		ProjectCommandDir: func(project string) string {
			return filepath.Join(project, ".agents", "skills")
		},
		GlobalCommandDir: func() string { return filepath.Join(homeDir(), ".gemini", "antigravity-cli", "skills") },
		ProjectAgentDir: func(project string) string {
			return filepath.Join(project, ".agents", "agents")
		},
		GlobalAgentDir:  func() string { return filepath.Join(homeDir(), ".gemini", "config", "agents") },
		SettingsPath:    func() string { return filepath.Join(homeDir(), ".gemini", "antigravity-cli", "settings.json") },
		CommandFilename: func(name string) string { return "tomo-" + name + "/SKILL.md" },
		Filename:        func(name string) string { return name + "/agent.md" },
		CommandCast:     CastAntigravityCommand,
		Cast:            CastAntigravity,
	},
	"codex": {
		ID:    "codex",
		Label: "OpenAI Codex",
		ProjectCommandDir: func(project string) string {
			return filepath.Join(project, ".agents", "skills")
		},
		GlobalCommandDir: func() string { return filepath.Join(homeDir(), ".agents", "skills") },
		ProjectAgentDir: func(project string) string {
			return filepath.Join(project, ".codex", "agents")
		},
		GlobalAgentDir:  func() string { return filepath.Join(homeDir(), ".codex", "agents") },
		CommandFilename: func(name string) string { return "tomo-" + name + "/SKILL.md" },
		Filename:        func(name string) string { return name + ".toml" },
		CommandCast:     CastAntigravityCommand,
		Cast:            CastCodex,
	},
	"pi": {
		ID:    "pi",
		Label: "Pi",
		ProjectCommandDir: func(project string) string {
			return filepath.Join(project, ".pi", "prompts")
		},
		GlobalCommandDir: func() string { return filepath.Join(homeDir(), ".pi", "agent", "prompts") },
		ProjectAgentDir: func(project string) string {
			return filepath.Join(project, ".pi", "agents")
		},
		GlobalAgentDir: func() string { return filepath.Join(homeDir(), ".pi", "agent", "agents") },
		CommandFilename: func(name string) string { return "tomo-" + name + ".md" },
		Filename:        func(name string) string { return name + ".md" },
		Cast:            CastPi,
	},
}

// ProviderOrder is the stable provider id order (Go map iteration is random).
var ProviderOrder = []string{"opencode", "claude", "copilot", "antigravity", "codex", "pi"}

func ProviderIds() []string {
	return append([]string{}, ProviderOrder...)
}

func IsProvider(id string) bool {
	_, ok := PROVIDERS[id]
	return ok
}

// ParseProviderSpec parses a --provider spec: 'all'|” → every provider;
// comma-separated ids. Unknown ids error.
func ParseProviderSpec(spec string) ([]string, error) {
	raw := strings.TrimSpace(spec)
	if raw == "" || raw == "all" {
		return ProviderIds(), nil
	}
	var ids []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		ids = append(ids, s)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("tomo: --provider needs a value: all | <id>[,<id>...]")
	}
	for _, id := range ids {
		if !IsProvider(id) {
			return nil, fmt.Errorf("tomo: unknown provider %q (have: %s)", id, strings.Join(ProviderIds(), ", "))
		}
	}
	return ids, nil
}

// SeedSettingsProvider seeds the effective harness tag into a project's
// .tomo/settings.json, keeping any other keys. Only a single unambiguous
// provider is written.
func SeedSettingsProvider(project string, ids []string) (changed bool, settingsPath string, err error) {
	settingsPath = filepath.Join(project, ".tomo", "settings.json")
	current := map[string]any{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &current); err != nil {
			return false, settingsPath, fmt.Errorf("tomo: cannot parse %s: %v", settingsPath, err)
		}
	}
	next := ""
	if len(ids) == 1 {
		next = ids[0]
	} else if v, ok := current["provider"].(string); ok {
		next = v
	}
	curProvider, _ := current["provider"].(string)
	if next == curProvider {
		return false, settingsPath, nil
	}
	current["provider"] = next
	buf, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return false, settingsPath, err
	}
	buf = append(buf, '\n')
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return false, settingsPath, err
	}
	if err := os.WriteFile(settingsPath, buf, 0o644); err != nil {
		return false, settingsPath, err
	}
	return true, settingsPath, nil
}
