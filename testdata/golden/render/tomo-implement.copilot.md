---
name: tomo-implement
description: Run the approved plan to completion, then record the spec as complete
---

The user invoked this skill as `/tomo-implement`; text typed after the command name is its arguments. Treat that text as `$ARGUMENTS` in the body below (absent text means no arguments were provided).

# Tomo implement (phase 4)

Drive the approved plan's tasks to done and record the change as complete. The
plan gate already approved the work — no new approval applies here; this is a
run phase, not a gate.

## Preconditions

Read `.tomo/state.json` (the active machine is the `changes[<change>]` record).
Stop and say exactly what is missing if any of
`gates.requirements/design/plan` is not `approved` (`/tomo requirements`,
`/tomo design`, or `/tomo plan` first). If `gates.implement` is `complete`,
report the spec is done and suggest `/tomo commit` or `/tomo switch` — do not
run again.

Resolve the active change folder: `change` from `.tomo/state.json`, or, if
unset or its folder is gone, the newest numbered folder under `.tomo/specs/`.
Read that folder's `tasks.json` (authoritative machine plan). If `tasks.json`
is missing, stop: `/tomo plan` must run first.

## Start

If `gates.implement` is empty, set the `implement` phase to `in-progress` (run
`tomo state set-gate implement in-progress`, or `.tomo/bin/tomo state set-gate
implement in-progress`; if the binary is unavailable, edit `.tomo/state.json`
directly) and append one line to `.tomo/log.md` (`implement → in-progress`).
If it is already `in-progress` (or `complete` is handled above), resume —
`tasks.json` statuses are the source of truth for what remains.

## Loop

Iterate the plan in `phases` → `tasks` order until every task is `done`
(repeats the pass if a pass makes progress but leaves work):

1. Pick the first phase with a task not `done`. Within it, pick the first task
   whose `status` is not `done` and whose `dependencies` are all `done`. If a
   task exists but no task can start (dependency cycle), stop and report the
   cycle — do not guess.
2. Mark the task `in-progress` in `tasks.json` and rewrite the whole file (the
   same write you make after every transition; persist every change).
3. Implement the task: hand the task description + `files` + the change's
   `design.md` + the active `src/` seams (existing modules, conventions from
   `AGENTS.md`) to a subagent running the **`coder`** persona — the harness
   agent/subagent named `coder` installed alongside this tool (resolve with
   `tomo persona implement`, or `.tomo/bin/tomo persona implement`, when
   available). Verify the subagent touched the named files; if any were missed,
   have it finish the task.
4. Mark the task `done` in `tasks.json` and persist.

When a phase's tasks are all `done` and the phase does not carry
`"reviewed": true`:

5. Run the **`reviewer`** persona subagent (resolve with `tomo persona
   implement-review` when available) over that phase's changes
   (correctness, scope vs. the design, tests). If it raises `critical` or
   `high` findings, reset the affected tasks to `pending` and go back to step 1
   (re-drive). Cap re-review of one phase at 2 cycles, then report to the user
   and stop. Otherwise set the phase's `"reviewed": true` in `tasks.json` and
   persist.

When a phase is `reviewed: true` but does not carry `"securityReviewed": true`:

6. Run a dedicated **`security`** persona subagent (resolve with `tomo persona
   implement-security` when available) over that phase's changes —
   vulnerabilities, input handling, authn/authz, secrets, dependency risk. If
   it raises `critical` or `high` findings, reset the affected tasks to
   `pending` and go back to step 1. Cap a phase's security pass at 2 cycles,
   then report to the user and stop. Otherwise set the phase's
   `"securityReviewed": true` in `tasks.json` and persist.

Resume behavior: earlier `done` tasks and phases that are both `reviewed` and
`securityReviewed` are skipped by construction, so a fresh session or a compact
in the middle of the loop picks up exactly where the plan left off. `/tomo
status` shows `implement → in-progress` while running.

## Verify and complete

When every task is `done` and every phase carries both `"reviewed": true` and
`"securityReviewed": true`:

- Run the project's test command — the build tool's test script (e.g. `go test
  ./...` for a Go project, `npm test` for a Node project). If it fails, stop
  and report the failure; do NOT mark the spec complete.
- On success: set the `implement` phase to `complete` (run `tomo state
  set-gate implement complete`, or `.tomo/bin/tomo state set-gate implement
  complete`) and append one line to `.tomo/log.md` (`implement complete`).

## Report

Summarize: tasks done per phase, the test command run and its result, files
touched. State that the change is complete and hand off to `/tomo commit` for
the reviewed micro commits (README step 10). Mention `/tomo status` now reports
the spec as complete. This command does not commit, push, or open PRs on its
own.

Arguments (may be empty): $ARGUMENTS
