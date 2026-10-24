---
description: Show Tomo's effective settings (global + project merged)
---

Show Tomo's effective settings for this project. Settings are mergeable JSON:
global `~/.config/tomo/settings.json` is the base, then the project file
`.tomo/settings.json` is layered on top — anything set in the project
supersedes the global value. Nothing is stored in the harness.

Read the sources and report the effective merge:

1. If the `tomo` binary is available (or `.tomo/bin/tomo`), load effective
   settings with `tomo settings` (defaults ← global ← project) and print the
   keys.
2. Otherwise, read `~/.config/tomo/settings.json` and `.tomo/settings.json`
   directly (missing files count as `{}`), overlay project onto global, and
   print:
   - `baseSource` — default base requirement source
     (`file:path | issue:N | url:...`), empty means you must supply one
   - `gates` — approval-gate policy: `block` (default, never proceed without
     explicit approval) | `prompt` (ask once, proceed only on explicit yes) |
     `auto` (a go-ahead or moving to the next command counts as approval)
   - `provider` — optional harness tag, informational
   - `artifactDir` — where artifacts are written (default `.tomo/specs`)
   - `issues` — issue backend: `github` (default, `issue:N` resolved via `gh`)
     | `local` (a markdown issue store in `issuesDir`)
   - `issuesDir` — where local issues live (default `issues`, relative to the
     project root)
   - `personas` — step→persona configuration: `map` maps a step
     (`requirements|design|plan|implement|implement-review|implement-security|
     arch|trace|debt`) to a built-in persona (`analyst|architect|coder|reviewer|
     security`), an alias, or a file path; `custom` maps aliases to file paths.
     If an applied step→persona is unknown, say so and note the step keeps the
     default.
3. If `.tomo/settings.json` is missing, say the project file is absent and this
   command is showing global + defaults only.

If the merged `gates` value is not `block|prompt|auto`, say the value is
invalid and that the effective policy falls back to the reading above. If the
merged `issues` value is not `github|local`, say it is invalid and that the
effective backend falls back to `github`.

This command is informational and read-only: it never writes state.

Arguments (may be empty): $ARGUMENTS
