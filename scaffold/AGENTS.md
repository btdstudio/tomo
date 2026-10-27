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
| `tomo add kumi <name> [module-path]` | scaffold a kumi Go+HTMX app into `src/apps/<name>/` |
| `tomo add hachigo <name>` | scaffold a hachigo site into `src/apps/<name>/` |
| `tomo status` | show the active change's phase/status |
| `tomo settings` | show/merge settings |
| `tomo switch <change>` | switch the active change |
| `tomo issues new|list|show` | manage `00_issues/` |
| `tomo commit` | propose conventional micro-commits |
| `tomo pr [title]` | open a pull request |
| `tomo help` | usage |

The folders/personas below describe *what* to do; `tomo` is a plain CLI — run
it like `git` or `gh`. Prefer `tomo <command>` over hand-editing `.tomo/` or
`00_issues/` when the binary is on your PATH.

## How to start a change

1. **Create an issue** — drop `00_issues/<id>.md` (or run `tomo issues new`).
2. **Point the workspace at it** — set `baseSource` in `.tomo/settings.json`.
3. **Seed the change** in `.tomo/state.json` (a `<n>-<slug>` id, `phase: requirements`).
4. **Kick off** with a prompt like: *"Start the loop on `00_issues/1.md`."*

## Routing table

| Task / Intent | Directory | Persona | Read | Skip |
|---|---|---|---|---|
| Write requirements | `01_planning/` | analyst | `01_planning/context.md`, `00_issues/*` | `src/*`, `03_testing/*` |
| Design solution | `01_planning/` | architect | `01_planning/context.md`, `specs/<n>-<slug>/requirements.md` | `src/*` |
| Plan tasks | `01_planning/` | architect | `specs/<n>-<slug>/design.md` | `src/*` |
| Implement code | `02_implementation/` | coder | `02_implementation/context.md`, `01_planning/specs/<n>-<slug>/*`, `src/apps/*` | `04_refactoring/*` |
| Test | `03_testing/` | qa | `03_testing/context.md`, `src/apps/*`, `specs/<n>-<slug>/requirements.md` | `00_issues/*` |
| Review | `04_refactoring/` | reviewer | `04_refactoring/context.md`, `src/apps/*`, `specs/<n>-<slug>/*` | `00_issues/*` |
| Security pass | `04_refactoring/` | security | `04_refactoring/context.md`, `src/apps/*` | `00_issues/*` |

## Naming conventions

- Specs: `01_planning/specs/<n>-<slug>/requirements.md|design.md|tasks.json`
- Issues: `00_issues/<id>.md`
- Personas: `<stage>/personas/<name>.md`
- Templates: `01_planning/templates/<artifact>.md`
- Apps: `src/apps/<name>/`

## Workflow rules

- Requirements before design; design before plan; plan before implement.
- Each phase stops at a gate for human approval — never proceed past a gate on
  your own (tracked in `.tomo/state.json`).
- Read only the files the routing table names for your task; skip the rest.
- Always write or update tests for new behavior.
- Use conventional commit messages.

## Tech stack & standards

- <FILL IN: frameworks, style/lint, test command>

## Directory map

```
├── CLAUDE.md
├── AGENTS.md
├── .gitignore
├── .tomo/                # workspace state (settings/state/log/manifest)
├── 00_issues/            # local issue store (<id>.md)
├── 01_planning/          # analyst + architect
├── 02_implementation/    # coder
├── 03_testing/           # qa
├── 04_refactoring/       # reviewer + security
└── src/apps/             # monorepo root — one dir per app
```
