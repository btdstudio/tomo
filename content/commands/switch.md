# Tomo switch

Point the workspace at a different per-change folder
(`.tomo/specs/<id>-<slug>/`) and resume that change's phase machine. Use it to
pause one spec's process and pick up another — each change keeps its own
phase, gates, and sources in `.tomo/state.json` `changes`.

## List available specs

List the numbered `<id>-<slug>` folders under `.tomo/specs/` with `tomo state
changes` (or `.tomo/bin/tomo state changes`; if the binary is unavailable,
scan the directory and read each folder's `phase`/`status` from
`.tomo/state.json`). For each print the folder name and its recorded
phase/status, marking the active `change`. If `$ARGUMENTS` is empty or does not
match exactly one folder, list the specs and ask which to switch to. Never
guess a target; if the named folder does not exist, say so and list what does.

## Switch

On a confirmed `<id>-<slug>` target:

1. Run `tomo state switch <id>-<slug>` (or `.tomo/bin/tomo state switch
   <id>-<slug>`), which validates the folder and persists the active change;
   if the binary is unavailable, set `change` in `.tomo/state.json` yourself
   and restore that change's phase/status/gates/sources from its `changes`
   record.
2. Append one line to `.tomo/log.md` (`switch → <id>-<slug>`): run `tomo log
   "switch → <id>-<slug>"` (or `.tomo/bin/tomo log ...`).
3. Report the resumed phase machine: phase, status, gates. If the target has
   no recorded progress (a fresh folder), say so and set the next step to
   `/tomo requirements [src]`.

Switching never deletes or rewrites other changes' folders or records — the
previously active change keeps its progress in `.tomo/state.json`. This
command changes the active `change` (and thus which artifacts the phase
commands read/write) but never edits spec artifacts themselves.
