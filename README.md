# Tomo (友) 🤝 — folder-driven AI SDLC toolset

Tomo scaffolds and maintains an Interpretable Context Methodology (ICM)
workspace: a folder structure that takes one change in one repo from a ticket to a
pull request, through a spec-first loop, for any AI harness or none. The filesystem
is the architecture; no framework or runtime is required after scaffold.

## Attribution

The Interpretable Context Methodology (ICM) that tomo implements is inspired by:

- **Jake Van Clief**, **David McDermott** — ["Interpretable Context Methodology: Folder Structure as Agentic Architecture"](https://arxiv.org/abs/2603.16021)

## Install

```sh
go install github.com/btdstudio/tomo/cmd/tomo@latest
```

## Commands

| Command | Purpose |
|---|---|
| `tomo init [dir] [--no-git]` | scaffold the ICM structure; `--no-git` makes a local workspace (`vcs: none`) |
| `tomo update` | refresh managed files to the installed version |
| `tomo add kumi <name> --path <dir> --remote <url> [module-path]` | scaffold a [kumi Go+HTMX app](https://github.com/btdstudio/kumi) outside the workspace and make it the target repo |
| `tomo add hanga <name> --path <dir> --remote <url>` | scaffold a [hanga site](https://github.com/btdstudio/hanga) outside the workspace and make it the target repo |
| `tomo status` | show the active change |
| `tomo settings` | show/merge settings |
| `tomo switch [--all] [<change>]` | list changes, or switch the active change |
| `tomo change new <title> [--issue <id>]` | create a change + native ticket + switch |
| `tomo change new --from <ticket.md>` | start a change from a planned ticket |
| `tomo change close <change> --reason <r>` | close a change |
| `tomo gate <phase> <status> [--reason <r>] [--force]` | advance the active change's gate |
| `tomo reopen <phase> --reason <r>` | move back to a phase and clear later approvals |
| `tomo issues new\|list\|show` | manage the tickets in `00_issues/` |
| `tomo repo add\|list\|path\|doctor` | manage this workspace's one target repo |
| `tomo env` | print harness flags for the target repo |
| `tomo doctor [--install]` | check toolchain prerequisites |
| `tomo list` | list installed tools + versions |
| `tomo commit [git commit args]` | commit what's staged in the target repo, one micro-commit at a time |
| `tomo pr [title]` | open a pull request for the target repo, linking the ticket |
| `tomo help` | usage |

## The loop

A change flows through nine phases, each owned by a persona:

1. **intake** (analyst) — settle what the issue asks and why.
2. **requirements** (analyst) — turn it into an unambiguous spec.
3. **design** (architect) — the smallest solution that satisfies it.
4. **plan** (architect) — break it into dependency-ordered tasks.
5. **implement** (coder) — execute tasks one at a time, with tests.
6. **test** (qa) — report pass/fail per acceptance criterion.
7. **review** (reviewer) — correctness, scope and test adequacy.
8. **security** (security) — vulnerabilities and security debt. Skippable, with a reason.
9. **ship** — a branch, micro-commits, and a PR linking the ticket.

Implement and ship are run phases; the rest stop at a gate for your approval. With
the default `gates: block` setting, `tomo gate` refuses a phase until the earlier ones
are finished; `--force` overrides this and records it in `LOG.md`. A test failure or
serious finding loops back with `tomo reopen implement`, which clears the approvals
after it.

In a git workspace, state changes commit `STATE.json` and `LOG.md` for you, without
pushing, and gates also commit the change's spec folder. In a local workspace
(`vcs: none`) they only write the files. `tomo commit` and `tomo pr` act on the
target repo, which is always git.

## The workspace

A workspace holds planning and state, and targets exactly one repo, which holds the
code. Keep each workspace in its own git repo, or declare it local (see
[Local workspaces](#local-workspaces)).

```
├── AGENTS.md          root map + routing table + command list
├── STATE.json         per-change phase, gates, sources (shared)
├── LOG.md             gate history (shared)
├── REPOS.json         logical repos and remotes (shared)
├── .tomo/             settings + manifest (shared), active change + repo paths (local)
├── 00_issues/         tickets (<ID>.md)
├── 01_planning/       analyst + architect (specs, personas, templates)
├── 02_implementation/ coder
├── 03_testing/        qa
└── 04_refactoring/    reviewer + security
```

## Local workspaces

The `vcs` setting says whether a workspace lives in git. It's a core
[kiso](https://github.com/btdstudio/kiso) setting and goes in `.tomo/settings.json` or, for every workspace,
`~/.config/tomo/settings.json`.

| Value | Meaning |
|---|---|
| `git` (default) | `init` runs `git init` and scaffolds `.gitignore`, `.gitattributes` and `.gitkeep`. State changes and gates commit. |
| `none` | Plain local files. `init` skips `git init`, `.gitattributes` and `.gitkeep`, and `update` neither restores nor reports them. It keeps `.gitignore`, so `.tomo/active`, `.tomo/paths.json` and `.tomo/SLIP.md` stay out of an enclosing repo or one you create later. State changes and gates write `STATE.json` and `LOG.md` without running git or warning. |

`tomo init --no-git` records `"vcs": "none"`, and a global `vcs: none` applies to
`init` too. `tomo status` and `tomo settings` show the mode. Any other value is an
error.

To move a local workspace into git, run `git init`, set `"vcs": "git"`, and run
`tomo update` to put `.gitattributes` and `.gitkeep` back; `.gitignore` is already
there. That's also the fix if you used `--no-git` only to avoid a repo nested inside
another one. Going the other way leaves existing git files on disk; tomo just stops
managing them.

## Getting started

```sh
tomo init ../workspaces/my-app
cd ../workspaces/my-app
tomo repo add my-app git@github.com:me/my-app.git --path ~/source/my-app
git add -A && git commit -m "chore: scaffold workspace"
tomo change new "Add password reset"
eval claude $(tomo env)
```

For a new app, `tomo add kumi my-app --path ~/source/my-app --remote <url>` scaffolds
it and registers it instead.

## Tickets

`00_issues/` holds tickets in a schema shared with
[zumen](https://github.com/btdstudio/zumen), through
[kiso](https://github.com/btdstudio/kiso). A ticket created here gets an `N-0001` id.
One planned elsewhere comes in as a snapshot:

```sh
tomo change new --from ../planning/specs/1-release/tickets/S-0014.md
```

It has to be ready or published, and land in this workspace's target repo. A ready ticket
must have been readied through zumen (`zumen tickets ready`). One set to ready by hand, or
edited since, is refused. If it's been published, the analyst reads the live item first,
following `01_planning/sync.md`, and requirements become a confirmation of what was
already approved. It's still a gate, because tomo approves something zumen didn't: how
this ticket lands in this repo.

On ship, `tomo status` suggests a branch that includes the ticket's id, and `tomo pr`
links the ticket, its external ids, spec and ADRs. Neither tool needs the other to run.

## Development

```sh
go build -o tomo ./cmd/tomo   # build
go test ./...                 # all tests (content/golden/placement)
go test ./internal/icm -update # regenerate golden fixtures
```

Runtime: Go >= 1.22, standard library only, plus the first-party
[kiso](https://github.com/btdstudio/kiso) core.
