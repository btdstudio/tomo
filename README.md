# Tomo (友) 🤝 — folder-driven AI SDLC toolset

Tomo scaffolds and maintains an Interpretable Context Methodology (ICM)
workspace: a folder structure that drives a spec-first software development
loop — `requirements → design → plan → implement`, with review and security
passes — for any AI harness, or none. The filesystem is the architecture; no
framework or runtime is required after scaffold.

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
| `tomo init [dir]` | scaffold the ICM structure |
| `tomo update` | refresh managed files to the installed version |
| `tomo add kumi <name> --path <dir> --remote <url> [module-path]` | scaffold a [kumi Go+HTMX app](https://github.com/btdstudio/kumi) outside the workspace and make it the target repo |
| `tomo add hachigo <name> --path <dir> --remote <url>` | scaffold a [hachigo site](https://github.com/btdstudio/hachigo) outside the workspace and make it the target repo |
| `tomo status` | show the active change |
| `tomo settings` | show/merge settings |
| `tomo switch [--all] [<change>]` | list changes, or switch the active change |
| `tomo change new <title> [--issue <id>]` | create a change + issue + switch |
| `tomo change close <change> --reason <r>` | close a change |
| `tomo gate <phase> <status> [--force]` | advance the active change's gate |
| `tomo reopen <phase> --reason <r>` | move back to a phase and clear later approvals |
| `tomo issues new\|list\|show` | manage `00_issues/` |
| `tomo repo add\|list\|path\|doctor` | manage this workspace's one target repo |
| `tomo env` | print harness flags for the target repo |
| `tomo doctor [--install]` | check toolchain prerequisites |
| `tomo list` | list installed tools + versions |
| `tomo commit [git commit args]` | commit what's staged in the target repo, one micro-commit at a time |
| `tomo pr [title]` | open a pull request for the target repo |
| `tomo help` | usage |

## The loop

A change flows through four gated phases, each owned by a persona:

1. **requirements** (analyst) — turn an issue into an unambiguous spec.
2. **design** (architect) — the smallest solution that satisfies it.
3. **plan** (architect) — break it into dependency-ordered tasks.
4. **implement** (coder) — execute tasks one at a time, with tests.

`review` and `security` are read-only passes after implement; `qa` owns the
testing stage. Each phase stops at a gate for your approval. With the default
`gates: block` setting, `tomo gate` refuses a phase until the earlier ones are
finished; `--force` overrides this and records it in `LOG.md`. State changes
commit `STATE.json` and `LOG.md` for you, without pushing.

## The workspace

A workspace holds planning and state, and targets exactly one repo, which holds the
code. Keep each workspace in its own git repo.

```
├── AGENTS.md          root map + routing table + command list
├── STATE.json         per-change phase, gates, sources (committed)
├── LOG.md             gate history (committed)
├── REPOS.json         logical repos and remotes (committed)
├── .tomo/             settings + manifest (committed), active change + repo paths (local)
├── 00_issues/         local issue store (<id>.md)
├── 01_planning/       analyst + architect (specs, personas, templates)
├── 02_implementation/ coder
├── 03_testing/        qa
└── 04_refactoring/    reviewer + security
```

## Development

```sh
go build -o tomo ./cmd/tomo   # build
go test ./...                 # all tests (content/golden/placement)
go test ./internal/icm -update # regenerate golden fixtures
```

Runtime: Go >= 1.22, standard library only.
