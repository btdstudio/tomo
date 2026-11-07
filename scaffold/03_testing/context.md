# 03_testing — QA room

Owns the **test** phase after implementation. (Hybrid stage: tomo folds test
authoring into the coder and adequacy into the reviewer — this workspace keeps a
dedicated QA pass.)

## Persona
- `personas/qa.md` — writes/runs tests and checks them against requirements

## Input (read)
- target repo code (the code under test; see `tomo repo list`)
- `01_planning/specs/<n>-<slug>/requirements.md` (acceptance criteria)

## Output (write)
- tests alongside the app under the target repo (see `tomo repo list`)
- `01_planning/specs/<n>-<slug>/test-results.md` — run results & coverage

## Gate
Report pass/fail per acceptance criterion, then stop for approval. Failures loop
back to the coder with `tomo reopen implement --reason <r>`.
