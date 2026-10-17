# Tomo (友), harness-agnostic AI SDD (AI-SDLC) toolset

Tomo runs a structured software development lifecycle as an agent workflow that
installs into any AI coding harness. One canonical prompt body per step
(`content/commands/<name>.md`) is rendered into a wrapper per harness, so the
same process and approval gates run identically everywhere. State lives in
files on disk (`.tomo/`), not in the harness.

## The process

1. Review base requirements (local file/PRD, or GitHub Issue)
2. Ask clarifying questions to hone in on final requirements
3. Write requirements document — approval gate
4. Analyze code base to determine the design of the solution
5. Write design document — approval gate
6. Break up design into phases and tasks — approval gate
7. `/tomo implement` runs the plan to completion: a **coder** subagent
   implements each task, a **reviewer** checks each phase, and critical/high
   issues go back to implement — progress persists in `tasks.json`
8. After each phase a **security** persona reviews generated code
9. Update project documentation to match new changes
10. Ask to commit/push/PR (`/tomo commit` proposes reviewed micro commits)

## Commands

| Phase / command | opencode | Claude Code | Antigravity CLI | gate | args |
|---|---|---|---|---|---|
| help | `/tomo-help` | `/tomo help` | `/tomo-help` | none | |
| status | `/tomo-status` | `/tomo status` | `/tomo-status` | none | |
| settings | `/tomo-settings` | `/tomo settings` | `/tomo-settings` | none | |
| init | `/tomo-init` | `/tomo init` | `/tomo-init` | none | |
| requirements | `/tomo-requirements` | `/tomo requirements` | `/tomo-requirements` | block | `$ARGUMENTS` → source |
| design | `/tomo-design` | `/tomo design` | `/tomo-design` | block | |
| plan | `/tomo-plan` | `/tomo plan` | `/tomo-plan` | block | |
| continue | `/tomo-continue` | `/tomo continue` | `/tomo-continue` | none | `$ARGUMENTS` → source |
| implement | `/tomo-implement` | `/tomo implement` | `/tomo-implement` | none | |
| handoff | `/tomo-handoff` | `/tomo handoff` | `/tomo-handoff` | none | |
| pickup | `/tomo-pickup` | `/tomo pickup` | `/tomo-pickup` | none | |
| switch | `/tomo-switch` | `/tomo switch` | `/tomo-switch` | none | `$ARGUMENTS` → change |
| issues | `/tomo-issues` | `/tomo issues` | `/tomo-issues` | none | `$ARGUMENTS` → verb |
| commit | `/tomo-commit` | `/tomo commit` | `/tomo-commit` | none | |
| pr | `/tomo-pr` | `/tomo pr` | `/tomo-pr` | none | `$ARGUMENTS` → title |
| arch | `/tomo-arch` | `/tomo arch` | `/tomo-arch` | none | |
| trace | `/tomo-trace` | `/tomo trace` | `/tomo-trace` | none | `$ARGUMENTS` → problem + output path |
| debt | `/tomo-debt` | `/tomo debt` | `/tomo-debt` | none | `$ARGUMENTS` → focus + severity |

All eighteen commands are live in opencode, Claude Code, GitHub Copilot CLI,
Antigravity CLI, and OpenAI Codex. Copilot and Antigravity ship commands as
Skills prompted as `/tomo-<command>`; Codex ships them as Skills the model
loads by name. Approved gates (`requirements → design → plan`) persist in
`.tomo/state.json`, so your place survives across harnesses, sessions, and
tools. Each spec keeps its own phase machine, so `/tomo switch` pauses one work
stream and resumes another.

## Personas

Every step runs under a named persona. Interactive steps — `requirements`
(**analyst**), `design`/`plan` (**architect**), `/tomo arch` and `/tomo debt`
(architect), `/tomo trace` (reviewer) — read the profile at
`.tomo/personas/<name>.md` and **adopt** it in-session. The implement loop
instead **spawns** personas as real harness subagents: `coder` per task,
`reviewer` per phase, and a `security` pass per phase.

