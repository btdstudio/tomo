// Package cli implements the `tomo` runtime-helper subcommands that the command
// bodies invoke in place of the old .tomo/lib/*.js modules.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/btdstudio/tomo/internal/issues"
	"github.com/btdstudio/tomo/internal/personas"
	"github.com/btdstudio/tomo/internal/providers"
	"github.com/btdstudio/tomo/internal/render"
	"github.com/btdstudio/tomo/internal/settings"
	"github.com/btdstudio/tomo/internal/state"
)

func cwd() string {
	wd, _ := os.Getwd()
	return wd
}

func marshal(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b) + "\n"
}

// Settings prints the effective settings as JSON.
func Settings(root string) (string, error) {
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		return "", err
	}
	return marshal(loaded.Effective), nil
}

type stateOutput struct {
	Change    string      `json:"change"`
	Phase     string      `json:"phase"`
	Status    string      `json:"status"`
	Gates     state.Gates `json:"gates"`
	Sources   []string    `json:"sources"`
	UpdatedAt string      `json:"updatedAt"`
}

// State prints the active machine as JSON.
func State(root string) (string, error) {
	st, err := state.LoadState(root)
	if err != nil {
		return "", err
	}
	return marshal(stateOutput{
		Change:    st.Change,
		Phase:     st.Phase,
		Status:    st.Status,
		Gates:     st.Gates,
		Sources:   st.Sources,
		UpdatedAt: st.UpdatedAt,
	}), nil
}

// StateChanges prints the known changes as JSON.
func StateChanges(root string) (string, error) {
	kc, err := state.KnownChanges(root)
	if err != nil {
		return "", err
	}
	if kc == nil {
		kc = []state.KnownChange{}
	}
	return marshal(kc), nil
}

// NextId prints the next change id.
func NextId(root string) (string, error) {
	return strconv.Itoa(state.NextChangeId(root)) + "\n", nil
}

// Slug prints the kebab-case slug.
func Slug(text string) string {
	return state.Slugify(text) + "\n"
}

// StartChange creates a new numbered change folder and points the workspace at
// it, returning the change name.
func StartChange(root, slug string) (string, error) {
	id := state.NextChangeId(root)
	change := fmt.Sprintf("%d-%s", id, state.Slugify(slug))
	if err := os.MkdirAll(state.ChangeDir(root, change), 0o755); err != nil {
		return "", err
	}
	st := state.DefaultState()
	st.Change = change
	if err := state.SaveState(root, &st); err != nil {
		return "", err
	}
	return change + "\n", nil
}

// SetGate transitions a phase and persists + logs.
func SetGate(root, phase, status string) error {
	st, err := state.LoadState(root)
	if err != nil {
		return err
	}
	if err := state.SetGate(&st, phase, status); err != nil {
		return err
	}
	if err := state.SaveState(root, &st); err != nil {
		return err
	}
	return state.AppendLog(root, phase+" → "+status)
}

// SwitchChange switches the active change.
func SwitchChange(root, name string) error {
	_, err := state.SwitchTo(root, name)
	return err
}

// AppendLog appends a timestamped log line.
func AppendLog(root, line string) error {
	return state.AppendLog(root, line)
}

// Persona resolves the persona for a step and prints {id, mode, source, file}.
func Persona(root, step string) (string, error) {
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		return "", err
	}
	res := personas.ResolvePersona(step, personas.Overrides{
		Map:    loaded.Effective.Personas.Map,
		Custom: loaded.Effective.Personas.Custom,
	})
	return marshal(res), nil
}

// Step emits the composed, context-injected prompt for a command: a context
// block (change/gates, resolved persona, settings, artifacts) followed by the
// inlined body. With printRaw it emits only the raw canonical body.
func Step(root, name string, printRaw bool) (string, error) {
	commands, err := render.ReadCommands()
	if err != nil {
		return "", err
	}
	found := false
	for _, c := range commands {
		if c.Name == name {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(commands))
		for _, c := range commands {
			names = append(names, c.Name)
		}
		return "", fmt.Errorf("tomo: unknown command %q (have: %s)", name, strings.Join(names, ", "))
	}

	raw := render.ReadContentCommands()[name]
	if printRaw {
		return raw + "\n", nil
	}

	body, err := render.InlineBody(providers.Command{Name: name})
	if err != nil {
		return "", err
	}

	eff := settings.DefaultSettings()
	if loaded, err := settings.LoadSettings(root); err == nil {
		eff = loaded.Effective
	}
	res := personas.ResolvePersona(name, personas.Overrides{
		Map:    eff.Personas.Map,
		Custom: eff.Personas.Custom,
	})
	st, err := state.LoadState(root)
	if err != nil {
		st = state.DefaultState()
	}

	personaLine := "(none)"
	if res.ID != "" {
		personaLine = fmt.Sprintf("%s (mode=%s, file=%s)", res.ID, res.Mode, res.File)
	}
	change := st.Change
	if change == "" {
		change = "(none)"
	}
	phase := st.Phase
	if phase == "" {
		phase = "(none)"
	} else if st.Status != "" {
		phase = fmt.Sprintf("%s (%s)", phase, st.Status)
	}
	base := state.ChangeDir(root, st.Change)
	mark := func(n string) string {
		if _, err := os.Stat(filepath.Join(base, n)); err == nil {
			return "✓"
		}
		return "✗"
	}

	var sb strings.Builder
	sb.WriteString("<!-- tomo context -->\n")
	fmt.Fprintf(&sb, "change: %s\n", change)
	fmt.Fprintf(&sb, "phase: %s\n", phase)
	fmt.Fprintf(&sb, "gates: requirements=%s design=%s plan=%s implement=%s\n",
		st.Gates.Requirements, st.Gates.Design, st.Gates.Plan, st.Gates.Implement)
	fmt.Fprintf(&sb, "persona: %s\n", personaLine)
	fmt.Fprintf(&sb, "artifacts: requirements.md %s, design.md %s, tasks.json %s\n",
		mark("requirements.md"), mark("design.md"), mark("tasks.json"))
	fmt.Fprintf(&sb, "settings: gates=%s issues=%s issuesDir=%s\n", eff.Gates, eff.Issues, eff.IssuesDir)
	sb.WriteString("\n")
	sb.WriteString(body)
	return sb.String(), nil
}

