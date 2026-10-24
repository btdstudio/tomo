---
description: Seed the .tomo workspace and base settings for this project
---

# Tomo init

Initialize this project for the SDD loop: create the `.tomo/` workspace and a
base `.tomo/settings.json`. Idempotent and safe — never overwrites an existing
settings file, never starts a phase. Run it before the first phase to pin the
project's settings explicitly and lay down the directories every command expects.

## Steps

1. If the `.tomo/` workspace does not exist, create it along with the artifact
   directory (`mkdir -p .tomo/specs`).
2. Compute effective settings (defaults ← global `~/.config/tomo/settings.json`
   ← project `.tomo/settings.json`). Run `tomo settings` (or `.tomo/bin/tomo
   settings`) when the binary is available; otherwise read the two JSON files
   directly (missing files count as `{}`) and merge: start from the defaults,
   overlay global, then project. The effective `gates` must be `block|prompt|
   auto`; if it is not, report the invalid value and still proceed with `block`.
3. If `.tomo/settings.json` does not exist, write it with the effective
   settings keys as explicit JSON — the defaults are `baseSource`, `gates`,
   `provider`, `artifactDir`, `issues`, `issuesDir`, `personas` — so the
   project file always reflects its own configuration. If it already exists,
   leave it untouched.
4. Append one line to `.tomo/log.md`: run `tomo log "workspace initialized
   (init)"` (or `.tomo/bin/tomo log ...`; if the binary is unavailable, append
   `- <ISO timestamp> workspace initialized (init)` directly).
5. Report the paths created and the effective settings, then suggest starting
   with `/tomo requirements [src]`.

This command only sets up the workspace. It does not touch `.tomo/state.json`
and does not write or change any phase; re-running it only confirms the
workspace already exists.

Arguments (may be empty): $ARGUMENTS
