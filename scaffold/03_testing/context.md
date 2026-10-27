# 03_testing — QA room

Owns the **testing** pass after implementation. (Hybrid stage: tomo folds test
authoring into the coder and adequacy into the reviewer — this workspace keeps a
dedicated QA pass.)

## Persona
- `personas/qa.md` — writes/runs tests and checks them against requirements

## Input (read)
- `src/apps/*` (the code under test)
- `01_planning/specs/<n>-<slug>/requirements.md` (acceptance criteria)

## Output (write)
- tests alongside the app under `src/apps/`
- `reports/<feature>_test_results.md` — run results & coverage

## Gate
Report pass/fail per acceptance criterion. Failures loop back to the coder.