// IssueList prints the local issues.
func IssueList(root, issuesDir string) (string, error) {
	list := issues.ListIssues(root, issuesDir)
	return marshal(list), nil
}

// IssueShow prints one local issue.
func IssueShow(root, issuesDir string, id int) (string, error) {
	issue, err := issues.ReadIssue(root, issuesDir, id)
	if err != nil {
		return "", err
	}
	return marshal(issue), nil
}

// IssueNew creates a local issue and prints it.
func IssueNew(root, issuesDir string, opts issues.NewIssueOptions) (string, error) {
	issue, err := issues.WriteNewIssue(root, issuesDir, opts)
	if err != nil {
		return "", err
	}
	return marshal(issue), nil
}

// Usage is the top-level help text.
const Usage = `tomo — harness-agnostic AI SDD toolset

Usage:
  tomo install [--global] [--project <dir>] [--provider <ids|all>]
  tomo render [--out <dir>]      dump the rendered files (debug)
  tomo check                     validate the content store + vocabulary
  tomo session-hook              print the Claude resume anchor
  tomo settings                  effective settings as JSON
  tomo state                     active machine as JSON
  tomo state changes             known changes as JSON
  tomo state next-id             next change id
  tomo state start <slug>        create a new change folder and activate it
  tomo state set-gate <phase> <status>
  tomo state switch <name>       switch the active change
  tomo slug <text>               kebab-case slug
  tomo log <line>                append a timestamped log line
  tomo persona <step>            resolve the persona for a step
  tomo step <name> [--print]     composed, context-injected prompt for a command
  tomo issue list                local issues
  tomo issue show <id>           one local issue
  tomo issue new --title --labels --body
`

// Run dispatches the runtime-helper subcommands. It returns the exit code.
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, Usage)
		return 1
	}
	root := cwd()
	var out string
	var err error
	switch args[0] {
	case "settings":
		out, err = Settings(root)
	case "state":
		out, err = runState(root, args[1:])
	case "slug":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "tomo slug <text>")
			return 1
		}
		out = Slug(args[1])
	case "log":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "tomo log <line>")
			return 1
		}
		err = AppendLog(root, args[1])
	case "persona":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "tomo persona <step>")
			return 1
		}
		out, err = Persona(root, args[1])
	case "step":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "tomo step <name> [--print]")
			return 1
		}
		printRaw := len(args) > 2 && args[2] == "--print"
		out, err = Step(root, args[1], printRaw)
	case "issue":
		out, err = runIssue(root, args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(os.Stdout, Usage)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "tomo: unknown command %q (see `tomo help`)\n", args[0])
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprint(os.Stdout, out)
	return 0
}

func runState(root string, args []string) (string, error) {
	if len(args) == 0 {
		return State(root)
	}
	switch args[0] {
	case "changes":
		return StateChanges(root)
	case "next-id":
		return NextId(root)
	case "start":
		if len(args) < 2 {
			return "", fmt.Errorf("tomo state start <slug>")
		}
		return StartChange(root, args[1])
	case "set-gate":
		if len(args) < 3 {
			return "", fmt.Errorf("tomo state set-gate <phase> <status>")
		}
		return "", SetGate(root, args[1], args[2])
	case "switch":
		if len(args) < 2 {
			return "", fmt.Errorf("tomo state switch <name>")
		}
		return "", SwitchChange(root, args[1])
	default:
		return "", fmt.Errorf("tomo: unknown state command %q", args[0])
	}
}

func runIssue(root string, args []string) (string, error) {
	issuesDir := ""
	if loaded, err := settings.LoadSettings(root); err == nil {
		issuesDir = loaded.Effective.IssuesDir
	}
	if len(args) == 0 {
		return IssueList(root, issuesDir)
	}
	switch args[0] {
	case "list":
		return IssueList(root, issuesDir)
	case "show":
		if len(args) < 2 {
			return "", fmt.Errorf("tomo issue show <id>")
		}
		id, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("tomo issue show <id>: %v", err)
		}
		return IssueShow(root, issuesDir, id)
	case "new":
		opts := issues.NewIssueOptions{}
		rest := args[1:]
		for i := 0; i < len(rest); i++ {
			switch rest[i] {
			case "--title":
				if i+1 < len(rest) {
					opts.Title = rest[i+1]
					i++
				}
			case "--labels":
				if i+1 < len(rest) {
					for _, l := range strings.Split(rest[i+1], ",") {
						if s := strings.TrimSpace(l); s != "" {
							opts.Labels = append(opts.Labels, s)
						}
					}
					i++
				}
			case "--body":
				if i+1 < len(rest) {
					opts.Body = rest[i+1]
					i++
				}
			}
		}
		return IssueNew(root, issuesDir, opts)
	default:
		return "", fmt.Errorf("tomo: unknown issue command %q", args[0])
	}
}
