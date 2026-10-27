// Package cli implements the tomo workspace helpers: status, settings, switch,
// issues, commit, pr, help.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

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
	case "issues":
		return runIssues(root, args[1:])
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
