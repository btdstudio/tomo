# Tomo (友) 🤝 — folder-driven AI SDLC toolset

Tomo scaffolds and maintains an Interpretable Context Methodology (ICM)
workspace: a folder structure that drives a spec-first software development
loop — `requirements → design → plan → implement`, with review and security
passes — for any AI harness, or none. The filesystem is the architecture; no
framework or runtime is required after scaffold.

## Attribution

The Interpretable Context Methodology (ICM) that tomo implements is inspired by:

- **Jake Van Clief** — ["Stop Building AI Agents. Use This Folder System Instead"](https://youtu.be/MkN-ss2Nl10)

## Install

```sh
go install github.com/btdstudio/tomo/cmd/tomo@latest
```

## Commands

| Command | Purpose |
|---|---|
| `tomo init [dir]` | scaffold the ICM structure |
| `tomo update` | refresh managed files to the installed version |
| `tomo add kumi <name> [module-path]` | scaffold a kumi Go+HTMX app into `src/apps/<name>/` |
| `tomo add hachigo <name>` | scaffold a hachigo site into `src/apps/<name>/` |
| `tomo status` | show the active change |
| `tomo settings` | show/merge settings |
| `tomo switch <change>` | switch the active change |
| `tomo issues new\|list\|show` | manage `00_issues/` |
| `tomo doctor [--install]` | check toolchain prerequisites |
| `tomo list` | list installed tools + versions |
| `tomo commit` | stage all + commit (conventional) |
| `tomo pr [title]` | open a pull request via `gh` |
| `tomo help` | usage |

## The loop

A change flows through four gated phases, each owned by a persona:

1. **requirements** (analyst) — turn an issue into an unambiguous spec.
2. **design** (architect) — the smallest solution that satisfies it.
3. **plan** (architect) — break it into dependency-ordered tasks.
4. **implement** (coder) — execute tasks one at a time, with tests.

`review` and `security` are read-only passes after implement; `qa` owns the
testing stage. Each phase stops at a gate for your approval.

## The workspace

```
├── AGENTS.md          root map + routing table + command list
├── .tomo/             workspace state (settings, state, log, manifest)
├── 00_issues/         local issue store (<id>.md)
├── 01_planning/       analyst + architect (specs, personas, templates)
├── 02_implementation/ coder
├── 03_testing/        qa
├── 04_refactoring/    reviewer + security
└── src/apps/          monorepo root — one dir per app
```

## Development

```sh
go build -o tomo ./cmd/tomo   # build
go test ./...                 # all tests (content/golden/placement)
go test ./internal/icm -update # regenerate golden fixtures
```

Runtime: Go >= 1.22, standard library only.
