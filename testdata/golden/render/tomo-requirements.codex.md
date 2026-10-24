---
name: tomo-requirements
description: Write the requirements doc for the base source, asking clarifying questions
---

The user invoked this skill as `/tomo-requirements`; text typed after the command name is its arguments. Treat that text as `$ARGUMENTS` in the body below (absent text means no arguments were provided).

# Tomo requirements (phase 1)

Run the requirements phase end to end: resolve the base source, ask clarifying
questions, and write `.tomo/specs/<change>/requirements.md`. This phase is the
input to design — do not design anything yet.

## Persona

Resolve this step's persona with `tomo persona requirements` (run from the
project root; `.tomo/bin/tomo persona requirements` also works) and adopt it:
read the profile — `.tomo/personas/<id>.md` for a built-in, or the resolved
file path for a custom persona — and run the step as that persona. By default
this is the **analyst**, who owns the interview and the artifact; an override
in settings may replace it.

## Resolve the base source

`$ARGUMENTS`, when present, is one of:

- `file:<path>` or a bare `<path>` — read the local file or PRD
- `issue:<N>` — fetch issue N per the effective issue mode: with `issues:
  github` (default) run `gh issue view N` (fall back to the web); with
  `issues: local` read `<issuesDir>/<N>.md` from the local issue store
  (`issuesDir` defaults to `issues`)
- `url:<...>` — fetch the URL

When `$ARGUMENTS` is empty, use `baseSource` from `.tomo/settings.json`
(run `tomo settings` to load effective settings when available). If nothing
resolves, stop and ask for a source — never fabricate one.

## Clarify

Read the source, then ask focused clarifying questions until the scope is
unambiguous: goal, audience, boundaries, priorities, constraints, and what
"done" looks like. Keep the interview tight — stop as soon as the requirements
are unambiguous. Note each question and its resolution in the artifact.

## Start the change

Every piece of work gets its own numbered folder under `.tomo/specs/`. Read
`.tomo/state.json`. If it has a `change` whose folder still exists, keep it
(this also covers feedback iterations — never renumber mid-change). Otherwise
start a new one with `tomo state start <slug>` (or `.tomo/bin/tomo state start
<slug>`), which computes the next id, kebab-cases the slug, creates
`.tomo/specs/<id>-<slug>/`, and sets `change` in `.tomo/state.json`. If the
binary is unavailable, do it manually: compute the next id (highest numeric
`<id>-` folder prefix under `.tomo/specs/` + 1, else 1), pick a short
kebab-case slug, create `.tomo/specs/<id>-<slug>/`, and set
`change: "<id>-<slug>"` in `.tomo/state.json`.

## Write the artifact

Write `.tomo/specs/<change>/requirements.md` (create directories as needed). If
`.tomo/templates/requirements.md` exists, use it as the skeleton; otherwise
use this structure: Problem/Goal, Scope (in scope + explicit out of scope),
Functional requirements (must/should/could), Non-functional requirements,
Acceptance criteria, Clarifying questions & resolutions, Open questions, Notes.
Record the resolved base sources in the header.

## Update state

Follow the `.tomo/state.json` contract:

- ensure the `.tomo/` workspace exists
- start the `requirements` phase and set its gate status to `draft`: run `tomo
  state set-gate requirements draft` (or `.tomo/bin/tomo state set-gate
  requirements draft`; if the binary is unavailable, edit `.tomo/state.json`
  directly) — this also records the `change` folder from above
- append one line to `.tomo/log.md` (`requirements → draft`): run `tomo log
  "requirements → draft"` (or `.tomo/bin/tomo log ...`)

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
