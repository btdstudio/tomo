# Tomo, harness-agnostic AI SDD (AI-SDLC) toolset

This file also applies to agents working on the tomo repo itself. Especially to
them.

## The non-negotiable rules

- Command bodies are single-sourced: the canonical prompt body for each command
  lives in `content/commands/` (and the shared approval gate in
  `content/gate.md`). `src/commands/` holds the command contract as
  frontmatter only. Rendering inlines each body (expanding the
  `<!-- tomo:include gate.md -->` directive in gated bodies) into the wrappers,
  so no command reads a body file at runtime. The content store is embedded in
  the binary (`embed.go`, `//go:embed content src`). Never hand-edit anything
  under `adapters/` that is generated — edit the canonical body in
  `content/commands/` (or the frontmatter in `src/commands/`) and rebuild. `go
  test ./...` runs the consistency + render + placement tests; it fails if the
  content store disagrees with the command registry, if a body contains live
  harness template syntax (`` !`cmd` ``, `@path`, `$1`), or if install/render
  ever drift. Keep it green.
- No third-party runtime or build dependencies. Go >= 1.22, standard library
  only. Test the build with `go test ./...`; keep it green. `go test ./...` is
  safe from a clean checkout — the tests render from the embedded store, so it
  must never require a prior `./tomo install` to pass.
- Never commit `.tomo/` (per-project workspace, gitignored).

## How this repo works

- Build is folded into install: `tomo install` (the CLI in `cmd/tomo`) renders
  the body-inlined wrappers and persona casts in-memory from the embedded store
  (the render library in `internal/render`) and writes them into a project or
  the harness-global dirs. The binary is self-contained — no source checkout is
  required at runtime. Nothing generated is ever committed.
- Each canonical command body is a markdown prompt in `content/commands/`; the
  command contract in `src/commands/<name>.md` is YAML frontmatter only
  (`name`, `description`, `args`, `gate` `none|block|prompt|auto`).
  `$ARGUMENTS` substitutes in both opencode and Claude Code; bodies without it
  get a trailing arguments line. Providers with a divergent command format
  (Claude's `allowed-tools`, Copilot/Antigravity Skills) own a `commandCast` in
  `internal/providers`.
- The `tomo` binary also replaces the old `.tomo/lib/*.js` runtime helpers: the
  command bodies invoke `tomo settings`, `tomo state set-gate`, `tomo persona`,
  `tomo issue`, etc. (from `.tomo/bin/tomo` for a project install, or `PATH`
  for a global install), falling back to editing `.tomo/state.json` directly
  when the binary is absent.
- Gated commands' bodies contain the `<!-- tomo:include gate.md -->` directive
  exactly when `gate != none`; `tomo check` (render.CheckContent) verifies the
  two agree.
- Global installs seed read grants for the tomo config dir (global
  `settings.json`) into opencode (`permission.external_directory`) and
  Antigravity settings; Claude commands carry it as `allowed-tools`.
- Gates are prompted contracts, not enforcement: gated command bodies must stop
  and ask. Settings are mergeable JSON, project supersedes global.

## Process for this repo

Ship the SDD loop in the planned phases (tracked in the active change's
`.tomo/specs/<n>-<slug>/tasks.json`). Requirements before design, design before
plan. Gate each phase; track progress in `.tomo/` only after running `/tomo`
commands, otherwise keep the working tree clean.
