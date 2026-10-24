---
description: Assess technical debt by category and file GitHub issues for findings
---

# Tomo debt

## Persona

Resolve this step's persona with `tomo persona debt` (run from the project
root; `.tomo/bin/tomo persona debt` also works) and adopt it: read the profile
— `.tomo/personas/<id>.md` for a built-in, or the resolved file path for a
custom persona. By default this is the **architect**; an override in settings
may replace it.

Assess this repository for technical debt as a principal engineer, write a
ranked findings report to `debt.md` in the active change folder
(`.tomo/specs/<change>/` from `.tomo/state.json`; if `change` unset or its
folder is gone, the newest numbered folder under `.tomo/specs/`, else
`.tomo/specs/` itself), then propose a GitHub issue for each qualifying
finding. Do not make code changes.

## Inputs

`$ARGUMENTS`, when present, is `<focus> [min-severity]`:

- `<focus>` — comma-separated categories to limit scope (see list below). If
  omitted, assess all categories.
- `<min-severity>` — `critical | high | medium | low` floor for findings.
  Defaults to `medium` (report and file at that level and above).

## Scope

Assess these categories (or only `<focus>` if given):

1. architecture boundaries
2. coupling and cohesion
3. duplicated logic
4. complexity hotspots
5. test debt
6. dependency/runtime debt
7. security-sensitive design debt
8. operational/observability debt
9. migration/schema debt
10. areas where future changes are likely to be expensive

Use `git log --format="%H %s" -- <file>` to identify churn hotspots — frequently
changed files carry higher maintenance cost and warrant closer scrutiny.

## Severity filter

Map `<min-severity>` to the threshold and discard findings below it before
writing up or proposing issues: `critical` → critical only; `high` → critical +
high; `medium` → critical + high + medium; `low` → all findings.

## For each qualifying finding

Provide:

- title
- category
- severity (critical / high / medium / low)
- confidence (high / medium / low)
- exact file paths and line references
- why this is technical debt
- likely impact
- remediation approach
- smallest safe first PR
- estimated effort
- risks of remediation

Exclude stylistic nits unless they create real maintenance risk. Rank findings
by engineering ROI, not aesthetic preference.

## Write the report

Write the ranked findings to `debt.md` in the active change folder (as above,
create directories as needed) with the above fields per finding, ordered by
ROI. Append one line to `.tomo/log.md` (`debt assessment complete`): run `tomo
log "debt assessment complete"` (or `.tomo/bin/tomo log ...`).

## File the issues

For each qualifying finding, stop and confirm. Then, per effective `issues`
mode (run `tomo settings`, or `.tomo/bin/tomo settings`, when available;
default `github`):

- `github` (default): create a GitHub issue (`gh issue create`; fall back to
  the web) with the finding's title and full details.
- `local`: create a local issue with `tomo issue new --title "<finding title>"
  --body "<full details>"` (or `.tomo/bin/tomo issue new ...`) and report the
  created `<issuesDir>/<id>.md` path.

Ask before creating each issue; do not batch-file issues the user has not
seen. This command never writes code and does not touch `.tomo/state.json`.
