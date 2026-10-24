// Package render renders the harness-facing command wrappers and persona casts
// from the canonical content store embedded in the binary (tomo.FS). It is a
// library for the install command and the tests — there is no separate build
// step: install folds rendering in and writes directly to harness dirs.
//
// This is a port of scripts/render.js, adapted to read the embedded store.
package render

import (
	"bytes"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"text/template"

	tomo "github.com/btdstudio/tomo"
	"github.com/btdstudio/tomo/internal/personas"
	"github.com/btdstudio/tomo/internal/providers"
)

// Embedded store paths (forward slashes, as io/fs requires).
const (
	commandsDir        = "src/commands"
	personasDir        = "src/personas"
	templatesDir       = "src/templates"
	contentCommandsDir = "content/commands"
	gateContentPath    = "content/gate.md"
)

var personaRequired = []string{"name", "description", "focus"}

var validGates = map[string]bool{"none": true, "block": true, "prompt": true, "auto": true}
var commandRequired = []string{"name", "description", "args", "gate"}

// GATE_INCLUDE is the directive a gated canonical body uses to pull in the
// shared gate.
const GATE_INCLUDE = "<!-- tomo:include gate.md -->"

var frontmatterRe = regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n?`)

// ParseFrontmatter splits a canonical file into its frontmatter map and body.
func ParseFrontmatter(src string) (fm map[string]string, body string, err error) {
	m := frontmatterRe.FindStringSubmatch(src)
	if m == nil {
		return nil, "", fmt.Errorf("command has no YAML frontmatter")
	}
	fm = map[string]string{}
	lines := strings.Split(m[1], "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		colon := strings.Index(line, ":")
		if colon == -1 {
			return nil, "", fmt.Errorf("frontmatter line is not \"key: value\": %s", line)
		}
		key := strings.TrimSpace(line[:colon])
		value := strings.TrimSpace(line[colon+1:])
		if value == ">" {
			var folded []string
			for i+1 < len(lines) && strings.TrimSpace(strings.TrimRight(lines[i+1], "\r")) != "" &&
				!strings.Contains(strings.TrimRight(lines[i+1], "\r"), ":") {
				folded = append(folded, strings.TrimSpace(strings.TrimRight(lines[i+1], "\r")))
				i++
			}
			value = strings.Join(folded, " ")
		} else if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}
		fm[key] = value
	}
	rest := src[len(m[0]):]
	rest = strings.TrimPrefix(rest, "\n")
	body = strings.TrimRight(rest, " \t\r\n")
	return fm, body, nil
}

// ReadCommands reads the command contracts (frontmatter only) from src/commands.
func ReadCommands() ([]providers.Command, error) {
	entries, err := fs.ReadDir(tomo.FS, commandsDir)
	if err != nil {
		return nil, fmt.Errorf("missing %s", commandsDir)
	}
	var commands []providers.Command
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		raw, err := fs.ReadFile(tomo.FS, commandsDir+"/"+e.Name())
		if err != nil {
			return nil, err
		}
		fm, body, err := ParseFrontmatter(string(raw))
		if err != nil {
			return nil, fmt.Errorf("%s: %v", e.Name(), err)
		}
		for _, key := range commandRequired {
			if _, ok := fm[key]; !ok {
				return nil, fmt.Errorf("%s: missing frontmatter field %q", e.Name(), key)
			}
		}
		for _, key := range []string{"name", "description", "gate"} {
			if fm[key] == "" {
				return nil, fmt.Errorf("%s: empty frontmatter field %q", e.Name(), key)
			}
		}
		if fm["name"] != name {
			return nil, fmt.Errorf("%s: frontmatter name %q != filename %q", e.Name(), fm["name"], name)
		}
		if !validGates[fm["gate"]] {
			return nil, fmt.Errorf("%s: invalid gate %q", e.Name(), fm["gate"])
		}
		if body != "" {
			return nil, fmt.Errorf("%s: command contracts are frontmatter only; put the body in content/commands/", e.Name())
		}
		commands = append(commands, providers.Command{
			Name: name,
			FM: providers.CommandFM{
				Name:        fm["name"],
				Description: fm["description"],
				Args:        fm["args"],
				Gate:        fm["gate"],
			},
			Body: body,
		})
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("no canonical commands found in src/commands")
	}
	return commands, nil
}

// ReadContentCommands reads the canonical command bodies keyed by name.
func ReadContentCommands() map[string]string {
	out := map[string]string{}
	entries, err := fs.ReadDir(tomo.FS, contentCommandsDir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		b, err := fs.ReadFile(tomo.FS, contentCommandsDir+"/"+e.Name())
		if err != nil {
			continue
		}
		out[name] = strings.TrimRight(string(b), " \t\r\n")
	}
	return out
}

// ContentCommandPath returns the embedded path of a canonical body (for error
// messages).
func ContentCommandPath(name string) string {
	return contentCommandsDir + "/" + name + ".md"
}

// GateContent returns the shared approval-gate contract.
func GateContent() (string, error) {
	b, err := fs.ReadFile(tomo.FS, gateContentPath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// InlineBody returns the fully inlined prompt for a command: its canonical body
// with the gate directive expanded, plus a trailing arguments line for bodies
// that never mention $ARGUMENTS.
func InlineBody(cmd providers.Command) (string, error) {
	content := ReadContentCommands()
	if _, ok := content[cmd.Name]; !ok {
		return "", fmt.Errorf("missing %s (canonical body)", ContentCommandPath(cmd.Name))
	}
	body := content[cmd.Name]
	if strings.Contains(body, GATE_INCLUDE) {
		gate, err := fs.ReadFile(tomo.FS, gateContentPath)
		if err != nil {
			return "", err
		}
		body = strings.ReplaceAll(body, GATE_INCLUDE, strings.TrimRight(string(gate), " \t\r\n"))
	}
	if !strings.Contains(body, "$ARGUMENTS") {
		body += "\n\nArguments (may be empty): $ARGUMENTS"
	}
	return body, nil
}

// InlineCommand returns the command with its body replaced by the inlined
// prompt.
func InlineCommand(cmd providers.Command) (providers.Command, error) {
	body, err := InlineBody(cmd)
	if err != nil {
		return providers.Command{}, err
	}
	cmd.Body = body
	return cmd, nil
}

var wrapperTmpl = template.Must(template.New("wrapper").Parse("---\ndescription: {{.Description}}\n---\n\n{{.Body}}\n"))

// Render produces the plain opencode wrapper for a command.
func Render(cmd providers.Command) (string, error) {
	body, err := InlineBody(cmd)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := wrapperTmpl.Execute(&buf, map[string]string{"Description": cmd.FM.Description, "Body": body}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var liveSyntax = []struct {
	re   *regexp.Regexp
	what string
}{
	{regexp.MustCompile("!`"), "shell injection (!`cmd`)"},
	{regexp.MustCompile(`\$[0-9]`), "positional argument ($N)"},
	{regexp.MustCompile(`(?m)(^|\s)@[\w./~-]`), "file reference (@path)"},
}

