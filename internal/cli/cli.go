// Package cli implements the tomo workspace helpers: status, settings, switch,
// issues, commit, pr, help.
package cli

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/btdstudio/kiso/repos"
	kiso "github.com/btdstudio/kiso/state"
	"github.com/btdstudio/kiso/ticket"
	"github.com/btdstudio/kiso/version"
	"github.com/btdstudio/tomo/internal/icm"
	"github.com/btdstudio/tomo/internal/issues"
	"github.com/btdstudio/tomo/internal/settings"
	"github.com/btdstudio/tomo/internal/state"
)

// Run dispatches a helper subcommand and returns a process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		PrintUsage()
		return 1
	}
	root := "."
	switch args[0] {
	case "help", "-h", "--help":
		PrintUsage()
		return 0
	case "status":
		return runStatus(root)
	case "settings":
		return runSettings(root)
	case "switch":
		return runSwitch(root, args[1:])
	case "change":
		return runChange(root, args[1:])
	case "gate":
		return runGate(root, args[1:])
	case "reopen":
		return runReopen(root, args[1:])
	case "issues":
		return runIssues(root, args[1:])
	case "doctor":
		return runDoctor(root, args[1:])
	case "list", "ls":
		return runList(root)
	case "commit":
		return runCommit(root, args[1:])
	case "pr":
		return runPR(root, args[1:])
	case "repo":
		return runRepo(root, args[1:])
	case "env":
		return runEnv(root)
	default:
		fmt.Fprintf(os.Stderr, "tomo: unknown command %q\n", args[0])
		PrintUsage()
		return 1
	}
}

// PrintUsage prints the tomo usage summary.
func PrintUsage() {
	fmt.Print(`tomo — folder-driven SDLC tool

Usage:
  tomo init [dir] [--no-git] [--force]   scaffold the ICM structure
  tomo update                             refresh managed files
  tomo add kumi|hanga <name> --path <p> --remote <url> [module]
                                          scaffold an app outside the workspace and target it
  tomo status                             show the active change
  tomo settings                           show effective settings
  tomo switch [--all] [<change>]          list changes, or switch the active one
  tomo change new <title> [--issue <id>] [--body <b>]
                                          create a change + native ticket + switch
  tomo change new --from <ticket.md>      start a change from a planned ticket
  tomo change close <change> --reason <r> close a change
  tomo gate <phase> <status> [--force]    advance the active change's gate
  tomo reopen <phase> --reason <r>        move back to a phase, clearing later approvals
  tomo issues new|list|show [args]        manage 00_issues/
` + repos.Usage("tomo") + `  tomo env                                print harness flags for the target repo
  tomo doctor [--install]                 check toolchain prerequisites
  tomo list                               list installed tools + versions
  tomo commit [git commit args]           commit what's staged in the target repo
  tomo pr [title]                         open a pull request for the target repo, linking the ticket
  tomo help                               this help
`)
}

