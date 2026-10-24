# Tomo arch

## Persona

Resolve this step's persona with `tomo persona arch` (run from the project
root; `.tomo/bin/tomo persona arch` also works) and adopt it: read the profile
— `.tomo/personas/<id>.md` for a built-in, or the resolved file path for a
custom persona. By default this is the **architect**; an override in settings
may replace it.

Explore a codebase, surface architectural friction, and design improvements as
candidate interfaces. A **deep module** (John Ousterhout, "A Philosophy of
Software Design") has a small interface hiding a large implementation — it is
more testable, more AI-navigable, and testable at the boundary instead of
inside. Finish by filing each agreed design as a GitHub RFC issue that proposes
the deepened module.

## Process

### 1. Explore the codebase

Spawn parallel exploration subagents to navigate the codebase naturally. Do NOT
follow rigid heuristics — explore organically and note where you experience
friction:

- Where does understanding one concept require bouncing between many small files?
- Where are modules so shallow that the interface is nearly as complex as the implementation?
- Where have pure functions been extracted just for testability, but the real bugs hide in how they're called?
- Where do tightly-coupled modules create integration risk in the seams between them?
- Which parts of the codebase are untested, or hard to test?

The friction you encounter IS the signal.

### 2. Present candidates

Present a numbered list of deepening opportunities. For each candidate show:

- **Cluster**: which modules/concepts are involved
- **Why they're coupled**: shared types, call patterns, co-ownership of a concept
- **Dependency category**: see below
- **Test impact**: what existing tests would be replaced by boundary tests

Do NOT propose interfaces yet. Ask: "Which of these would you like to explore?"

### 3. User picks a candidate

### 4. Frame the problem space

Write a user-facing explanation of the chosen candidate: the constraints any new
interface must satisfy, the dependencies it would rely on, and a rough
illustrative code sketch to make the constraints concrete — not a proposal, just
grounding. Show it, then proceed immediately to step 5.

### 5. Design multiple interfaces

Spawn 3–4 parallel subagents. Give each a separate technical brief (file paths,
coupling details, dependency category, what is being hidden) and a different
design constraint:

- Agent 1: "Minimize the interface — aim for 1–3 entry points max"
- Agent 2: "Maximize flexibility — support many use cases and extension"
- Agent 3: "Optimize for the most common caller — make the default case trivial"
- Agent 4 (if applicable): "Design around ports & adapters for cross-boundary dependencies"

Each subagent outputs: (1) interface signature, (2) usage example, (3) what
complexity it hides internally, (4) dependency strategy, (5) trade-offs.

Present the designs sequentially, compare them in prose, then give your own
recommendation and any hybrid you'd propose. Be opinionated — the user wants a
strong read, not a menu.

### 6. User picks an interface

Once picked, write the RFC and file it as a GitHub issue.

## Dependency categories

Classify a candidate's dependencies to choose the strategy:

1. **In-process** — pure computation, in-memory state, no I/O. Always deepenable: merge the modules and test directly.
2. **Local-substitutable** — dependencies with a local test stand-in (e.g. PGLite, in-memory filesystem). Deepen and test against the stand-in.
3. **Remote but owned** — your own services across a network boundary. Define a port at the module boundary; the deep module owns the logic, the transport is injected. Test with an in-memory adapter, production with the real HTTP/gRPC/queue adapter.
4. **True external** — third-party services you don't control. Mock at the boundary; the deepened module takes the dependency as an injected port.

## Testing strategy

**Replace, don't layer.**

- Old unit tests on shallow modules are waste once boundary tests exist — delete them.
- Write new tests at the deepened module's interface boundary.
- Assert on observable outcomes through the public interface, not internal state.
- Tests must survive internal refactors — they describe behavior, not implementation.

## File the RFC

Check the effective `issues` mode (run `tomo settings`, or `.tomo/bin/tomo
settings`, when available; default `github`). Write the agreed design as an
RFC issue:

- `github` (default): file a GitHub issue (`gh issue create`; fall back to the
  web) titled as an arch RFC, with:
- `local`: create a local issue with `tomo issue new --title "arch RFC: <name>"
  --body "<RFC body>"` (or `.tomo/bin/tomo issue new ...`) and report the
  created `<issuesDir>/<id>.md` path.

Either way the RFC body carries:

- **Problem**: which modules are shallow/tightly coupled, the integration risk in the seams, why navigation and maintenance suffer.
- **Proposed interface**: signature, usage example, hidden complexity.
- **Dependency strategy**: which category applies and how dependencies are handled.
- **Testing strategy**: new boundary tests to write, old tests to delete, any stand-ins/adapters needed.
- **Implementation recommendations**: what the module owns, hides, exposes (not coupled to current file paths), and how callers migrate.

Stop and confirm with the user before creating each issue. This command never
writes code and does not touch `.tomo/` state.
