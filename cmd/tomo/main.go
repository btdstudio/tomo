// Command tomo is the ICM tool: scaffold a folder-driven SDLC workspace
// (`init`), refresh its managed files (`update`), add apps (`add`), and a
// small set of workspace helpers.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/btdstudio/kiso/version"
	"github.com/btdstudio/tomo/internal/cli"
	"github.com/btdstudio/tomo/internal/icm"
	"github.com/btdstudio/tomo/internal/issues"
	"github.com/btdstudio/tomo/internal/settings"
	"github.com/btdstudio/tomo/internal/state"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		cli.PrintUsage()
		return 1
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("tomo version %s\n", version.String())
		return 0
	case "init":
		return runInit(args[1:])
	case "update":
		return runUpdate()
	case "add":
		return runAdd(args[1:])
	case "check":
		return runCheck()
	default:
		return cli.Run(args)
	}
}

func runInit(args []string) int {
	dir := "."
	noGit := false
	force := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--no-git":
			noGit = true
		case a == "--force":
			force = true
		case a == "--help" || a == "-h":
			fmt.Println("usage: tomo init [dir] [--no-git] [--force]")
			return 0
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "tomo init: unknown flag %s\n", a)
			return 1
		default:
			dir = a
		}
	}
	vcs, err := settings.InitVCS(dir, noGit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	rep, err := icm.Scaffold(dir, vcs, icm.Options{NoGit: noGit, Force: force})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, rel := range rep.Written {
		fmt.Println("wrote", rel)
	}
	if rep.GitInit {
		fmt.Println("git init")
	}
	fmt.Printf("scaffolded ICM workspace in %s\n", dir)
	return 0
}

func runUpdate() int {
	migrated, err := state.MigrateOldState(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if migrated {
		fmt.Println("moved .tomo/state.json and .tomo/log.md to STATE.json and LOG.md — review and commit them")
	}
	if code := migrateIssues(); code != 0 {
		return code
	}
	if icm.OldAppsDir(".") {
		fmt.Println("note: code now lives outside the workspace. src/apps/ is left as it is; register it with `tomo repo add <name> <remote> --path <dir>`")
	}
	loaded, err := settings.LoadSettings(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	rep, err := icm.Update(".", loaded.VCS)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, rel := range rep.Updated {
		fmt.Println("updated", rel)
	}
	for _, rel := range rep.Skipped {
		fmt.Println("skipped (edited)", rel)
	}
	fmt.Printf("update complete (%d updated, %d skipped, %d unchanged)\n",
		len(rep.Updated), len(rep.Skipped), len(rep.Unchanged))
	return 0
}

// migrateIssues converts old <n>.md issues, which need a target repo to name.
func migrateIssues() int {
	loaded, err := settings.LoadSettings(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir := loaded.Effective.IssuesDir
	if !issues.HasOldIssues(".", dir) {
		return 0
	}
	target, err := icm.TargetName(".")
	if err != nil {
		fmt.Printf("note: %s has issues in the old format. Register a target repo, then run `tomo update` again to convert them\n", dir)
		return 0
	}
	moved, err := issues.Migrate(".", dir, target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	sources := map[string]string{}
	for from, to := range moved {
		sources[dir+"/"+from] = dir + "/" + to
	}
	if err := state.RewriteSources(".", sources); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("converted %d issues to tickets in %s — review and commit them\n", len(moved), dir)
	return 0
}

func runAdd(args []string) int {
	usage := "usage: tomo add kumi|hanga <name> --path <dir> --remote <url> [module-path]"
	var pos []string
	opts := icm.AddOptions{}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--path" && i+1 < len(args):
			i++
			opts.Path = args[i]
		case a == "--remote" && i+1 < len(args):
			i++
			opts.Remote = args[i]
		case strings.HasPrefix(a, "-"):
			fmt.Fprintln(os.Stderr, usage)
			return 1
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) < 2 || opts.Path == "" || opts.Remote == "" {
		fmt.Fprintln(os.Stderr, usage)
		return 1
	}
	opts.Args = pos[2:]
	rep, err := icm.Add(".", pos[0], pos[1], opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("added %s from %s at %s — commit REPOS.json to share it\n", pos[1], rep.Source, rep.Path)
	return 0
}

func runCheck() int {
	problems := icm.Check()
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p)
	}
	if len(problems) > 0 {
		return 1
	}
	fmt.Println("scaffold is clean")
	return 0
}
