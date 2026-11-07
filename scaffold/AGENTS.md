# <project> — ICM workspace

Layer 1 root map (Interpretable Context Methodology). Every agent reads this
file first, then navigates by the routing table below. `CLAUDE.md` just points
here; this file is harness-agnostic and the single source of truth.

## The loop

A change flows through nine phases, each owned by a persona. Implement and ship are
run phases (`in-progress`, then `complete`); the rest are gated.

`intake → requirements → design → plan → implement → test → review → security → ship`

- **intake** (analyst) — settle the issue in `00_issues/`: what's asked and why.
- **requirements** (analyst) — turn a raw issue into an unambiguous spec.
- **design** (architect) — the smallest solution that satisfies the requirements.
- **plan** (architect) — break the design into dependency-ordered tasks.
- **implement** (coder) — execute tasks, one at a time, with tests.
- **test** (qa) — report pass/fail per acceptance criterion.
- **review** (reviewer) — correctness, scope and test adequacy. Read-only.
- **security** (security) — vulnerabilities and security debt. Read-only. Skippable,
  with a reason.
- **ship** — branch, micro-commits, and a PR linking the issue.

A test failure, or a critical or high finding, loops back with
`tomo reopen implement --reason <r>`. That clears test and review, which then run
again against the fixed code.

## Commands (the `tomo` CLI)

| Command | Purpose |
|---|---|
| `tomo init [dir]` | scaffold this ICM structure |
| `tomo update` | refresh managed files to the installed version |
| `tomo add kumi <name> --path <dir> --remote <url> [module-path]` | scaffold a kumi Go+HTMX app outside the workspace and make it the target repo |
| `tomo add hanga <name> --path <dir> --remote <url>` | scaffold a hanga site outside the workspace and make it the target repo |
| `tomo status` | show the active change's phase/status |
| `tomo settings` | show/merge settings |
| `tomo switch [--all] [<change>]` | list changes (`--all` includes closed ones), or switch the active change |
| `tomo change new <title> [--issue <id>]` | create a change + issue + switch |
| `tomo change close <change> --reason <r>` | close a change |
| `tomo gate <phase> <status> [--force]` | advance the active change's gate (earlier phases must be finished) |
| `tomo reopen <phase> --reason <r>` | move back to a phase and clear later approvals |
| `tomo issues new|list|show` | manage `00_issues/` |
| `tomo repo add\|list\|path\|doctor` | manage this workspace's one target repo |
| `tomo env` | print harness flags (`--add-dir <path>`) for the target repo |
| `tomo doctor [--install]` | check toolchain prerequisites |
| `tomo list` | list installed tools + versions |
| `tomo commit [git commit args]` | commit what's staged in the target repo. Split work into reviewed micro-commits: propose the groups, stage one, commit it with a conventional message, repeat |
| `tomo pr [title]` | open a pull request for the target repo |
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
| Settle the issue | `00_issues/` | analyst | `00_issues/<id>.md` | the target repo, `02_implementation/*` |
| Write requirements | `01_planning/` | analyst | `01_planning/context.md`, `00_issues/*` | the target repo, `03_testing/*` |
| Design solution | `01_planning/` | architect | `01_planning/context.md`, `specs/<n>-<slug>/requirements.md` | the target repo |
| Plan tasks | `01_planning/` | architect | `specs/<n>-<slug>/design.md` | the target repo |
| Implement code | `02_implementation/` | coder | `02_implementation/context.md`, `01_planning/specs/<n>-<slug>/*`, the target repo (`tomo repo list`) and its `AGENTS.md` | `04_refactoring/*` |
| Test | `03_testing/` | qa | `03_testing/context.md`, the target repo (`tomo repo list`) and its `AGENTS.md`, `specs/<n>-<slug>/requirements.md` | `00_issues/*` |
| Review | `04_refactoring/` | reviewer | `04_refactoring/context.md`, the target repo (`tomo repo list`) and its `AGENTS.md`, `specs/<n>-<slug>/*` | `00_issues/*` |
| Security pass | `04_refactoring/` | security | `04_refactoring/context.md`, the target repo (`tomo repo list`) and its `AGENTS.md` | `00_issues/*` |
| Ship | the target repo | — | `specs/<n>-<slug>/tasks.json`, `00_issues/<id>.md` | `02_implementation/*` |

## Naming conventions

- Specs: `01_planning/specs/<n>-<slug>/requirements.md|design.md|tasks.json|test-results.md|code-review.md|security-review.md`
- Issues: `00_issues/<id>.md`
- Personas: `<stage>/personas/<name>.md`
- Templates: `01_planning/templates/<artifact>.md`

## Workflow rules

- Each phase in order: a phase can't be gated until the earlier ones are approved,
  skipped or complete.
- Each phase stops at a gate for human approval — never proceed past a gate on
  your own. Record transitions with `tomo gate <phase> <status>`, never by editing
  `STATE.json`. `--force` overrides the order check and is logged; only use it
  when a human says so.
- Use `tomo reopen <phase> --reason <r>` to go back: it clears the approvals after
  that phase, because they no longer hold.
- `tomo change new`, `gate`, `reopen` and `change close` commit `STATE.json` and
  `LOG.md` for you, leaving anything else you have staged alone. `gate` also commits
  the change's spec folder, so write the artifact before you gate it. They don't push.
- Read only the files the routing table names for your task; skip the rest.
- Code lives in the target repo, not in this workspace. A workspace targets exactly
  one repo: `tomo repo list` shows where it is, and `tomo env` prints the flags that
  give a harness access to it, such as `eval claude $(tomo env)`. Read the target
  repo's own `AGENTS.md` before touching its code.
- `tomo commit` and `tomo pr` act on the target repo. Gates commit this workspace's
  specs; commit anything else here, such as issues you edit later, with plain `git`.
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
└── 04_refactoring/       # reviewer + security
```
