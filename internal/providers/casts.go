package providers

import (
	"bytes"
	"regexp"
	"strings"
	"text/template"
)

// The cast templates reproduce the exact bytes of the Node string builders in
// src/lib/providers.js. Data values are emitted verbatim, so persona/command
// bodies containing literal `{{` are safe.

type opencodeData struct {
	Description string
	ReadOnly    bool
	Body        string
}

var opencodeTmpl = template.Must(template.New("opencode").Parse(`---
description: {{.Description}}
mode: subagent{{if .ReadOnly}}
permission:
  edit: deny
  bash: deny{{end}}
---

{{.Body}}
`))

func CastOpenCode(p Persona) string {
	var buf bytes.Buffer
	_ = opencodeTmpl.Execute(&buf, opencodeData{
		Description: p.FM.Description,
		ReadOnly:    READONLY_PERSONAS[p.Name],
		Body:        p.Body,
	})
	return buf.String()
}

type claudeData struct {
	Name        string
	Description string
	Tools       string
	Body        string
}

var claudeTmpl = template.Must(template.New("claude").Parse(`---
name: {{.Name}}
description: {{.Description}}{{if .Tools}}
tools: {{.Tools}}{{end}}
---

{{.Body}}
`))

func CastClaude(p Persona) string {
	var buf bytes.Buffer
	_ = claudeTmpl.Execute(&buf, claudeData{
		Name:        p.Name,
		Description: p.FM.Description,
		Tools:       p.FM.Tools,
		Body:        p.Body,
	})
	return buf.String()
}

type copilotData struct {
	Name        string
	Description string
	ReadOnly    bool
	Body        string
}

var copilotTmpl = template.Must(template.New("copilot").Parse(`---
name: {{.Name}}
description: {{.Description}}{{if .ReadOnly}}
tools: ["read", "search"]{{end}}
---

{{.Body}}`))

func CastCopilot(p Persona) string {
	var buf bytes.Buffer
	_ = copilotTmpl.Execute(&buf, copilotData{
		Name:        p.Name,
		Description: p.FM.Description,
		ReadOnly:    READONLY_PERSONAS[p.Name],
		Body:        p.Body,
	})
	return buf.String()
}

type antigravityData struct {
	Name        string
	Description string
	ReadOnly    bool
	Body        string
}

var antigravityTmpl = template.Must(template.New("antigravity").Parse(`---
name: {{.Name}}
description: {{.Description}}
subagent: true
mainAgent: false{{if .ReadOnly}}
tools:
  - view_file
  - grep_search{{end}}
---

{{.Body}}
`))

func CastAntigravity(p Persona) string {
	var buf bytes.Buffer
	_ = antigravityTmpl.Execute(&buf, antigravityData{
		Name:        p.Name,
		Description: p.FM.Description,
		ReadOnly:    READONLY_PERSONAS[p.Name],
		Body:        p.Body,
	})
	return buf.String()
}

func codexEsc(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	return strings.ReplaceAll(v, `"`, `\"`)
}

func codexEscBody(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	return strings.ReplaceAll(v, `"""`, `\"""`)
}

var codexFuncs = template.FuncMap{
	"esc":     codexEsc,
	"escBody": codexEscBody,
}

type codexData struct {
	Name        string
	Description string
	Sandbox     string
	Body        string
}

var codexTmpl = template.Must(template.New("codex").Funcs(codexFuncs).Parse(`name = "{{esc .Name}}"
description = "{{esc .Description}}"
sandbox_mode = "{{.Sandbox}}"
developer_instructions = """
{{escBody .Body}}
"""
`))

func CastCodex(p Persona) string {
	sandbox := "workspace-write"
	if READONLY_PERSONAS[p.Name] {
		sandbox = "read-only"
	}
	var buf bytes.Buffer
	_ = codexTmpl.Execute(&buf, codexData{
		Name:        p.Name,
		Description: p.FM.Description,
		Sandbox:     sandbox,
		Body:        p.Body,
	})
	return buf.String()
}

type claudeCommandData struct {
	Description string
	Grant       string
	Body        string
}

var claudeCommandTmpl = template.Must(template.New("claude-command").Parse(`---
description: {{.Description}}
allowed-tools: {{.Grant}}
---

{{.Body}}
`))

func CastClaudeCommand(cmd Command) string {
	var buf bytes.Buffer
	_ = claudeCommandTmpl.Execute(&buf, claudeCommandData{
		Description: cmd.FM.Description,
		Grant:       ClaudeReadGrant(),
		Body:        cmd.Body,
	})
	return buf.String()
}

var commandTmpl = template.Must(template.New("command").Parse("---\nname: {{.Name}}\ndescription: {{.Description}}\n---\n\nThe user invoked this skill as `/{{.Name}}`; text typed after the command name is its arguments. Treat that text as `$ARGUMENTS` in the body below (absent text means no arguments were provided).\n\n{{.Body}}\n"))

type commandData struct {
	Name        string
	Description string
	Body        string
}

func CastAntigravityCommand(cmd Command) string {
	var buf bytes.Buffer
	_ = commandTmpl.Execute(&buf, commandData{
		Name:        "tomo-" + cmd.Name,
		Description: cmd.FM.Description,
		Body:        cmd.Body,
	})
	return buf.String()
}

var copilotSkillNameRe = regexp.MustCompile(`^tomo-[a-z]+(?:-[a-z]+)*$`)

func CastCopilotCommand(cmd Command) string {
	name := "tomo-" + cmd.Name
	if !copilotSkillNameRe.MatchString(name) {
		panic("tomo: Copilot skill name \"" + name + "\" must be lowercase and hyphenated")
	}
	var buf bytes.Buffer
	_ = commandTmpl.Execute(&buf, commandData{
		Name:        name,
		Description: cmd.FM.Description,
		Body:        cmd.Body,
	})
	return buf.String()
}