Profiles live in `src/personas/`; each install casts them into opencode-agent,
Claude-subagent, GitHub Copilot, Antigravity custom-agent, and Codex TOML
subagent formats. Override the binding per project in `.tomo/settings.json`:

```json
{
  "personas": {
    "map":    { "plan": "lead", "implement-security": "personas/extra.md" },
    "custom": { "lead": "personas/lead-arch.md" }
  }
}
```

`map` keys are `requirements | design | plan | implement | implement-review |
implement-security | arch | trace | debt`; values are a built-in persona
(`analyst | architect | coder | reviewer | security`), an alias, or a file
path. `custom` maps alias → path.

## State, settings, gates

- `.tomo/` workspace (never committed): `state.json` phase machine
  (`draft → await-approval → approved`), `specs/<n>-<slug>/` per-change
  artifacts, `log.md` paper trail, `SLIP.md` handoff context.
- Settings are mergeable JSON: global `~/.config/tomo/settings.json` +
  project `.tomo/settings.json`; project supersedes global.
- Approval gates are prompted contracts, not harness-enforced: every gated
  command stops and asks. `settings.gates` is `block` (default) | `prompt` |
  `auto`; `/tomo continue` resumes after approval or rejection.

## Layout

```
content/commands/   canonical command bodies (edit these)
content/gate.md     shared approval-gate contract
src/commands/       command contracts: YAML frontmatter only
src/personas/       canonical persona profiles (edit these)
src/templates/      artifact skeletons (requirements, design, tasks)
src/lib/            shared state/settings/personas/providers logic
adapters/common/    gitignored generated cache (never commit or hand-edit)
scripts/            render.js + install.js
tests/              node --test; consistency, render, and placement tests
```

## Installing

Tomo is a source-style install: keep this repo checked out anywhere (e.g.
`~/source/tomo`), then run its `install.js` from wherever you want the commands
to appear.

### Per-project (`--project`, default cwd)

```sh
node ~/source/tomo/scripts/install.js            # into the current project
node ~/source/tomo/scripts/install.js --project path/to/mypkg
```

Renders the wrappers and persona casts into the gitignored `adapters/common/`
cache, then copies them into the project and seeds its `.tomo/` workspace. The
harness-side dirs (`.opencode/`, `.claude/`, `.agents/`, `.codex/agents/`, plus
Copilot's `.github/skills/tomo-*` and `.github/agents/*.agent.md`) are created
only for the harness(es) you pick and are generated output — gitignored, never
committed; `.tomo/` always ships.

On an interactive terminal the installer asks which harness(es) you use
(`all | opencode | claude | copilot | antigravity | codex`, default `all`);
`--provider opencode[,claude]` (or `--provider all`) skips the prompt, and a
non-interactive run defaults to `all`. New harnesses are one entry in
`src/lib/providers.js`.

Re-run any time to refresh — projects seeded before the implement phase or
before personas existed need a re-run to pick up new state vocabulary and
persona files.

### Global (`--global`)

```sh
node ~/source/tomo/scripts/install.js --global
```

Symlinks the command wrappers and persona casts into your user-level harness
dirs, so every project and session has the `/tomo` commands and persona
subagents. A global install also pre-approves reads of the tomo config dir
(`~/.config/tomo/settings.json`) for each harness, merge-aware and idempotent.

Because these symlinks point **into this checkout**, keep the clone where you
installed it (like a dotfiles repo); edit a canonical body or contract and
re-run `node scripts/install.js --global` to refresh. `.tomo/` is per-project,
so a fresh project gets its workspace seeded the first time a phase runs.

Run `node scripts/install.js --help` for a recap with the resolved paths.
`npm run install` (inside this checkout) is the per-project form targeting the
tomo repo itself.

## Development

- `npm run install` — render + place into the current project
- `npm test` — content-store consistency + render + shape + placement tests

Runtime: Node >= 20, standard library only, no third-party dependencies.
