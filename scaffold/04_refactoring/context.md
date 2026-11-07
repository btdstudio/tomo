# 04_refactoring — Tech Lead / Code Review + Security room

Owns the **review** and **security** passes after implementation.

## Personas
- `personas/reviewer.md` — correctness, scope vs design, test adequacy
- `personas/security.md` — vulnerability classes and security debt

## Input (read)
- target repo code (the change and its seams; see `tomo repo list`)
- `01_planning/specs/<n>-<slug>/*` (the design the code claims to implement)

## Output (write)
- `01_planning/specs/<n>-<slug>/code-review.md` — numbered findings
- `01_planning/specs/<n>-<slug>/security-review.md` — vulnerability findings

## Gate
Only **critical** / **high** findings loop back to the coder, with `tomo reopen
implement --reason <r>`. Skip security with `tomo gate security skipped --reason
<r>` when the change has no security surface. Read-only by default — report, don't
fix.
