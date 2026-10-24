---
description: Break the approved design into phases and tasks
---

# Tomo plan (phase 3)

Break the approved design into dependency-ordered phases and tasks, and write
the plan as a single machine-readable `tasks.json` artifact.

## Persona

Resolve this step's persona with `tomo persona plan` (run from the project
root; `.tomo/bin/tomo persona plan` also works) and adopt it: read the profile
— `.tomo/personas/<id>.md` for a built-in, or the resolved file path for a
custom persona — and run the step as that persona. By default this is the
**architect**, who owns decomposing the design into single-agent tasks; an
override in settings may replace it.

## Preconditions

Read `.tomo/state.json` (the active machine is the `changes[<change>]` record).
If `gates.design` is not `approved`, stop and tell the
user the design phase must be approved first (`/tomo design` then
`/tomo continue`). Otherwise resolve the active change folder (from
`.tomo/state.json` `change`; if unset or its folder is gone, the newest
numbered folder under `.tomo/specs/`) and read its `requirements.md` and
`design.md` — the design is the source; the requirements set scope.

## Decompose

Split the design into phases (grouped, dependency-ordered) and tasks small
enough for a single agent to complete. Give every task a dot-notation ID
(`<phase>.<task>`, starting at 1) and declare dependencies as IDs that must be
complete first. Each task must name the `files` it touches.

## Write the artifacts

- `.tomo/specs/<change>/tasks.json` (the plan):
  ```json
  {
    "specName": "<project/feature>",
    "generatedAt": "<ISO 8601>",
    "phases": [{
      "id": 1,
      "name": "<phase name>",
      "reviewed": false,
      "securityReviewed": false,
      "tasks": [{
        "id": "1.1",
        "description": "<one actionable line>",
        "files": ["path/to/file"],
        "dependencies": ["1.0"],
        "status": "pending"
      }]
    }]
  }
  ```
  A task's `status` starts `pending`; `/tomo implement` moves it to
  `in-progress` then `done`. A phase's `reviewed` and `securityReviewed` start
  `false` and become `true` as the implement loop clears each pass. IDs use dot
  notation.

## Update state

Follow the `.tomo/state.json` contract:

- start the `plan` phase and set its gate status to `draft`: run `tomo state
  set-gate plan draft` (or `.tomo/bin/tomo state set-gate plan draft`; if the
  binary is unavailable, edit `.tomo/state.json` directly)
- append one line to `.tomo/log.md` (`plan → draft`): run `tomo log "plan →
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
