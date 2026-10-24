// Command tomo is the single binary that replaces the Node.js scripts and
// .tomo/lib/*.js runtime helpers. It dispatches to install, render, check,
// session-hook, and the runtime-helper subcommands the command bodies invoke.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/btdstudio/tomo/internal/cli"
	"github.com/btdstudio/tomo/internal/hook"
	"github.com/btdstudio/tomo/internal/install"
	"github.com/btdstudio/tomo/internal/providers"
	"github.com/btdstudio/tomo/internal/render"
	"github.com/btdstudio/tomo/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, cli.Usage)
		return 1
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("tomo version v%s\n", version.Version)
		return 0
	case "install":
		return runInstall(args[1:])
	case "render":
		return runRender(args[1:])
	case "check":
		problems, err := render.CheckContent()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, p)
		}
		if len(problems) > 0 {
			return 1
		}
		fmt.Println("content store is in sync")
		return 0
	case "session-hook":
		if msg := hook.SessionMessage(mustCwd()); msg != "" {
			fmt.Println(msg)
		}
		return 0
	default:
		return cli.Run(args)
	}
}

func mustCwd() string {
	wd, _ := os.Getwd()
	return wd
}

// runRender writes the in-memory rendered output to a directory (--out), or
// lists the filenames when no --out is given.
func runRender(args []string) int {
	outDir := ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--out":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "tomo render: --out needs a value")
				return 1
			}
			i++
			outDir = args[i]
		case a == "--help" || a == "-h":
			fmt.Fprintln(os.Stderr, "tomo render [--out <dir>]")
			return 0
		default:
			fmt.Fprintf(os.Stderr, "unknown argument: %s\n", a)
			return 1
		}
	}
	all, err := render.GenerateAll()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	sort.Strings(names)
	if outDir == "" {
		for _, name := range names {
			fmt.Println(name)
		}
		return 0
	}
	for _, name := range names {
		p := filepath.Join(outDir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := os.WriteFile(p, []byte(all[name]), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(filepath.ToSlash(p))
	}
	return 0
}

func runInstall(args []string) int {
	var globalFlag bool
	var providerSpec string
	project := mustCwd()
	providerSet := false

	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--global":
			globalFlag = true
		case a == "--project":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "tomo install: --project needs a value")
				return 1
			}
			i++
			project, _ = filepath.Abs(args[i])
		case strings.HasPrefix(a, "--provider="):
			providerSpec = strings.TrimPrefix(a, "--provider=")
			providerSet = true
		case a == "--provider":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "tomo install: --provider needs a value")
				return 1
			}
			i++
			providerSpec = args[i]
			providerSet = true
		case a == "--help" || a == "-h":
			printInstallHelp()
			return 0
		default:
			fmt.Fprintf(os.Stderr, "unknown argument: %s\n", a)
			return 1
		}
	}

	var ids []string
	var err error
	if providerSet {
		ids, err = providers.ParseProviderSpec(providerSpec)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	} else if isTTY() {
		ids, err = promptProviders()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	} else {
		ids = providers.ProviderIds()
	}

	var notices []install.Notice
	written, err := install.Install(install.Options{
		Project:   project,
		Global:    globalFlag,
		Providers: ids,
		Notices:   &notices,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	base := project
	label := "installed"
	if globalFlag {
		label = "linked"
		base, _ = os.UserHomeDir()
	}
	for _, p := range written {
		rel, _ := filepath.Rel(base, p)
		if rel == "" {
			rel = filepath.Base(p)
		}
		fmt.Println(label, filepath.ToSlash(rel))
	}
	harness := fmt.Sprintf("%d harnesses", len(ids))
	if len(ids) == 1 {
		harness = ids[0]
	}
	if globalFlag {
		fmt.Printf("tomo wrappers copied into your user dirs for %s — every project now has /tomo\n", harness)
	} else {
		fmt.Printf("tomo installed into %s (%s); .tomo/ seeded. Re-run to update.\n", project, harness)
	}
	for _, n := range notices {
		fmt.Printf("\nopencode: could not add the tomo read grant to %s (%s).\nAdd this to your opencode config so reads of %s don't prompt:\n%s\n", n.Path, n.Reason, n.Pattern, n.Snippet)
	}
	return 0
}

func printInstallHelp() {
	fmt.Printf(`tomo install

  PROJECT install  copies the /tomo commands into the current project and seeds
                   .tomo/ with templates, personas, and the tomo binary:
      tomo install
      tomo install --project <dir>
      tomo install --provider opencode[,claude]   (or --provider all)

  GLOBAL install   copies the commands into your user-level harness dirs so
                   every project has /tomo:
      tomo install --global [--provider ...]

  PROVIDER         which harness's command wrappers and persona subagents to
                   materialize. Prompted on an interactive terminal; defaults to
                   all (%s).
`, strings.Join(providers.ProviderIds(), " + "))
}

func isTTY() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func promptProviders() ([]string, error) {
	fmt.Printf("Which AI coding harness(es) do you use? (all | %s) [all]: ", strings.Join(providers.ProviderIds(), " | "))
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil && answer == "" {
		return providers.ProviderIds(), nil
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		answer = "all"
	}
	return providers.ParseProviderSpec(answer)
}
