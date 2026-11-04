# 02_implementation — Software Engineer room

Owns the **implement** phase.

## Persona
- `personas/coder.md` — implements approved tasks, one at a time, with tests

## Input (read)
- `01_planning/specs/<n>-<slug>/tasks.json` — the approved plan
- `01_planning/specs/<n>-<slug>/design.md` — the approved design
- relevant files under the app root (see `.tomo/settings.json` → `outputDir` + `projectType`)

## Output (write)
- code under the app root (see `.tomo/settings.json` → `outputDir` + `projectType`), touching exactly the files each task names
- tests alongside the change

## Gate
Report task-by-task results (files changed, tests run). Do not commit or push;
version control is handled by a later review/commit step.
