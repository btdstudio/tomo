---
description: Resume Tomo's current phase after an approval or requested changes
allowed-tools: Read(~/.config/tomo/**)
---

# Tomo continue

Resume the phase machine at the current gate. Read `.tomo/state.json` (the
active machine is the `changes[<change>]` record) and `.tomo/log.md` first.

## Behavior

- No state file or empty phase: say nothing is in progress; next is
  `/tomo requirements [src]`.
- Phase status `draft`: the phase command has not presented its gate yet —
  tell the user to re-run that phase's command.
- Phase status `await-approval`: ask the user to approve or request changes.
  - **Approve** → set the phase gate status to `approved` (run `tomo state
    set-gate <phase> approved`, or `.tomo/bin/tomo state set-gate <phase>
    approved`; if the binary is unavailable, edit `.tomo/state.json` directly),
    append one line to `.tomo/log.md` (`<phase> approved`), then tell the user
    the next phase and its command (requirements → design → plan → implement).
  - **Request changes** → incorporate the feedback into the artifact
    (`.tomo/specs/<change>/requirements.md`, `.tomo/specs/<change>/design.md`,
    or `.tomo/specs/<change>/tasks.json` per phase, in the active change
    folder), keep the gate at `await-approval`, append a log line, and ask for
    approval again. Loop until approved.
- Phase status `approved`: say the phase is approved and state the next phase.
  If `plan` is approved and all gates are approved, the next phase is
  `implement` — tell the user to run `/tomo implement`.
- Implement phase `in-progress`/`complete`: the phase machine is in the run
  phase — `/tomo implement` resumes a paused loop; a `complete` spec has
  nothing pending (suggest `/tomo commit` or `/tomo switch`).

Honor `settings.gates` (block | prompt | auto) as documented in `/tomo
settings`. This command never invents requirements: to skip ahead, the user
must explicitly approve the gate.

Arguments (may be empty): $ARGUMENTS
