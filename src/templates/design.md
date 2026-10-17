# Design — <short name>

- **Status:** draft | await-approval | approved
- **Date:** <ISO date>
- **Designing for:** <link to the active change's requirements: `.tomo/specs/<id>-<slug>/requirements.md`>

## Overview

<Three to five sentences: the shape of the solution and why it's the right one.>

## Context

<Current state of the codebase relevant to this change: existing modules,
seams, data flow, conventions to follow or break. Cite `file:line` where
useful.>

## Design decisions

- D1. <decision> — <rationale> (<alternatives rejected in brief>)
- D2. ...

## Architecture / components

<Components, their responsibilities, and how they connect. Keep it as the
implementer's map.>

## Data flow / behavior

<Step-by-step behavior for the main flows, aligned to the acceptance criteria
in requirements.>

## Interfaces

<Module boundaries, function/CLI shapes, config schema, env vars. Nothing
else is public.>

## Out of scope (deferred to later phases)

- <what design deliberately leaves out>

## Open items / risks

- <assumptions, API endpoints to confirm, risks with their impact>

## Notes

<references, constraints, implementation hints for the later implementation
phase.>