func runStatus(root string) int {
	st, err := state.LoadState(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if st.Name == "" {
		fmt.Println("no active change")
		return 0
	}
	fmt.Printf("change:   %s\n", st.Name)
	fmt.Printf("phase:    %s\n", st.Phase)
	fmt.Printf("status:   %s\n", st.Status)
	var gates []string
	for _, p := range state.Machine.Phases {
		gates = append(gates, p.Name+"="+st.Gates[p.Name])
	}
	fmt.Printf("gates:    %s\n", strings.Join(gates, " "))
	if t := changeTicket(root, st); t != nil {
		fmt.Printf("ticket:   %s %s\n", t.ID, t.Title)
		fmt.Printf("branch:   %s\n", branchName(t))
	}
	return 0
}

func runSettings(root string) int {
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	e := loaded.Effective
	fmt.Printf("baseSource:  %s\n", e.BaseSource)
	fmt.Printf("gates:       %s\n", e.Gates)
	fmt.Printf("artifactDir: %s\n", e.ArtifactDir)
	fmt.Printf("issuesDir:   %s\n", e.IssuesDir)
	fmt.Printf("syncFile:    %s\n", e.SyncFile)
	for k, v := range e.AppSources {
		fmt.Printf("appSource.%s: %s\n", k, v)
	}
	return 0
}

func runSwitch(root string, args []string) int {
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	artifactDir := loaded.Effective.ArtifactDir
	all := len(args) > 0 && args[0] == "--all"
	if all {
		args = args[1:]
	}
	if len(args) == 0 {
		changes, err := state.KnownChanges(root, artifactDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if len(changes) == 0 {
			fmt.Println("no changes yet — run `tomo change new <title>`")
			return 0
		}
		for _, c := range changes {
			if c.Status == kiso.Closed && !all {
				continue
			}
			fmt.Printf("%s  %s %s\n", c.Name, c.Phase, c.Status)
		}
		return 0
	}
	st, err := state.SwitchTo(root, args[0], artifactDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("switched to %s\n", st.Name)
	return 0
}

// gatePolicy decides whether a gate violation stops the command: "block" refuses
// it unless forced; "prompt", "auto" and a forced run warn and proceed.
func gatePolicy(gates string, violation error, force bool) error {
	if gates == "block" && !force {
		return fmt.Errorf("%v (pass --force to override)", violation)
	}
	fmt.Fprintf(os.Stderr, "warning: %v\n", violation)
	return nil
}

// parseTransition splits positional arguments from --reason and --force.
func parseTransition(args []string) (pos []string, reason string, force bool, err error) {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--force":
			force = true
		case a == "--reason" && i+1 < len(args):
			i++
			reason = args[i]
		case strings.HasPrefix(a, "-"):
			return nil, "", false, fmt.Errorf("unknown flag %q", a)
		default:
			pos = append(pos, a)
		}
	}
	return pos, reason, force, nil
}

func runChange(root string, args []string) int {
	usage := "usage: tomo change new <title> [--issue <id>] [--body <b>]\n       tomo change new --from <ticket.md>\n       tomo change close <change> --reason <r>"
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		return 1
	}
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	switch args[0] {
	case "new":
		if len(args) > 1 && args[1] == "--from" {
			if len(args) != 3 {
				fmt.Fprintln(os.Stderr, "usage: tomo change new --from <ticket.md>")
				return 1
			}
			return runChangeFrom(root, loaded.Effective, args[2])
		}
		return runChangeNew(root, loaded.Effective, args[1:])
	case "close":
		pos, reason, _, err := parseTransition(args[1:])
		if err != nil || len(pos) != 1 {
			fmt.Fprintln(os.Stderr, "usage: tomo change close <change> --reason <r>")
			return 1
		}
		st, err := state.CloseChange(root, loaded.Effective.ArtifactDir, pos[0], reason)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("%s: closed\n", st.Name)
		return 0
	}
	fmt.Fprintln(os.Stderr, usage)
	return 1
}

