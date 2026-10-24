package install

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	tomo "github.com/btdstudio/tomo"
	"github.com/btdstudio/tomo/internal/providers"
	"github.com/btdstudio/tomo/internal/render"
)

func isGitIgnored(rel string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", "--", rel)
	cmd.Dir = filepath.Join("..", "..")
	return cmd.Run() == nil
}

// Every provider project install target must be gitignored, otherwise a
// consumer project's first install leaves an unresolvable dirty tree.
func TestProviderTargetsGitIgnored(t *testing.T) {
	commands, err := render.ReadCommands()
	if err != nil {
		t.Fatal(err)
	}
	personas, err := render.ReadPersonas()
	if err != nil {
		t.Fatal(err)
	}
	sampleCommand := commands[0].Name
	samplePersona := personas[0].Name
	for _, prov := range providers.PROVIDERS {
		commandPath := filepath.Join(prov.ProjectCommandDir("."), prov.CommandFilename(sampleCommand))
		if !isGitIgnored(commandPath) {
			t.Fatalf("%s: command target not ignored: %s", prov.ID, commandPath)
		}
		agentPath := filepath.Join(prov.ProjectAgentDir("."), prov.Filename(samplePersona))
		if !isGitIgnored(agentPath) {
			t.Fatalf("%s: agent target not ignored: %s", prov.ID, agentPath)
		}
	}
}

var assetRe = regexp.MustCompile(`\.tomo\/(templates|personas)\/([A-Za-z0-9_./-]+?\.md)`)

// Canonical bodies only reference installed asset sources that exist in the
// embedded store.
func TestContentReferencesExistingAssets(t *testing.T) {
	content := render.ReadContentCommands()
	seen := 0
	for name, body := range content {
		for _, m := range assetRe.FindAllStringSubmatch(body, -1) {
			seen++
			kind, file := m[1], m[2]
			if _, err := fs.Stat(tomo.FS, "src/"+kind+"/"+file); err != nil {
				t.Fatalf("%s: references .tomo/%s/%s but src/%s/%s is missing", name, kind, file, kind, file)
			}
		}
	}
	if seen == 0 {
		t.Fatal("expected at least one asset reference to check")
	}
}
