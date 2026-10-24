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

<!-- tomo:include gate.md -->