func runChangeNew(root string, e settings.Settings, args []string) int {
	usage := "usage: tomo change new <title> [--issue <id>] [--body <b>]"
	var title, body, issueID string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--issue" && i+1 < len(args):
			i++
			issueID = issues.NormalizeID(args[i])
		case a == "--body" && i+1 < len(args):
			i++
			body = args[i]
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "tomo change new: unknown flag %q\n", a)
			return 1
		case title != "":
			fmt.Fprintf(os.Stderr, "tomo change new: unexpected argument %q (quote a multi-word title)\n", a)
			return 1
		default:
			title = a
		}
	}
	if title == "" {
		fmt.Fprintln(os.Stderr, usage)
		return 1
	}
	// Fail on unreadable state before writing the issue and spec folder.
	if _, err := state.LoadState(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if issueID != "" {
		if _, err := issues.Read(root, e.IssuesDir, issueID); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	} else {
		target, err := icm.TargetName(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		it, err := issues.New(root, e.IssuesDir, issues.NewOptions{Title: title, Repo: target, Body: body})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		issueID = it.ID
	}
	return seedFromTicket(root, e, title, issueID)
}

// seedFromTicket records a change whose source is a ticket in the store.
func seedFromTicket(root string, e settings.Settings, title, id string) int {
	slug := state.Slugify(title)
	if slug == "" {
		slug = "change"
	}
	name := fmt.Sprintf("%d-%s", state.NextChangeId(root, e.ArtifactDir), slug)
	rel := filepath.ToSlash(filepath.Join(e.IssuesDir, id+".md"))
	st, err := state.SeedChange(root, name, e.ArtifactDir, []string{rel})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("created change %s (ticket %s)\n", st.Name, id)
	return 0
}

// runChangeFrom starts a change from a planned ticket: it validates the file, checks
// the ticket lands in this workspace's target repo, and snapshots it into the store.
func runChangeFrom(root string, e settings.Settings, path string) int {
	fail := func(err error) int {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := state.LoadState(root); err != nil {
		return fail(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fail(fmt.Errorf("tomo: cannot read %s: %v", path, err))
	}
	t, err := ticket.Parse(data)
	if err != nil {
		return fail(fmt.Errorf("tomo: %s: %v", path, err))
	}
	t.Path = path
	if problems := ticket.ValidateFile(t); len(problems) > 0 {
		return fail(fmt.Errorf("tomo: invalid ticket: %s", problems[0]))
	}
	if t.Status != ticket.StatusReady && t.Status != ticket.StatusPublished {
		return fail(fmt.Errorf("tomo: %s is %s; only a ready or published ticket can start a change", t.ID, t.Status))
	}
	target, err := icm.TargetName(root)
	if err != nil {
		return fail(err)
	}
	if t.Repo != target {
		return fail(fmt.Errorf("tomo: %s lands in repo %q, but this workspace targets %q", t.ID, t.Repo, target))
	}
	origin, err := ticketOrigin(path)
	if err != nil {
		return fail(err)
	}
	if _, err := issues.Import(root, e.IssuesDir, t, origin); err != nil {
		return fail(err)
	}
	return seedFromTicket(root, e, t.Title, t.ID)
}

// ticketOrigin says where a ticket file came from: the origin remote of the repo
// holding it, without credentials, and its path there. Outside a repo, or without a
// remote, the workspace is the folder's name, so no local path gets committed.
func ticketOrigin(path string) (ticket.Origin, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ticket.Origin{}, err
	}
	dir := filepath.Dir(abs)
	gitOut := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	top := gitOut("rev-parse", "--show-toplevel")
	if top == "" {
		return ticket.Origin{Workspace: filepath.Base(dir), Path: filepath.Base(abs)}, nil
	}
	// Resolve symlinks the way git did, so the relative path is right.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	rel, err := filepath.Rel(top, abs)
	if err != nil {
		return ticket.Origin{}, err
	}
	workspace := stripCredentials(gitOut("remote", "get-url", "origin"))
	if workspace == "" {
		workspace = filepath.Base(top)
	}
	return ticket.Origin{Workspace: workspace, Path: filepath.ToSlash(rel)}, nil
}

func stripCredentials(remote string) string {
	if u, err := url.Parse(remote); err == nil && u.User != nil && (u.Scheme == "http" || u.Scheme == "https") {
		u.User = nil
		return u.String()
	}
	return remote
}

// changeTicket returns the ticket a change started from, if its source is one.
func changeTicket(root string, st state.State) *ticket.Ticket {
	for _, src := range st.Sources {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(src)))
		if err != nil {
			continue
		}
		if t, err := ticket.Parse(data); err == nil && t.ID != "" {
			return t
		}
	}
	return nil
}

// branchName is the ship branch for a ticket, which includes its id.
func branchName(t *ticket.Ticket) string {
	name := strings.ToLower(t.ID)
	if slug := state.Slugify(t.Title); slug != "" {
		name += "-" + slug
	}
	return name
}

