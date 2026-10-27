# 04_refactoring — Tech Lead / Code Review + Security room

Owns the **review** and **security** passes after implementation.

## Personas
- `personas/reviewer.md` — correctness, scope vs design, test adequacy
- `personas/security.md` — vulnerability classes and security debt

## Input (read)
- `src/apps/*` (the change and its seams)
- `01_planning/specs/<n>-<slug>/*` (the design the code claims to implement)

## Output (write)
- `reviews/<feature>_code_review.md` — numbered findings
- `reviews/<feature>_security_review.md` — vulnerability findings

## Gate
Only **critical** / **high** findings loop back to the coder. Read-only by
default — report, don't fix.
