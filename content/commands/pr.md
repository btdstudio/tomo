# Tomo pr

Write a pull request body for the current branch and open the PR with the `gh`
CLI. Nothing is pushed or created without an explicit yes.

## Procedure

### 1. Inspect the branch

Run `git status`, `git log --oneline` against the base branch, and
`git diff <base>...HEAD` to understand every change the PR introduces. Determine
the base branch (default `main`; confirm if the repo uses another). Review all
commits in the PR, not just the latest.

### 2. Write the body

Use this template, keeping prose brief and skipping preambles:

```markdown
## Summary

<diagram, diff-sketch, or tree>

## Evidence

- **Before:** <screenshot/output/failing test run>
  **After:** <screenshot/output/passing test run>

## Merge Danger

**Door:** <one-way or two-way>

<optional: description>

**Blast Radius:** <one-word description>

<optional: potential ramifications of merge>
```

#### Summary

Pick the smallest view that makes the key point clear. Show logic or an
algorithm as pseudocode, runtime control flow as a call tree, UI structure as a
component tree, file responsibility as a shallow file tree, or interaction as
Mermaid. Use `diff` when the point is what changes and the surrounding shape
already exists. Show the whole block when most of it is new or when omitted
context would hide ownership. Place each visual next to the short text it
supports; keep only the calls, files, props, states, and boundaries needed to
answer the point. Use one or several of these, rarely all.

#### Evidence

Concrete evidence the change works, as a before and after. Screenshots are best
when the change is visual; execution-based evidence (test results, console
output) is next best. Show the exact test that now fails and passes.

#### Merge Danger

State whether the change is a one-way or two-way door: two-way doors can be
walked back, one-way doors (destructive or hard-to-reverse decisions) cannot.
The blast radius is the potential impact or scope of the change — consider all
possibilities (layout shift, breakage for consumers, etc.).

### 3. Propose and open

Show the title and body and stop for approval. On yes, run
`gh pr create --base <base> --title "<title>" --body "<body>"` (add
`--draft` if the user asks for a draft). Print the PR URL. On no or feedback,
incorporate it and re-propose.

## Rules

- Never push or create a PR without an explicit yes.
- Use the user's domain language; keep prose brief.
- This command does not touch `.tomo/` state.

The Summary visuals and their placement guidance are adapted from Dex Horthy's
`show-me` skill (humanlayer/skills).