// LiveSyntaxProblems reports harness template syntax in text.
func LiveSyntaxProblems(label, text string) []string {
	var out []string
	for _, ls := range liveSyntax {
		if ls.re.MatchString(text) {
			out = append(out, fmt.Sprintf("%s: contains harness template syntax: %s", label, ls.what))
		}
	}
	return out
}

// validSubcommands is the set of top-level `tomo` subcommands the command
// bodies may reference in prose. The vocabulary check pins every backticked
// `tomo <verb>` mention to a real subcommand (the Go analog of the old
// `.tomo/lib/<m>.js` export check).
var validSubcommands = map[string]bool{
	"settings": true, "state": true, "slug": true, "log": true,
	"persona": true, "issue": true,
}

var subcommandRe = regexp.MustCompile("`tomo ([a-z][a-z-]*)`")

var personaRe = regexp.MustCompile("`tomo persona ([a-z-]+)`(?s:.*?)By default this is the \\*\\*([a-z]+)\\*\\*")

// CheckVocabulary compares command-body prose against the code-owned lists it
// restates, returning one problem per drift.
func CheckVocabulary(commands []providers.Command, content map[string]string) []string {
	var problems []string

	if help, ok := content["help"]; ok {
		for _, cmd := range commands {
			if !regexp.MustCompile(`/tomo[ -]` + regexp.QuoteMeta(cmd.Name) + `\b`).MatchString(help) {
				problems = append(problems, fmt.Sprintf("help: command index missing \"/tomo %s\"", cmd.Name))
			}
		}
	}

	keys := settingsKeys()
	for _, name := range []string{"init", "settings"} {
		body := content[name]
		if body == "" {
			continue
		}
		for _, key := range keys {
			if !strings.Contains(body, "`"+key+"`") {
				problems = append(problems, fmt.Sprintf("%s: settings enumeration missing %q (defaultSettings has %d keys)", name, key, len(keys)))
			}
		}
	}

	for name, body := range content {
		for _, m := range subcommandRe.FindAllStringSubmatch(body, -1) {
			if !validSubcommands[m[1]] {
				problems = append(problems, fmt.Sprintf("%s: unknown tomo subcommand %q", name, m[1]))
			}
		}
	}

	defaultMap := personas.DEFAULT_PERSONA_MAP
	for name, body := range content {
		for _, m := range personaRe.FindAllStringSubmatch(body, -1) {
			step, persona := m[1], m[2]
			if expected, ok := defaultMap[step]; ok && expected != persona {
				problems = append(problems, fmt.Sprintf("%s: %q default persona is %q but DEFAULT_PERSONA_MAP says %q", name, step, persona, expected))
			}
		}
	}

	return problems
}

