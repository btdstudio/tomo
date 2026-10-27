# 01_planning — Product Manager + Architect room

Owns the **requirements → design → plan** phases.

## Personas
- `personas/analyst.md` — requirements (what + how we know it's done)
- `personas/architect.md` — design (smallest solution) + plan (`tasks.json`)

## Input (read)
- `00_issues/<id>.md` — the base source / issue
- `templates/requirements.md`, `templates/design.md` — skeletons

## Output (write)
- `specs/<n>-<slug>/requirements.md`
- `specs/<n>-<slug>/design.md`
- `specs/<n>-<slug>/tasks.json`

## Gate
Stop after each artifact and hand control back for approval. Never start the
next phase until the current one is approved (see `.tomo/state.json` gates).