// prBody links a pull request to its ticket: id, external items, spec and ADRs.
func prBody(t *ticket.Ticket) string {
	lines := []string{fmt.Sprintf("Implements %s: %s", t.ID, t.Title), ""}
	tools := make([]string, 0, len(t.External))
	for tool := range t.External {
		tools = append(tools, tool)
	}
	sort.Strings(tools)
	for _, tool := range tools {
		lines = append(lines, fmt.Sprintf("- **%s:** %s", tool, t.External[tool]))
	}
	if t.Origin.Workspace != "" {
		lines = append(lines, fmt.Sprintf("- **Planned in:** %s (`%s`)", t.Origin.Workspace, t.Origin.Path))
	}
	if t.Spec != "" {
		lines = append(lines, fmt.Sprintf("- **Spec:** `%s`", t.Spec))
	}
	if len(t.ADRs) > 0 {
		lines = append(lines, "- **ADRs:** "+strings.Join(t.ADRs, ", "))
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

func runGate(root string, args []string) int {
	pos, reason, force, err := parseTransition(args)
	if err != nil || len(pos) != 2 {
		fmt.Fprintln(os.Stderr, "usage: tomo gate <phase> <status> [--reason <r>] [--force]")
		return 1
	}
	phase, status := pos[0], pos[1]
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	st, err := state.LoadState(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var notes []string
	if st.Name != "" {
		probe := st
		if err := state.SetGate(&probe, phase, status, reason); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if violation := state.CheckGate(st, phase, status); violation != nil {
			if err := gatePolicy(loaded.Effective.Gates, violation, force); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			notes = append(notes, "override: "+violation.Error())
		}
	}
	st, err = state.AdvanceGate(root, loaded.Effective.ArtifactDir, phase, status, reason, notes...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("%s: %s = %s\n", st.Name, st.Phase, st.Status)
	return 0
}

func runReopen(root string, args []string) int {
	pos, reason, _, err := parseTransition(args)
	if err != nil || len(pos) != 1 {
		fmt.Fprintln(os.Stderr, "usage: tomo reopen <phase> --reason <r>")
		return 1
	}
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	st, err := state.Reopen(root, loaded.Effective.ArtifactDir, pos[0], reason)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("%s: %s = %s\n", st.Name, st.Phase, st.Status)
	return 0
}

// runRepo handles `tomo repo`, keeping a workspace to its one target repo.
func runRepo(root string, args []string) int {
	if len(args) > 0 && args[0] == "add" {
		if err := icm.RefuseSecondTarget(root); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return repos.Run(root, "tomo", args, os.Stdout, os.Stderr)
}

// runEnv prints harness flags for the target repo.
func runEnv(root string) int {
	name, _, err := icm.Target(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return repos.RunEnv(root, "tomo", []string{name}, os.Stdout, os.Stderr)
}

func runIssues(root string, args []string) int {
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir := loaded.Effective.IssuesDir
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "list":
		all, err := issues.List(root, dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		for _, it := range all {
			fmt.Printf("%s [%s] %s\n", it.ID, it.Status, it.Title)
		}
		return 0
	case "show":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: tomo issues show <id>")
			return 1
		}
		it, err := issues.Read(root, dir, args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("%s [%s] %s\n%s\n", it.ID, it.Status, it.Title, it.Body)
		return 0
	case "new":
		var title string
		var labels []string
		var body string
		for i := 1; i < len(args); i++ {
			switch a := args[i]; {
			case a == "--title" && i+1 < len(args):
				i++
				title = args[i]
			case a == "--labels" && i+1 < len(args):
				i++
				labels = strings.Split(args[i], ",")
			case a == "--body" && i+1 < len(args):
				i++
				body = args[i]
			}
		}
		if title == "" {
			fmt.Fprintln(os.Stderr, "usage: tomo issues new --title <t> [--labels a,b] [--body <b>]")
			return 1
		}
		target, err := icm.TargetName(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		it, err := issues.New(root, dir, issues.NewOptions{Title: title, Repo: target, Labels: labels, Body: body})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("created %s %s\n", it.ID, it.Title)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "tomo issues: unknown verb %q\n", args[0])
		return 1
	}
}

// kumiPrereqs are the Go tools a kumi project needs, checked (and installed
// with --install) by `tomo doctor`.
var kumiPrereqs = map[string]string{
	"templ": "github.com/a-h/templ/cmd/templ",
	"air":   "github.com/air-verse/air",
	"goose": "github.com/pressly/goose/v3/cmd/goose",
}

func runDoctor(root string, args []string) int {
	install := false
	for _, a := range args {
		switch a {
		case "--install":
			install = true
		case "", "--help", "-h":
		default:
			fmt.Fprintf(os.Stderr, "tomo doctor: unknown flag %q\n", a)
			return 1
		}
	}
	checkSystem("go", "version")
	checkSystem("git", "--version")

	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	tools := map[string]string{}
	for name, path := range kumiPrereqs {
		tools[name] = path
	}
	for _, appType := range []string{"kumi", "hanga"} {
		if p, ok := loaded.Effective.AppSources[appType]; ok && p != "" {
			tools[appType] = p
		}
	}
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		checkGoTool(name, tools[name], install)
	}
	return 0
}

func checkSystem(name, arg string) {
	if err := exec.Command(name, arg).Run(); err != nil {
		fmt.Printf("✗ %-8s not found\n", name)
	} else {
		fmt.Printf("✓ %s\n", name)
	}
}

func checkGoTool(name, importPath string, install bool) {
	bin, err := icm.GoToolBin(importPath)
	if err != nil {
		fmt.Printf("✗ %-8s %v\n", name, err)
		return
	}
	if _, err := os.Stat(bin); err == nil {
		fmt.Printf("✓ %s\n", name)
		return
	}
	if install {
		fmt.Printf("… installing %s\n", name)
		if _, err := icm.GoInstall(importPath); err != nil {
			fmt.Printf("✗ %-8s %v\n", name, err)
			return
		}
		fmt.Printf("✓ %s\n", name)
	} else {
		fmt.Printf("✗ %-8s (run `tomo doctor --install`)\n", name)
	}
}

func runList(root string) int {
	fmt.Printf("tomo %s\n", version.String())
	loaded, err := settings.LoadSettings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, appType := range []string{"kumi", "hanga"} {
		path, ok := loaded.Effective.AppSources[appType]
		if !ok || path == "" {
			fmt.Printf("%-8s unknown\n", appType)
			continue
		}
		bin, err := icm.GoToolBin(path)
		if err != nil {
			fmt.Printf("%-8s error: %v\n", appType, err)
			continue
		}
		if _, err := os.Stat(bin); err != nil {
			fmt.Printf("%-8s not installed\n", appType)
			continue
		}
		fmt.Printf("%-8s %s\n", appType, goToolVersion(bin))
	}
	return 0
}

// goToolVersion reports a Go tool's module version via `go version -m`, or
// "unknown".
func goToolVersion(bin string) string {
	out, err := exec.Command("go", "version", "-m", bin).Output()
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "mod" {
			return fields[2]
		}
	}
	return "unknown"
}

// runCommit commits only what's staged in the target repo, so each commit is a
// reviewed micro-commit rather than everything in the tree. Extra arguments go to
// `git commit`.
func runCommit(root string, args []string) int {
	_, target, err := icm.Target(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	staged := exec.Command("git", "diff", "--cached", "--quiet")
	staged.Dir = target
	if err := staged.Run(); err == nil {
		fmt.Fprintln(os.Stderr, "tomo commit: nothing staged — stage one logical change with `git add`, then commit it")
		return 1
	}
	if err := run(target, "git", append([]string{"commit"}, args...)...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runPR(root string, args []string) int {
	_, target, err := icm.Target(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ghArgs := prArgs(root, target, args)
	if err := run(target, "gh", ghArgs...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// prArgs builds the `gh pr create` arguments. When the active change came from a
// ticket, the pull request links it, and a branch without the ticket's id gets a
// warning.
func prArgs(root, target string, args []string) []string {
	ghArgs := []string{"pr", "create"}
	title := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		title = args[0]
	}
	st, err := state.LoadState(root)
	if err == nil {
		if t := changeTicket(root, st); t != nil {
			if title == "" {
				title = "[" + t.ID + "] " + t.Title
			}
			ghArgs = append(ghArgs, "--body", prBody(t))
			branch := exec.Command("git", "branch", "--show-current")
			branch.Dir = target
			if out, err := branch.Output(); err == nil && !strings.Contains(strings.ToLower(string(out)), strings.ToLower(t.ID)) {
				fmt.Fprintf(os.Stderr, "warning: branch %s doesn't include %s — `tomo status` shows the suggested name\n", strings.TrimSpace(string(out)), t.ID)
			}
		}
	}
	if title != "" {
		ghArgs = append(ghArgs, "--title", title)
	}
	return ghArgs
}

func run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
