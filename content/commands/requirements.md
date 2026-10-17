# Tomo requirements (phase 1)

Run the requirements phase end to end: resolve the base source, ask clarifying
questions, and write `.tomo/specs/<change>/requirements.md`. This phase is the
input to design — do not design anything yet.

## Persona

Resolve this step's persona with `.tomo/lib/personas.js` `resolvePersona(
'requirements', settings)` (load effective settings with `.tomo/lib/settings.js`
when present) and adopt it: read the profile — `.tomo/personas/<id>.md` for a
built-in, or the resolved file path for a custom persona — and run the step as
that persona. By default this is the **analyst**, who owns the interview and
the artifact; an override in settings may replace it.

## Resolve the base source

`$ARGUMENTS`, when present, is one of:

- `file:<path>` or a bare `<path>` — read the local file or PRD
- `issue:<N>` — fetch issue N per the effective issue mode: with `issues:
  github` (default) run `gh issue view N` (fall back to the web); with
  `issues: local` read `<issuesDir>/<N>.md` from the local issue store
  (`issuesDir` defaults to `issues`)
- `url:<...>` — fetch the URL

When `$ARGUMENTS` is empty, use `baseSource` from `.tomo/settings.json`
(load effective settings with `.tomo/lib/settings.js` when that file exists).
If nothing resolves, stop and ask for a source — never fabricate one.

## Clarify

Read the source, then ask focused clarifying questions until the scope is
unambiguous: goal, audience, boundaries, priorities, constraints, and what
"done" looks like. Keep the interview tight — stop as soon as the requirements
are unambiguous. Note each question and its resolution in the artifact.

## Start the change

Every piece of work gets its own numbered folder under `.tomo/specs/`. Read
`.tomo/state.json`. If it has a `change` whose folder still exists, keep it
(this also covers feedback iterations — never renumber mid-change). Otherwise
start a new one:

- compute the next id with `.tomo/lib/state.js` `nextChangeId` when present
  (highest numeric `<id>-` folder prefix under `.tomo/specs/` + 1, else 1)
- pick a short kebab-case slug for the work (`.tomo/lib/state.js` `slugify`
  when present)
- create `.tomo/specs/<id>-<slug>/` and set `change: "<id>-<slug>"` in
  `.tomo/state.json`

## Write the artifact

Write `.tomo/specs/<change>/requirements.md` (create directories as needed). If
`.tomo/templates/requirements.md` exists, use it as the skeleton; otherwise
use this structure: Problem/Goal, Scope (in scope + explicit out of scope),
Functional requirements (must/should/could), Non-functional requirements,
Acceptance criteria, Clarifying questions & resolutions, Open questions, Notes.
Record the resolved base sources in the header.

## Update state

Follow the `.tomo/state.json` contract (via `.tomo/lib/state.js` when present):

- ensure the `.tomo/` workspace exists
- start the `requirements` phase and set its gate status to `draft`, save state
  (including the `change` folder from above)
- append one line to `.tomo/log.md`: `requirements → draft`

Then follow the approval gate below.

<!-- tomo:include gate.md -->
