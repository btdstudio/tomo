package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/btdstudio/tomo/internal/providers"
)

func vocabInput(t *testing.T) ([]providers.Command, map[string]string) {
	t.Helper()
	commands, err := ReadCommands()
	if err != nil {
		t.Fatal(err)
	}
	content := ReadContentCommands()
	gate, err := GateContent()
	if err != nil {
		t.Fatal(err)
	}
	content["gate"] = gate
	return commands, content
}

func TestCommandContractsParse(t *testing.T) {
	commands, err := ReadCommands()
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) < 2 {
		t.Fatal("expected at least help and status")
	}
	nameRe := regexp.MustCompile(`^[\w-]+$`)
	for _, cmd := range commands {
		if cmd.Name != cmd.FM.Name {
			t.Fatalf("%s: name != filename", cmd.Name)
		}
		if cmd.FM.Description == "" || cmd.FM.Gate == "" {
			t.Fatalf("%s: empty description/gate", cmd.Name)
		}
		if cmd.Body != "" {
			t.Fatalf("%s: contract must be frontmatter only", cmd.Name)
		}
		if !nameRe.MatchString(cmd.Name) {
			t.Fatalf("%s: unsupported characters", cmd.Name)
		}
	}
}

func TestCheckContentGreen(t *testing.T) {
	problems, err := CheckContent()
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("content store out of sync:\n%s", strings.Join(problems, "\n"))
	}
}

func TestVocabularyGreen(t *testing.T) {
	commands, content := vocabInput(t)
	if p := CheckVocabulary(commands, content); len(p) != 0 {
		t.Fatalf("vocabulary drift:\n%s", strings.Join(p, "\n"))
	}
}

func TestVocabularyFlagsDrift(t *testing.T) {
	// help index
	commands, content := vocabInput(t)
	content["help"] = strings.Replace(content["help"], "/tomo design", "/tomo designX", 1)
	if !containsProblem(CheckVocabulary(commands, content), `"/tomo design"`) {
		t.Fatal("help index drift not flagged")
	}

	// settings enumeration
	_, content = vocabInput(t)
	content["init"] = strings.Replace(content["init"], "`personas`", "`personasX`", 1)
	if !containsProblem(CheckVocabulary(commands, content), `"personas"`) {
		t.Fatal("settings enumeration drift not flagged")
	}

	// unknown subcommand
	_, content = vocabInput(t)
	content["arch"] = strings.Replace(content["arch"], "`tomo settings`", "`tomo settingz`", 1)
	if !containsProblem(CheckVocabulary(commands, content), `unknown tomo subcommand "settingz"`) {
		t.Fatal("unknown subcommand drift not flagged")
	}

	// persona default
	_, content = vocabInput(t)
	content["trace"] = strings.Replace(content["trace"], "**reviewer**", "**architect**", 1)
	if !containsProblem(CheckVocabulary(commands, content), `"trace" default persona`) {
		t.Fatal("persona default drift not flagged")
	}
}

func containsProblem(problems []string, sub string) bool {
	for _, p := range problems {
		if strings.Contains(p, sub) {
			return true
		}
	}
	return false
}

func TestWrapperInlinesBodyAndGate(t *testing.T) {
	content := ReadContentCommands()
	gate, _ := GateContent()
	gateStr := strings.TrimRight(gate, " \t\r\n")
	commands, _ := ReadCommands()
	for _, cmd := range commands {
		body, err := InlineBody(cmd)
		if err != nil {
			t.Fatal(err)
		}
		wrapper, err := Render(cmd)
		if err != nil {
			t.Fatal(err)
		}
		want := "---\ndescription: " + cmd.FM.Description + "\n---\n\n" + body + "\n"
		if wrapper != want {
			t.Fatalf("%s: wrapper mismatch", cmd.Name)
		}
		if strings.Contains(wrapper, GATE_INCLUDE) {
			t.Fatalf("%s: gate directive not expanded", cmd.Name)
		}
		gated := cmd.FM.Gate != "none"
		if strings.Contains(wrapper, gateStr) != gated {
			t.Fatalf("%s: gate inlined iff gated", cmd.Name)
		}
		for _, part := range strings.Split(content[cmd.Name], GATE_INCLUDE) {
			if !strings.Contains(wrapper, strings.TrimSpace(part)) {
				t.Fatalf("%s: wrapper must carry the canonical body verbatim", cmd.Name)
			}
		}
		if regexp.MustCompile(`content store|\.tomo/commands/`).MatchString(wrapper) {
			t.Fatalf("%s: wrapper must not send the model to read a body", cmd.Name)
		}
	}
}

