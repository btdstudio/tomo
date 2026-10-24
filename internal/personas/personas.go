// Package personas owns the persona contract: the built-in persona ids, the
// default step→persona map, and runtime resolution of which persona runs a
// step (defaults overridden by settings.personas.map).
//
// This is a faithful port of src/lib/personas.js.
package personas

// The built-in personas; ids match the canonical files in src/personas/.
var PERSONAS = []string{"analyst", "architect", "coder", "reviewer", "security"}

// Which steps spawn the persona as a subagent vs adopt it in-session.
var spawnSteps = map[string]bool{
	"implement":          true,
	"implement-review":   true,
	"implement-security": true,
}

// Default step → persona binding. Settings overrides win per step.
var DEFAULT_PERSONA_MAP = map[string]string{
	"requirements":       "analyst",
	"design":             "architect",
	"plan":               "architect",
	"implement":          "coder",
	"implement-review":   "reviewer",
	"implement-security": "security",
	"arch":               "architect",
	"trace":              "reviewer",
	"debt":               "architect",
}

// Overrides is the settings.personas slice: step→persona (map) and alias→file
// (custom).
type Overrides struct {
	Map    map[string]string
	Custom map[string]string
}

// Resolved is the persona resolution result. Field order matches the JS
// { id, mode, source, file } shape for stable JSON.
type Resolved struct {
	ID     string `json:"id"`
	Mode   string `json:"mode"`
	Source string `json:"source"`
	File   string `json:"file"`
}

func IsBuiltIn(id string) bool {
	for _, p := range PERSONAS {
		if p == id {
			return true
		}
	}
	return false
}

// ModeForStep reports 'spawn' for implement-loop steps, else 'lens'.
func ModeForStep(step string) string {
	if spawnSteps[step] {
		return "spawn"
	}
	return "lens"
}

// NeutralPath is the lens-adoption profile path for a built-in persona.
func NeutralPath(id string) string {
	return ".tomo/personas/" + id + ".md"
}

// ResolvePersona resolves which persona drives step under the merged overrides.
// Custom resolution order: built-in id ⇒ alias in custom ⇒ raw path.
func ResolvePersona(step string, o Overrides) Resolved {
	value := o.Map[step]
	if value == "" {
		id := DEFAULT_PERSONA_MAP[step]
		file := ""
		if id != "" {
			file = NeutralPath(id)
		}
		return Resolved{ID: id, Mode: ModeForStep(step), Source: "default", File: file}
	}
	if IsBuiltIn(value) {
		return Resolved{ID: value, Mode: ModeForStep(step), Source: "override", File: NeutralPath(value)}
	}
	if alias, ok := o.Custom[value]; ok {
		return Resolved{ID: value, Mode: ModeForStep(step), Source: "custom", File: alias}
	}
	return Resolved{ID: value, Mode: ModeForStep(step), Source: "custom", File: value}
}
