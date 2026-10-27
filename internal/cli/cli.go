// Package cli implements the tomo workspace helpers: status, settings, switch,
// issues, commit, pr, help.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/btdstudio/tomo/internal/icm"
	"github.com/btdstudio/tomo/internal/issues"
	"github.com/btdstudio/tomo/internal/settings"
	"github.com/btdstudio/tomo/internal/state"
	"github.com/btdstudio/tomo/internal/version"
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
	case "issues":
		return runIssues(root, args[1:])
	case "doctor":
		return runDoctor(root, args[1:])
	case "list", "ls":
		return runList(root)
	case "commit":
		return runCommit(root)
	case "pr":
		return runPR(args[1:])
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
  tomo add kumi|hachigo <name> [module]   scaffold an app into src/apps/<name>/
  tomo status                             show the active change
  tomo settings                           show effective settings
  tomo switch <change>                    switch the active change
  tomo issues new|list|show [args]        manage 00_issues/
  tomo doctor [--install]                 check toolchain prerequisites
  tomo list                               list installed tools + versions
  tomo commit                             stage all + commit (conventional)
  tomo pr [title]                         open a pull request via gh
  tomo help                               this help
`)
}

func runStatus(root string) int {
	st, err := state.LoadState(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if st.Change == "" {
		fmt.Println("no active change")
		return 0
	}
	fmt.Printf("change:   %s\n", st.Change)
	fmt.Printf("phase:    %s\n", st.Phase)
	fmt.Printf("status:   %s\n", st.Status)
	fmt.Printf("gates:    requirements=%s design=%s plan=%s implement=%s\n",
		st.Gates.Requirements, st.Gates.Design, st.Gates.Plan, st.Gates.Implement)
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
	fmt.Printf("issues:      %s\n", e.Issues)
	fmt.Printf("issuesDir:   %s\n", e.IssuesDir)
	for k, v := range e.AppSources {
		fmt.Printf("appSource.%s: %s\n", k, v)
	}
	return 0
}

func runSwitch(root string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: tomo switch <change>")
		return 1
	}
	st, err := state.SwitchTo(root, args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("switched to %s\n", st.Change)
	return 0
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
		for _, it := range issues.ListIssues(root, dir) {
			fmt.Printf("#%d [%s] %s\n", it.ID, it.Status, it.Title)
		}
		return 0
	case "show":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: tomo issues show <id>")
			return 1
		}
		id, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid id: %s\n", args[1])
			return 1
		}
		it, err := issues.ReadIssue(root, dir, id)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("#%d [%s] %s\n%s\n", it.ID, it.Status, it.Title, it.Body)
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
		it, err := issues.WriteNewIssue(root, dir, issues.NewIssueOptions{Title: title, Labels: labels, Body: body})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("created #%d %s\n", it.ID, it.Title)
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
	for _, appType := range []string{"kumi", "hachigo"} {
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
	for _, appType := range []string{"kumi", "hachigo"} {
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

func runCommit(root string) int {
	if err := run(root, "git", "add", "-A"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := run(root, "git", "commit"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runPR(args []string) int {
	ghArgs := []string{"pr", "create"}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		ghArgs = append(ghArgs, "--title", args[0])
	}
	if err := run(".", "gh", ghArgs...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
