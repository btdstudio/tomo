# Tomo, harness-agnostic AI SDD (AI-SDLC) toolset

This file also applies to agents working on the tomo repo itself. Especially to
them.

## The non-negotiable rules

- Command bodies are single-sourced: the canonical prompt body for each command
  lives in `content/commands/` (and the shared approval gate in
  `content/gate.md`). `src/commands/` holds the command contract as
  frontmatter only. Rendering inlines each body (expanding the
  `<!-- tomo:include gate.md -->` directive in gated bodies) into the wrappers,
  so no command reads a body file at runtime. Never hand-edit anything under
  `adapters/common/` (a gitignored generated cache), `adapters/claude/` aside —
  edit the canonical body in `content/commands/` (or the frontmatter in
  `src/commands/`) and re-run `npm run install` for the change to reach a
  harness. `npm test` runs the consistency + render + placement tests; it fails
  if the content store disagrees with the command registry, if a body contains
  live harness template syntax (`` !`cmd` ``, `@path`, `$1`), or if
  install/render ever drift. Keep it green.
- No third-party runtime or build dependencies. Node >= 20, standard library
  only. Test the build with `npm test`; keep it green. `npm test` is safe from a
  clean checkout — the tests seed the `adapters/common/` render cache themselves,
  so it must never require a prior `npm run install` to pass.
- Never commit `.tomo/` (per-project workspace, gitignored) or
  `adapters/common/` (generated cache, gitignored).

## How this repo works

- Build is folded into install: `scripts/install.js` first renders the
  body-inlined wrappers and persona casts into the gitignored
  `adapters/common/` cache (`scripts/render.js`, a library — it has no CLI),
  then copies them into a project or symlinks them from the cache into
  harness-global dirs. Nothing generated is ever committed.
- Each canonical command body is a markdown prompt in `content/commands/`; the
  command contract in `src/commands/<name>.md` is YAML frontmatter only
  (`name`, `description`, `args`, `gate` `none|block|prompt|auto`).
  `$ARGUMENTS` substitutes in both opencode and Claude Code; bodies without it
  get a trailing arguments line. Providers with a divergent command format
  (Claude's `allowed-tools`, Copilot/Antigravity Skills) own a `commandCast` in
  `src/lib/providers.js`.
- Gated commands' bodies contain the `<!-- tomo:include gate.md -->` directive
  exactly when `gate != none`; `render.checkContent()` verifies the two agree.
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