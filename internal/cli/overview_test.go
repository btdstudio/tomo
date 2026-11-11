package cli

import (
	"strings"
	"testing"

	"github.com/btdstudio/kiso/ticket"
)

// TestPRBodyOverviewLine checks the Overview line sits right after Planned in,
// and that a ticket without a link gets the same body otherwise.
func TestPRBodyOverviewLine(t *testing.T) {
	const path = "specs/3-release/tickets/S-0014.md"
	body := func(workspace string) string {
		tk, err := ticket.Parse([]byte(plannedTicket))
		if err != nil {
			t.Fatal(err)
		}
		if err := tk.SetOrigin(ticket.Origin{Workspace: workspace, Path: path}); err != nil {
			t.Fatal(err)
		}
		return prBody(tk)
	}
	const overview = "- **Overview:** https://github.com/org/plan/blob/HEAD/specs/3-release/README.md\n"
	github := body("https://github.com/org/plan.git")
	want := "- **Planned in:** https://github.com/org/plan.git (`" + path + "`)\n" + overview + "- **Spec:** "
	if !strings.Contains(github, want) {
		t.Fatalf("body doesn't have %q:\n%s", want, github)
	}
	local := body("tickets")
	if strings.Contains(local, "Overview") {
		t.Fatalf("a folder-name origin got an Overview line:\n%s", local)
	}
	if got := strings.Replace(strings.Replace(github, overview, "", 1), "https://github.com/org/plan.git", "tickets", 1); got != local {
		t.Fatalf("bodies differ beyond the Overview line:\n%s\n---\n%s", got, local)
	}
}

func TestWebURL(t *testing.T) {
	const gh = "https://github.com/org/plan"
	const az = "https://dev.azure.com/org/proj/_git/plan"
	for in, want := range map[string]webRepo{
		"https://github.com/org/plan.git":               {base: gh},
		"https://github.com/org/plan":                   {base: gh},
		"https://github.com/org/plan/":                  {base: gh},
		"http://github.com/org/plan.git":                {base: gh},
		"https://GitHub.com/org/plan.git":               {base: gh},
		"git@github.com:org/plan.git":                   {base: gh},
		"github.com:org/plan":                           {base: gh},
		"ssh://git@github.com/org/plan.git":             {base: gh},
		"ssh://git@github.com:22/org/plan.git":          {base: gh},
		"git@ssh.dev.azure.com:v3/org/proj/plan":        {base: az, azure: true},
		"ssh://git@ssh.dev.azure.com/v3/org/proj/plan":  {base: az, azure: true},
		"https://dev.azure.com/org/proj/_git/plan":      {base: az, azure: true},
		"https://dev.azure.com/my%20org/proj/_git/plan": {base: "https://dev.azure.com/my%20org/proj/_git/plan", azure: true},
	} {
		got, ok := webURL(in)
		if !ok || got != want {
			t.Errorf("webURL(%q) = %+v, %v, want %+v", in, got, ok, want)
		}
	}
	for _, in := range []string{
		"",
		"tickets",
		"./a:b",
		"/abs/path:x",
		"https://gitlab.com/org/plan.git",
		"git@gitlab.com:org/plan.git",
		"https://github.com/org",
		"https://github.com/org/plan/extra",
		"https://github.com//plan",
		"https://github.com/org/..",
		"git@github.com:./plan.git",
		"file:///github.com/org/plan.git",
		"https://dev.azure.com/org/proj/plan",
		"https://dev.azure.com/org/proj/x/plan",
		"./git@github.com:org/plan.git",
		"file://github.com/org/plan.git",
		"https://github.com/%zz/plan",
		"git@ssh.dev.azure.com:v2/org/proj/plan",
		"git@ssh.dev.azure.com:v3/org/proj",
	} {
		if got, ok := webURL(in); ok {
			t.Errorf("webURL(%q) = %+v, want none", in, got)
		}
	}
}

func TestOverviewPath(t *testing.T) {
	for in, want := range map[string]string{
		"specs/2-x/tickets/S-0005.md":       "specs/2-x/README.md",
		"plans/specs/2-x/tickets/S-0005.md": "plans/specs/2-x/README.md",
		"tickets/S-0005.md":                 "README.md",
		"specs/./2-x/tickets/S-0005.md":     "specs/2-x/README.md",
	} {
		if got, ok := overviewPath(in); !ok || got != want {
			t.Errorf("overviewPath(%q) = %q, %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{
		"",
		"S-0005.md",
		"specs/2-x/S-0005.md",
		"specs/tickets/2-x/S-0005.md",
		"../x/tickets/S-0005.md",
		"specs/../tickets/S-0005.md",
		"/abs/tickets/S-0005.md",
	} {
		if got, ok := overviewPath(in); ok {
			t.Errorf("overviewPath(%q) = %q, want none", in, got)
		}
	}
}

func TestOverviewLink(t *testing.T) {
	const path = "specs/2-x/tickets/S-0005.md"
	const ghLink = "https://github.com/org/plan/blob/HEAD/specs/2-x/README.md"
	const azLink = "https://dev.azure.com/org/proj/_git/plan?path=/specs/2-x/README.md"
	for _, c := range []struct {
		name, workspace, path, want string
	}{
		{"AC1 https", "https://github.com/org/plan.git", path, ghLink},
		{"AC1 scp", "git@github.com:org/plan.git", path, ghLink},
		{"AC1 ssh", "ssh://git@github.com/org/plan.git", path, ghLink},
		{"AC2 azure ssh", "git@ssh.dev.azure.com:v3/org/proj/plan", path, azLink},
		{"AC3 azure https", "https://dev.azure.com/org/proj/_git/plan", path, azLink},
		{"AC4 nested", "https://github.com/org/plan.git", "plans/specs/2-x/tickets/S-0005.md", "https://github.com/org/plan/blob/HEAD/plans/specs/2-x/README.md"},
		{"AC5 unknown host", "https://gitlab.com/org/plan.git", path, ""},
		{"AC5 folder name", "tickets", "S-0005.md", ""},
		{"AC5 folder name, nested path", "tickets", path, ""},
		{"AC6 not under tickets", "https://github.com/org/plan.git", "S-0005.md", ""},
		{"AC7 github space", "https://github.com/org/plan.git", "my plan/2-x/tickets/S-0005.md", "https://github.com/org/plan/blob/HEAD/my%20plan/2-x/README.md"},
		{"AC7 azure space", "git@ssh.dev.azure.com:v3/org/proj/plan", "my plan/2-x/tickets/S-0005.md", "https://dev.azure.com/org/proj/_git/plan?path=/my%20plan/2-x/README.md"},
		{"azure ampersand", "git@ssh.dev.azure.com:v3/org/proj/plan", "a&b+c/tickets/S-0005.md", "https://dev.azure.com/org/proj/_git/plan?path=/a%26b%2Bc/README.md"},
		{"github hash", "https://github.com/org/plan.git", "a#b/tickets/S-0005.md", "https://github.com/org/plan/blob/HEAD/a%23b/README.md"},
	} {
		if got := overviewLink(ticket.Origin{Workspace: c.workspace, Path: c.path}); got != c.want {
			t.Errorf("%s: overviewLink = %q, want %q", c.name, got, c.want)
		}
	}
}
