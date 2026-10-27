// Command tomo is the ICM tool: scaffold a folder-driven SDLC workspace
// (`init`), refresh its managed files (`update`), add apps (`add`), and a
// small set of workspace helpers.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/btdstudio/tomo/internal/cli"
	"github.com/btdstudio/tomo/internal/icm"
	"github.com/btdstudio/tomo/internal/version"
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
	rep, err := icm.Scaffold(dir, icm.Options{NoGit: noGit, Force: force})
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
	rep, err := icm.Update(".")
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

func runAdd(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: tomo add kumi|hachigo <name> [module-path]")
		return 1
	}
	rep, err := icm.Add(".", args[0], args[1], args[2:]...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("added %s from %s at %s\n", args[1], rep.Source, rep.Path)
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