// settingsKeys returns the keys of defaultSettings in declaration order.
func settingsKeys() []string {
	return []string{"baseSource", "gates", "provider", "artifactDir", "issues", "issuesDir", "personas"}
}

// CheckContent validates the content store against the command registry.
func CheckContent() ([]string, error) {
	var problems []string
	if _, err := fs.Stat(tomo.FS, gateContentPath); err != nil {
		problems = append(problems, fmt.Sprintf("missing: %s (the shared approval gate)", gateContentPath))
	} else if gate, err := fs.ReadFile(tomo.FS, gateContentPath); err == nil {
		problems = append(problems, LiveSyntaxProblems(gateContentPath, string(gate))...)
	}

	commands, err := ReadCommands()
	if err != nil {
		return problems, err
	}
	content := ReadContentCommands()
	names := map[string]bool{}
	for _, c := range commands {
		names[c.Name] = true
	}
	for _, cmd := range commands {
		body, ok := content[cmd.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("missing: %s (canonical body)", ContentCommandPath(cmd.Name)))
			continue
		}
		includes := strings.Count(body, GATE_INCLUDE)
		if cmd.FM.Gate != "none" && includes != 1 {
			problems = append(problems, fmt.Sprintf("%s: gated command body must include the gate exactly once (%s)", cmd.Name, GATE_INCLUDE))
		}
		if cmd.FM.Gate == "none" && includes > 0 {
			problems = append(problems, fmt.Sprintf("%s: ungated command body must not include the gate", cmd.Name))
		}
		if regexp.MustCompile(`\bgate\.md\b`).MatchString(strings.ReplaceAll(body, GATE_INCLUDE, "")) {
			problems = append(problems, fmt.Sprintf("%s: body must not point at gate.md for a runtime read; use %s", cmd.Name, GATE_INCLUDE))
		}
		problems = append(problems, LiveSyntaxProblems(ContentCommandPath(cmd.Name), body)...)
	}
	for name := range content {
		if !names[name] {
			problems = append(problems, fmt.Sprintf("orphan: %s (no matching command)", ContentCommandPath(name)))
		}
	}

	vocabContent := map[string]string{}
	for k, v := range content {
		vocabContent[k] = v
	}
	if gate, err := fs.ReadFile(tomo.FS, gateContentPath); err == nil {
		vocabContent["gate"] = string(gate)
	}
	problems = append(problems, CheckVocabulary(commands, vocabContent)...)

	return problems, nil
}

