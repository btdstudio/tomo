---
description: Write .tomo/SLIP.md session context for a compact or resume
---

# Tomo handoff

Preserve the current session so the context can be compacted or closed and
picked up later without losing state.

1. Read `.tomo/state.json` (via `tomo state`, or `.tomo/bin/tomo state`, when
   available; otherwise read the file directly) and the artifact headers in the
   active change folder (`.tomo/specs/<change>/`; if `change` is unset or its
   folder is gone, the newest numbered folder under `.tomo/specs/`).
2. Write `.tomo/SLIP.md` capturing:
   - project and objective
   - phase machine: phase, status, per-phase gates, sources, active change
   - artifacts present (`requirements.md`, `design.md`, `tasks.json`)
   - key decisions and open questions (from the newest artifact)
   - the exact next command the picking-up session should run
   - any files the next session must read first
   Keep it tight — this file is the resume anchor, not a transcript.
3. Append one line to `.tomo/log.md` (`handoff written`): run `tomo log
   "handoff written"` (or `.tomo/bin/tomo log ...`).

Then tell the user the context is preserved at `.tomo/SLIP.md` and that they
can resume with `/tomo pickup` (or compact the session safely). This command
does not change gate status.

Arguments (may be empty): $ARGUMENTS
