---
description: Show Tomo's current phase, state, and next action
---

Read `.tomo/state.json` if it exists (the active machine is the
`changes[<change>]` record) and report, compactly: current phase, status,
active gates, sources, and the active change folder. Confirm which artifacts
exist in that folder under `.tomo/specs/<n>-<slug>/`
(`requirements.md`, `design.md`, `tasks.json`).
Peek at the tail of `.tomo/log.md` for the recent trail.

## Personas

Load effective settings with `tomo settings` (or `.tomo/bin/tomo settings`)
when available, and resolve each step with `tomo persona <step>` for
`requirements`, `design`, `plan`, `implement`, `implement-review`,
`implement-security`, `arch`, `trace`, and `debt`. Report the binding in one
line, e.g.
`personas: requirements→analyst, design→architect, plan→architect,
implement→coder, implement-review→reviewer, implement-security→security,
arch→architect, trace→reviewer, debt→architect`. Mark every step whose persona
is overridden in settings with an asterisk (`plan→security*`). If a resolved
persona id is not a built-in, echo the resolved file path instead.

Then state the next action per the phase machine:

- No `.tomo/state.json`: nothing started. Next is `/tomo requirements [src]`.
- requirements is `draft` or `await-approval`: next is resolving feedback, then
  `/tomo continue`.
- design is `draft` or `await-approval`: next is iterating `/tomo design`, then
  `/tomo continue`.
- plan is `draft` or `await-approval`: next is iterating `/tomo plan`, then
  `/tomo continue`.
- All gated artifacts approved, implement `in-progress`: the loop is running or
  paused — next is `/tomo implement` to resume.
- All gated artifacts approved, implement empty: next is `/tomo implement` to
  run the plan. If `.tomo/SLIP.md` exists and context feels stale, `/tomo
  pickup` restores the full picture.
- implement `complete`: the spec is done — no pending phase. Suggest `/tomo
  commit` for the reviewed micro commits; to start or resume a different spec,
  `/tomo switch`.
- Any other state (e.g. a gated phase `approved` with later gates empty but the
  machine mid-flight on `continue`): say the next unapproved gate's command.

Keep the reply to a few lines: phase, status, artifacts present, next command.
This command reads only; it never writes state.

Arguments (may be empty): $ARGUMENTS
