# <project> — ICM workspace

Layer 1 root map (Interpretable Context Methodology). Every agent reads this
file first, then navigates by the routing table below. `CLAUDE.md` just points
here; this file is harness-agnostic and the single source of truth.

## The loop

A change flows through four gated phases, each owned by one or more personas:

`requirements → design → plan → implement`

- **requirements** (analyst) — turn a raw issue into an unambiguous spec.
- **design** (architect) — the smallest solution that satisfies the requirements.
- **plan** (architect) — break the design into dependency-ordered tasks.
- **implement** (coder) — execute tasks, one at a time, with tests.

Two read-only passes follow implementation: **review** (reviewer) and
**security** (security). `qa` owns the testing stage — a hybrid extension of
tomo, which folds test authoring into the coder and adequacy into the reviewer.

## Commands (the `tomo` CLI)

| Command | Purpose |
|---|---|
| `tomo init [dir]` | scaffold this ICM structure |
| `tomo update` | refresh managed files to the installed version |
| `tomo add kumi <name> [module-path]` | scaffold a kumi Go+HTMX app (location from `.tomo/settings.json`: `outputDir` + `projectType`) |
| `tomo add hachigo <name>` | scaffold a hachigo site (location from `.tomo/settings.json`: `outputDir` + `projectType`) |
| `tomo status` | show the active change's phase/status |
| `tomo settings` | show/merge settings |
| `tomo switch [--all] [<change>]` | list changes (`--all` includes closed ones), or switch the active change |
| `tomo change new <title> [--issue <id>]` | create a change + issue + switch |
| `tomo change close <change> --reason <r>` | close a change |
| `tomo gate <phase> <status> [--force]` | advance the active change's gate (earlier phases must be finished) |
| `tomo reopen <phase> --reason <r>` | move back to a phase and clear later approvals |
| `tomo issues new|list|show` | manage `00_issues/` |
| `tomo repo add\|list\|path\|doctor` | manage the repos this workspace works with |
| `tomo env` | print harness flags (`--add-dir <path>`) for this workspace's repos |
| `tomo doctor [--install]` | check toolchain prerequisites |
| `tomo list` | list installed tools + versions |
| `tomo commit [git commit args]` | commit what's staged. Split work into reviewed micro-commits: propose the groups, stage one, commit it with a conventional message, repeat |
| `tomo pr [title]` | open a pull request |
| `tomo help` | usage |

The folders/personas below describe *what* to do; `tomo` is a plain CLI — run
it like `git` or `gh`. Prefer `tomo <command>` over hand-editing `STATE.json` or
`00_issues/` when the binary is on your PATH.

## How to start a change

1. **Create a change** — `tomo change new "Title"`. This writes the issue
   (`00_issues/<id>.md`), makes a `01_planning/specs/<n>-<slug>/` folder, seeds
   `STATE.json`, and switches to it. Use `--issue <id>` for an existing issue.
2. **Kick off** with a prompt like: *"Start the loop on `00_issues/1.md`."*

## Routing table

| Task / Intent | Directory | Persona | Read | Skip |
|---|---|---|---|---|
| Write requirements | `01_planning/` | analyst | `01_planning/context.md`, `00_issues/*` | `src/*`, `03_testing/*` |
| Design solution | `01_planning/` | architect | `01_planning/context.md`, `specs/<n>-<slug>/requirements.md` | `src/*` |
| Plan tasks | `01_planning/` | architect | `specs/<n>-<slug>/design.md` | `src/*` |
| Implement code | `02_implementation/` | coder | `02_implementation/context.md`, `01_planning/specs/<n>-<slug>/*`, `app code (see settings.outputDir/projectType)` | `04_refactoring/*` |
| Test | `03_testing/` | qa | `03_testing/context.md`, `app code (see settings.outputDir/projectType)`, `specs/<n>-<slug>/requirements.md` | `00_issues/*` |
| Review | `04_refactoring/` | reviewer | `04_refactoring/context.md`, `app code (see settings.outputDir/projectType)`, `specs/<n>-<slug>/*` | `00_issues/*` |
| Security pass | `04_refactoring/` | security | `04_refactoring/context.md`, `app code (see settings.outputDir/projectType)` | `00_issues/*` |

## Naming conventions

- Specs: `01_planning/specs/<n>-<slug>/requirements.md|design.md|tasks.json`
- Issues: `00_issues/<id>.md`
- Personas: `<stage>/personas/<name>.md`
- Templates: `01_planning/templates/<artifact>.md`
- Apps: location from `.tomo/settings.json` (`outputDir` + `projectType`; defaults `src` + `monorepo` → `src/apps/<name>/`)

## Workflow rules

- Requirements before design; design before plan; plan before implement.
- Each phase stops at a gate for human approval — never proceed past a gate on
  your own. Record transitions with `tomo gate <phase> <status>`, never by editing
  `STATE.json`. `--force` overrides the order check and is logged; only use it
  when a human says so.
- Use `tomo reopen <phase> --reason <r>` to go back: it clears the approvals after
  that phase, because they no longer hold.
- `tomo change new`, `gate`, `reopen` and `change close` commit `STATE.json` and
  `LOG.md` for you, leaving anything else you have staged alone. They don't push.
- Read only the files the routing table names for your task; skip the rest.
- App code location comes from `.tomo/settings.json`: `outputDir` (default `src`)
  and `projectType` (`monorepo` default → `<outputDir>/apps/<name>/`;
  `standard` → `<outputDir>/` is the app root). Consult both before touching
  app code.
- Always write or update tests for new behavior.
- Use conventional commit messages.

## Tech stack & standards

- <FILL IN: frameworks, style/lint, test command>

## Directory map

```
├── CLAUDE.md
├── AGENTS.md
├── .gitignore
├── .gitattributes
├── STATE.json            # per-change phase, gates, sources (committed)
├── LOG.md                # gate history (committed)
├── REPOS.json            # logical repos and remotes (committed)
├── .tomo/                # settings + manifest (committed), active change + repo paths (local)
├── 00_issues/            # local issue store (<id>.md)
├── 01_planning/          # analyst + architect
├── 02_implementation/    # coder
├── 03_testing/           # qa
├── 04_refactoring/       # reviewer + security
└── src/apps/             # monorepo root — one dir per app
```
