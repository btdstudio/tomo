# tomo — folder-driven AI SDLC toolset

This file applies to agents working on the tomo repo itself.

## Non-negotiable rules

- Go >= 1.22, standard library only — no third-party runtime or build deps. The
  one exception is first-party: [kiso](https://github.com/btdstudio/kiso), the
  shared core.
  `go test ./...` must stay green from a clean checkout.
- The ICM scaffold is the embedded content store: `scaffold/`, embedded via
  `//go:embed all:scaffold` (`embed.go`). Edit the scaffold there; `tomo check`
  and the golden test (`internal/icm/golden_test.go`) pin it. After editing the
  scaffold, run `go test ./internal/icm -update` to regenerate the golden.
- Workspace state is committed so approvals survive a clone: `STATE.json`,
  `LOG.md`, `REPOS.json`, `.tomo/settings.json`, `.tomo/manifest.json`. Only
  `.tomo/active`, `.tomo/paths.json` and `.tomo/SLIP.md` are local and gitignored — never commit them.

## Layout

```
scaffold/           the embedded ICM template (copied out by `tomo init`)
embed.go            //go:embed all:scaffold
cmd/tomo/           the CLI dispatch (init, update, add, check, helpers)
internal/icm/       kiso scaffold engine config, plus `tomo add`
internal/state/     kiso phase machine config, migration from .tomo/state.json
internal/settings/  settings shape, merged through kiso (global ← project)
internal/issues/    ticket store: native tickets, imports, old-format migration
internal/cli/       helper subcommands (status, settings, switch, change, gate, reopen, issues, …)
testdata/golden/    byte-identity fixtures (icm/manifest.json)
```

## Process

- intake → requirements → design → plan → implement → test → review → security → ship.
- `tomo init` scaffolds a project; `tomo update` refreshes its managed files;
  `tomo add` installs an app generator, scaffolds outside the workspace, and
  registers the result as the one target repo.

## Development

- `go build -o tomo ./cmd/tomo` — build the binary.
- `go test ./...` — run all tests.
- `go test ./internal/icm -update` — regenerate golden fixtures.
