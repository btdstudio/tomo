---
name: tomo-help
description: Show Tomo's phases, states, and commands
---

The user invoked this skill as `/tomo-help`; text typed after the command name is its arguments. Treat that text as `$ARGUMENTS` in the body below (absent text means no arguments were provided).

# Tomo help

Tomo is a harness-agnostic AI SDD (AI-SDLC) toolset. Run the SDD loop one phase
at a time; each gated phase stops for approval before the next can start. All
state lives in this project's `.tomo/` workspace — read it before acting.

## Phases

1. requirements — take a base source (local file, GitHub issue, URL, or the
   settings default), ask clarifying questions, write
   `.tomo/specs/<change>/requirements.md` (a new numbered `<id>-<slug>` folder
   per change), approval gate.
2. design — analyze the codebase and write `.tomo/specs/<change>/design.md`,
   approval gate.
3. plan — decompose the design into phases and tasks, write
   `.tomo/specs/<change>/tasks.json`, approval gate.

## Commands

- `/tomo help` — this overview
- `/tomo status` — current phase, state, next action
- `/tomo settings` — show merged settings (global + project)
- `/tomo init` — seed the `.tomo/` workspace and base settings
- `/tomo requirements [src]` — phase 1
- `/tomo design` — phase 2
- `/tomo plan` — phase 3
- `/tomo implement` — phase 4, run the approved plan to done
- `/tomo continue` — resume after an approval or rejection
- `/tomo handoff` — write `.tomo/SLIP.md` for compact/resume
- `/tomo pickup` — restore context from `.tomo/` artifacts
- `/tomo switch [change]` — switch the active spec folder, resume its phase
- `/tomo issues [new|list|show]` — manage issues (GitHub or local markdown)

## Workflow commands

- `/tomo commit` — propose reviewed micro commits, apply on approval
- `/tomo pr [title]` — write a PR body and open the pull request via gh
- `/tomo arch` — explore the codebase and design arch improvements as RFC issues
- `/tomo trace [problem] [path]` — investigate and write a findings report
- `/tomo debt [focus] [severity]` — assess tech debt and file GitHub issues

## Personas

Every step runs under a persona: interactive steps **adopt** it in-session
(lens); the implement loop **spawns** personas as real subagents. The five
built-in personas are installed into your harness's agent dirs
(`.opencode/agents/`, `.claude/agents/`) or as Copilot CLI agent
profiles. Copilot CLI installs each Tomo command as a skill named
`tomo-<command>` under `.github/skills/` (project) or `~/.copilot/skills/`
(user); prompt or invoke it as `/tomo-<command>`. These are skills, not native
slash-command files. Drive personas via `copilot --agent=<name> --prompt "..."`
or `/agent` in the CLI:

- `analyst` — requirements
- `architect` — design, plan, `/tomo arch`, `/tomo debt`
- `coder` — each implement task
- `reviewer` — senior-SWE phase review, `/tomo trace`
- `security` — dedicated per-phase security pass in the implement loop

Override the binding per project in `.tomo/settings.json`
(`personas.map.<step>` → built-in id, alias, or file path;
`personas.custom.<alias>` → file path). Profiles live at
`.tomo/personas/<name>.md`; `/tomo status` shows the effective mapping.

## State and gates

- `.tomo/state.json` holds the phase machine under `changes`: each change's
  `draft → await-approval → approved` gates, plus the `implement` run phase.
- Gated commands write their artifact, set the phase to `await-approval`, and
  ask for approval. `/tomo continue` resumes the current phase.
- `.tomo/log.md` is the changeset paper trail; `.tomo/SLIP.md` is the handoff
  anchor for compact/resume.
- Settings merge: `~/.config/tomo/settings.json` (global) overridden by
  `.tomo/settings.json` (project). `settings.gates` is `block` | `prompt` |
  `auto`.

If this project has no `.tomo/` yet, say so and suggest starting with
`/tomo requirements`. This command is informational only.

Arguments (may be empty): $ARGUMENTS
