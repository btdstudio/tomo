---
description: Restore Tomo session context from the .tomo workspace
---

# Tomo pickup

Restore the working context from the `.tomo/` workspace after a compact or a
fresh session.

1. Read `.tomo/state.json` (via `tomo state`, or `.tomo/bin/tomo state`, when
   available; otherwise read the file directly) and report: phase, status,
   per-phase gates (requirements/design/plan gates plus the `implement` run
   phase), sources, the active change folder (`change`).
2. Read `.tomo/SLIP.md` if present and report its summary: objective, key
   decisions, open questions.
3. Read the headers of the newest artifacts in the active change folder
   (`.tomo/specs/<change>/`; if `change` is unset or its folder is gone, the
   newest numbered folder under `.tomo/specs/`) — read the full artifact only
   when the task requires it. If the current phase is `await-approval`, read
   the full gated artifact so you can present the gate.
4. Read `.tomo/log.md` tail (last few lines) for the changeset trail.
5. State the exact next command, then stop. Do not run ahead:
   - `implement` is `in-progress`: `<change>` is mid-implementation —
     `/tomo implement` resumes at the first task not `done` in `tasks.json`.
   - `implement` is `complete`: the spec is done — nothing pending; suggest
     `/tomo commit` and `/tomo switch`.
   - otherwise: the command for the current draft/await-approval phase, or
     `/tomo implement` when all gated artifacts are approved.

If there is no `.tomo/` workspace, say so and suggest `/tomo requirements
[src]` to begin. This command reads only; it never writes state.

Arguments (may be empty): $ARGUMENTS
