package personas

import (
	"reflect"
	"strings"
	"testing"
)

func empty() Overrides { return Overrides{Map: map[string]string{}, Custom: map[string]string{}} }

func TestResolvePersona(t *testing.T) {
	cases := []struct {
		step string
		o    Overrides
		want Resolved
	}{
		{"requirements", empty(), Resolved{"analyst", "lens", "default", ".tomo/personas/analyst.md"}},
		{"implement", empty(), Resolved{"coder", "spawn", "default", ".tomo/personas/coder.md"}},
		{"implement-security", empty(), Resolved{"security", "spawn", "default", ".tomo/personas/security.md"}},
		{"plan", Overrides{Map: map[string]string{"plan": "security"}, Custom: map[string]string{}}, Resolved{"security", "lens", "override", ".tomo/personas/security.md"}},
		{"design", Overrides{Map: map[string]string{"design": "lead"}, Custom: map[string]string{"lead": "personas/lead.md"}}, Resolved{"lead", "lens", "custom", "personas/lead.md"}},
		{"trace", Overrides{Map: map[string]string{"trace": "personas/local.md"}, Custom: map[string]string{}}, Resolved{"personas/local.md", "lens", "custom", "personas/local.md"}},
		{"bogus", empty(), Resolved{"", "lens", "default", ""}},
	}
	for _, c := range cases {
		if got := ResolvePersona(c.step, c.o); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("resolvePersona(%q) = %+v, want %+v", c.step, got, c.want)
		}
	}
}

func TestModeForStepAndIsBuiltIn(t *testing.T) {
	if !IsBuiltIn("coder") {
		t.Fatal("coder should be built-in")
	}
	if IsBuiltIn("lead") {
		t.Fatal("lead should not be built-in")
	}
	for step := range DEFAULT_PERSONA_MAP {
		want := "lens"
		if step == "implement" || strings.HasPrefix(step, "implement-") {
			want = "spawn"
		}
		if ModeForStep(step) != want {
			t.Fatalf("modeForStep(%q) = %q, want %q", step, ModeForStep(step), want)
		}
	}
}
