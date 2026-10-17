# Tomo trace

## Persona

Resolve this step's persona with `.tomo/lib/personas.js` `resolvePersona(
'trace', settings)` (load effective settings with `.tomo/lib/settings.js` when
present) and adopt it: read the profile — `.tomo/personas/<id>.md` for a
built-in, or the resolved file path for a custom persona — and run the step as
that persona. By default this is the **reviewer** (senior engineer tracing
architecture and data flow); an override in settings may replace it. Flag
security concerns for human review.

Investigate a problem or question by tracing the codebase with parallel
subagents, then synthesize the findings into a structured report.

## Inputs

`$ARGUMENTS` holds `<problem> [output-path]`:

- `<problem>` — description of the problem or question to investigate. If it is
  ambiguous or missing entirely, ask clarifying questions before proceeding.
- `<output-path>` — where to write the findings report. Defaults to
  `findings.md` in the active change folder (`.tomo/specs/<change>/` from
  `.tomo/state.json`; if `change` unset or its folder is gone, fall back to
  the newest numbered folder under `.tomo/specs/`, else `.tomo/specs/` itself).
  Create directories as needed.

## Process

### 1. Clarify

If the problem description is ambiguous, ask clarifying questions before
proceeding. Capture any hypotheses, expected behavior, and architecture notes to
pass to the explorers.

### 2. Explore

Spawn one parallel exploration subagent per starting location or area of the
codebase. Each gets: the problem description, the hypotheses/context, and
instructions to trace the relevant code path, identify root cause candidates,
and note what was ruled out.

Spawn subagents in parallel for independent areas. Use sequential exploration
where findings in one area inform what to look for in another.

### 3. Synthesize

Write `<output-path>` with these sections:

```markdown
## Per-Area Findings
### [Area]
- Key files and code paths examined
- What was found
- What was ruled out

## Root Cause Hypothesis
One clear statement of the most likely root cause, supported by evidence from
the exploration.

## Ruled-Out Alternatives
Other hypotheses considered and why they were eliminated.

## Security Concerns
Any security issues discovered during investigation (if applicable).
```

### 4. Hand back

Append one line to `.tomo/log.md` via `.tomo/lib/state.js` `appendLog` when
present (`trace complete`). Report that findings are complete at
`<output-path>` and hand control back to the caller. This command never writes
code and does not touch `.tomo/state.json`.