func TestLiveSyntax(t *testing.T) {
	for _, bad := range []string{"run !`ls`", "use $1 here", "see @src/x.md", "@.tomo/state.json first"} {
		if len(LiveSyntaxProblems("x", bad)) == 0 {
			t.Fatalf("must reject: %s", bad)
		}
	}
	for _, ok := range []string{"feat(api)!: x", "mail a@b.com", "$ARGUMENTS", "`.tomo/lib/state.js`"} {
		if len(LiveSyntaxProblems("x", ok)) != 0 {
			t.Fatalf("must accept: %s", ok)
		}
	}
}

func TestArgumentsPassthrough(t *testing.T) {
	commands, _ := ReadCommands()
	for _, cmd := range commands {
		content, err := Render(cmd)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(content, "$ARGUMENTS") {
			t.Fatalf("%s: wrapper must keep $ARGUMENTS", cmd.Name)
		}
	}
}

func TestReadPersonasParse(t *testing.T) {
	ps, err := ReadPersonas()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range ps {
		names = append(names, p.Name)
	}
	want := []string{"analyst", "architect", "coder", "reviewer", "security"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("persona names = %v", names)
	}
	for _, p := range ps {
		if p.Name != p.FM.Name || p.FM.Description == "" || p.FM.Focus == "" || p.Body == "" {
			t.Fatalf("%s: invalid persona", p.Name)
		}
	}
}

func TestCastFormats(t *testing.T) {
	ps, err := ReadPersonas()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		readonly := providers.READONLY_PERSONAS[p.Name]

		oc := providers.CastOpenCode(p)
		if !strings.HasPrefix(oc, "---\ndescription: "+p.FM.Description+"\nmode: subagent\n") {
			t.Fatalf("%s: opencode cast prefix", p.Name)
		}
		if strings.Contains(oc, "edit: deny") != readonly {
			t.Fatalf("%s: opencode read-only edit:deny", p.Name)
		}

		cl := providers.CastClaude(p)
		if !strings.HasPrefix(cl, "---\nname: "+p.Name+"\ndescription: "+p.FM.Description+"\n") {
			t.Fatalf("%s: claude cast prefix", p.Name)
		}

		cp := providers.CastCopilot(p)
		if strings.Contains(cp, `tools: ["read", "search"]`) != readonly {
			t.Fatalf("%s: copilot read-only tools", p.Name)
		}

		ag := providers.CastAntigravity(p)
		if strings.Contains(ag, "tools:\n  - view_file\n  - grep_search\n") != readonly {
			t.Fatalf("%s: antigravity read-only tools", p.Name)
		}

		cx := providers.CastCodex(p)
		wantSandbox := "workspace-write"
		if readonly {
			wantSandbox = "read-only"
		}
		if !strings.Contains(cx, `sandbox_mode = "`+wantSandbox+`"`) {
			t.Fatalf("%s: codex sandbox", p.Name)
		}
	}

	commands, err := ReadCommands()
	if err != nil {
		t.Fatal(err)
	}
	for _, cmd := range commands {
		inline, err := InlineCommand(cmd)
		if err != nil {
			t.Fatal(err)
		}
		claude := providers.CastClaudeCommand(inline)
		if !strings.HasPrefix(claude, "---\ndescription: "+cmd.FM.Description+"\nallowed-tools: "+providers.ClaudeReadGrant()+"\n---\n\n") {
			t.Fatalf("%s: claude command cast", cmd.Name)
		}
		agy := providers.CastAntigravityCommand(inline)
		if !strings.HasPrefix(agy, "---\nname: tomo-"+cmd.Name+"\ndescription: "+cmd.FM.Description+"\n---\n\n") {
			t.Fatalf("%s: antigravity command cast", cmd.Name)
		}
		cp := providers.CastCopilotCommand(inline)
		if !strings.HasPrefix(cp, "---\nname: tomo-"+cmd.Name+"\ndescription: "+cmd.FM.Description+"\n---\n\n") {
			t.Fatalf("%s: copilot command cast", cmd.Name)
		}
	}
}
