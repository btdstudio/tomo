# Tomo plan (phase 3)

Break the approved design into dependency-ordered phases and tasks, and write
the plan as a single machine-readable `tasks.json` artifact.

## Persona

Resolve this step's persona with `.tomo/lib/personas.js` `resolvePersona(
'plan', settings)` (load effective settings with `.tomo/lib/settings.js` when
present) and adopt it: read the profile — `.tomo/personas/<id>.md` for a
built-in, or the resolved file path for a custom persona — and run the step as
that persona. By default this is the **architect**, who owns decomposing the
design into single-agent tasks; an override in settings may replace it.

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

Follow the `.tomo/state.json` contract (via `.tomo/lib/state.js` when
present):

- start the `plan` phase and set its gate status to `draft`, save state
- append one line to `.tomo/log.md`: `plan → draft`

Then follow the approval gate below.

<!-- tomo:include gate.md -->
