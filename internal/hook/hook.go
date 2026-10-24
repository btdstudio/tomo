// Package hook emits the Claude Code SessionStart resume anchor: a one-line
// tomo orientation when the project has a .tomo/ workspace. Silent otherwise.
//
// This is a faithful port of adapters/claude/hooks/tomo-session.js.
package hook

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/btdstudio/tomo/internal/state"
)

// SessionMessage returns the resume anchor for the given project root (cwd), or
// "" when there is no .tomo/ workspace to report.
func SessionMessage(root string) string {
	if _, err := os.Stat(filepath.Join(root, ".tomo", "state.json")); err != nil {
		return ""
	}
	st, err := state.LoadState(root)
	if err != nil {
		return ""
	}
	phase := st.Phase
	if phase == "" {
		phase = "no phase"
	}
	status := st.Status

	gated := []string{}
	for _, g := range []string{"requirements", "design", "plan"} {
		if gateVal(st, g) != "approved" {
			gated = append(gated, g)
		}
	}

	var resume string
	switch {
	case len(gated) > 0:
		g := gated[0]
		v := gateVal(st, g)
		if v == "" {
			v = "not started"
		}
		resume = fmt.Sprintf("next gate: %s (%s)", g, v)
	case st.Gates.Implement == "complete":
		resume = "spec complete — /tomo commit or /tomo switch"
	case st.Gates.Implement == "in-progress":
		resume = "implement in-progress — /tomo implement resumes"
	default:
		resume = "all gates approved — /tomo implement to run the plan"
	}
	return fmt.Sprintf("Tomo: %s (%s) — %s", phase, status, resume)
}

func gateVal(st state.State, phase string) string {
	switch phase {
	case "requirements":
		return st.Gates.Requirements
	case "design":
		return st.Gates.Design
	case "plan":
		return st.Gates.Plan
	case "implement":
		return st.Gates.Implement
	}
	return ""
}
