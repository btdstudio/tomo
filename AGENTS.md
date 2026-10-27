# tomo — folder-driven AI SDLC toolset

This file applies to agents working on the tomo repo itself.

## Non-negotiable rules

- Go >= 1.22, standard library only — no third-party runtime or build deps.
  `go test ./...` must stay green from a clean checkout.
- The ICM scaffold is the embedded content store: `scaffold/`, embedded via
  `//go:embed all:scaffold` (`embed.go`). Edit the scaffold there; `tomo check`
  and the golden test (`internal/icm/golden_test.go`) pin it. After editing the
  scaffold, run `go test ./internal/icm -update` to regenerate the golden.
- `.tomo/` is per-project workspace state and is gitignored — never commit it.

## Layout

```
scaffold/           the embedded ICM template (copied out by `tomo init`)
embed.go            //go:embed all:scaffold
cmd/tomo/           the CLI dispatch (init, update, add, check, helpers)
internal/icm/       the scaffold/update/add/check engine
internal/state/     .tomo/state.json phase machine
internal/settings/  mergeable settings (global ← project)
internal/issues/    local issue store (frontmatter, sequential ids)
internal/version/   version reporting (build info)
internal/cli/       helper subcommands (status, settings, switch, issues, …)
testdata/golden/    byte-identity fixtures (icm/manifest.json)
```

## Process

- Requirements before design; design before plan; plan before implement.
- `tomo init` scaffolds a project; `tomo update` refreshes its managed files;
  `tomo add` installs apps from their external repos into `src/apps/`.

## Development

- `go build -o tomo ./cmd/tomo` — build the binary.
- `go test ./...` — run all tests.
- `go test ./internal/icm -update` — regenerate golden fixtures.