// ReadPersonas reads the canonical persona profiles.
func ReadPersonas() ([]providers.Persona, error) {
	entries, err := fs.ReadDir(tomo.FS, personasDir)
	if err != nil {
		return nil, err
	}
	var out []providers.Persona
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		raw, err := fs.ReadFile(tomo.FS, personasDir+"/"+e.Name())
		if err != nil {
			return nil, err
		}
		fm, body, err := ParseFrontmatter(string(raw))
		if err != nil {
			return nil, fmt.Errorf("persona %s: %v", e.Name(), err)
		}
		for _, key := range personaRequired {
			if _, ok := fm[key]; !ok {
				return nil, fmt.Errorf("persona %s: missing frontmatter field %q", e.Name(), key)
			}
		}
		if fm["name"] != name {
			return nil, fmt.Errorf("persona %s: frontmatter name %q != filename %q", e.Name(), fm["name"], name)
		}
		if fm["description"] == "" || fm["focus"] == "" {
			return nil, fmt.Errorf("persona %s: empty frontmatter field", e.Name())
		}
		if body == "" {
			return nil, fmt.Errorf("persona %s: empty body", e.Name())
		}
		out = append(out, providers.Persona{
			Name: name,
			FM: providers.PersonaFM{
				Name:        fm["name"],
				Description: fm["description"],
				Tools:       fm["tools"],
				Focus:       fm["focus"],
			},
			Body: body,
		})
	}
	return out, nil
}

// PersonaCast is one persona rendered for one provider.
type PersonaCast struct {
	Name    string
	Cast    string
	Content string
}

func GeneratePersonas() ([]PersonaCast, error) {
	ps, err := ReadPersonas()
	if err != nil {
		return nil, err
	}
	var out []PersonaCast
	for _, p := range ps {
		for _, id := range providers.ProviderIds() {
			out = append(out, PersonaCast{Name: p.Name, Cast: id, Content: providers.PROVIDERS[id].Cast(p)})
		}
	}
	return out, nil
}

// CommonName returns the cache filename for the plain opencode wrapper.
func CommonName(name string) string {
	return "tomo-" + name + ".md"
}

// PersonaCastName returns the cache filename for a persona cast.
func PersonaCastName(name, cast string) string {
	return "persona-" + name + "." + cast + ".md"
}

// ProviderCommandName returns the cache filename for a provider's command
// wrapper.
func ProviderCommandName(name, id string) string {
	if providers.PROVIDERS[id].CommandCast != nil {
		return "tomo-" + name + "." + id + ".md"
	}
	return CommonName(name)
}

func CustomCommandProviders() []string {
	var ids []string
	for _, id := range providers.ProviderIds() {
		if providers.PROVIDERS[id].CommandCast != nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// GenerateAll renders every harness-facing file in memory, keyed by its cache
// filename. This replaces the checkout-resident adapters/common/ cache.
func GenerateAll() (map[string]string, error) {
	out := map[string]string{}
	commands, err := ReadCommands()
	if err != nil {
		return nil, err
	}
	for _, cmd := range commands {
		content, err := Render(cmd)
		if err != nil {
			return nil, err
		}
		out[CommonName(cmd.Name)] = content

		inline, err := InlineCommand(cmd)
		if err != nil {
			return nil, err
		}
		for _, id := range CustomCommandProviders() {
			out[ProviderCommandName(cmd.Name, id)] = providers.PROVIDERS[id].CommandCast(inline)
		}
	}
	personas, err := GeneratePersonas()
	if err != nil {
		return nil, err
	}
	for _, pc := range personas {
		out[PersonaCastName(pc.Name, pc.Cast)] = pc.Content
	}
	return out, nil
}

// TemplatesDir lists the artifact skeleton files under src/templates.
func TemplatesDir() ([]fs.DirEntry, error) {
	return fs.ReadDir(tomo.FS, templatesDir)
}

// ReadTemplate returns the verbatim content of an artifact skeleton.
func ReadTemplate(name string) ([]byte, error) {
	return fs.ReadFile(tomo.FS, templatesDir+"/"+name)
}

// ReadPersonaSource returns the verbatim content of a canonical persona profile
// (the neutral .tomo/personas/<name>.md copy).
func ReadPersonaSource(name string) ([]byte, error) {
	return fs.ReadFile(tomo.FS, personasDir+"/"+name+".md")
}
