---
description: Design the solution for the approved requirements
allowed-tools: Read(~/.config/tomo/**)
---

# Tomo design (phase 2)

Design the solution for the approved requirements. This phase does not
implement anything — it produces the map the plan phase decomposes.

## Persona

Resolve this step's persona with `tomo persona design` (run from the project
root; `.tomo/bin/tomo persona design` also works) and adopt it: read the
profile — `.tomo/personas/<id>.md` for a built-in, or the resolved file path
for a custom persona — and run the step as that persona. By default this is
the **architect**, who owns analysis, the design artifact, and resolving open
questions; an override in settings may replace it.

## Preconditions

Read `.tomo/state.json` (the active machine is the `changes[<change>]` record).
If `gates.requirements` is not `approved`, stop and
tell the user the requirements phase must be approved first (`/tomo
requirements` then `/tomo continue`). Otherwise read
`.tomo/specs/<change>/requirements.md` (the active change folder from
`.tomo/state.json`; if unset or its folder is gone, the newest numbered folder
under `.tomo/specs/`) — it defines the scope. Do not relax it without an
explicit conversation.

## Analyze the codebase

Survey the code relevant to the requirements: existing modules, seams,
libraries already in use, and native/stdlib capabilities that cover the work.
Prefer the smallest design that satisfies the acceptance criteria — note in
the design where the chosen approach trades away complexity. Mark anything
deliberately deferred.

## Interview

Resolve the open questions left in requirements (e.g. exact API endpoints,
formats) and confirm the key design decisions with the user. Record decisions
and any rejected alternatives in the design doc.

## Write the artifact

Write `.tomo/specs/<change>/design.md`. If `.tomo/templates/design.md` exists,
use it as the skeleton; otherwise use: Overview, Context, Design decisions
(with rationale), Architecture/components, Data flow/behavior, Interfaces, Out
of scope, Open items/risks, Notes. Cite `file:line` for integration points.
Use the same `<change>` folder that holds the requirements.

## Update state

Follow the `.tomo/state.json` contract:

- start the `design` phase and set its gate status to `draft`: run `tomo state
  set-gate design draft` (or `.tomo/bin/tomo state set-gate design draft`; if
  the binary is unavailable, edit `.tomo/state.json` directly)
- append one line to `.tomo/log.md` (`design → draft`): run `tomo log "design →
  draft"` (or `.tomo/bin/tomo log ...`)

Then follow the approval gate below.

# Approval gate

You are at an approval gate. The artifact you produced must be approved before
the next phase may begin. This is a prompted contract — stop and ask.

1. Present what you wrote (path + a short summary), not a wall of text.
2. Update the active change's record in `.tomo/state.json` (`changes[<change>]`)
   for this phase: status `await-approval` (run `tomo state set-gate <phase>
   await-approval`, or `.tomo/bin/tomo state set-gate <phase> await-approval`).
   If the binary is unavailable, edit `.tomo/state.json` directly.
3. Ask the user to approve or request changes. Do not proceed past the gate on
   your own.
4. On approval: set status `approved` (run `tomo state set-gate <phase>
   approved`), append one line to `.tomo/log.md` (artifact path, date,
   approved), then continue to the next action.
5. On requested changes: incorporate the feedback, rewrite the artifact, and
   repeat the gate from step 2.

`settings.gates`: `block` (default) — do not proceed without explicit approval;
`prompt` — ask once, proceed only on explicit yes; `auto` — treat a go-ahead or
the user's move to the next command as approval.

Arguments (may be empty): $ARGUMENTS
