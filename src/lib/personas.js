'use strict';
// Tomo personas module. Owns the persona contract: the built-in persona ids,
// the default step→persona map, and runtime resolution of which persona runs a
// step (defaults overridden by settings.personas.map).
//
// Each step maps to a persona plus a usage mode:
//   lens   — interactive steps (requirements, design, plan, arch, trace, debt):
//            the running agent reads the persona profile and adopts it
//   spawn  — implement-loop steps: the loop invokes the persona as a subagent
//
// A settings override may name a built-in persona id, an alias from
// personas.custom (which maps alias → file path), or a raw project-local file
// path. Command bodies reference this contract via .tomo/lib/personas.js so the
// resolved persona is identical in every harness.

// The built-in personas; ids match the canonical files in src/personas/.
const PERSONAS = ['analyst', 'architect', 'coder', 'reviewer', 'security'];

// Which steps spawn the persona as a subagent vs adopt it in-session.
const SPAWN_STEPS = new Set(['implement', 'implement-review', 'implement-security']);

// Default step → persona binding. Settings overrides win per step.
const DEFAULT_PERSONA_MAP = {
  requirements: 'analyst',
  design: 'architect',
  plan: 'architect',
  implement: 'coder',
  'implement-review': 'reviewer',
  'implement-security': 'security',
  arch: 'architect',
  trace: 'reviewer',
  debt: 'architect',
};

function isBuiltIn(id) {
  return PERSONAS.includes(id);
}

function modeForStep(step) {
  return SPAWN_STEPS.has(step) ? 'spawn' : 'lens';
}

function neutralPath(id) {
  return `.tomo/personas/${id}.md`;
}

// Resolve which persona drives `step` under a merged settings object.
// Returns { id, mode, source, file }:
//   id     — persona id (or custom alias) to invoke/adopt
//   mode   — 'lens' | 'spawn'
//   source — 'default' | 'override' (a built-in) | 'custom' (a file)
//   file   — the persona profile to read/adopt (built-ins: .tomo/personas/)
// Custom resolution order: built-in id ⇒ alias in personas.custom ⇒ raw path.
function resolvePersona(step, settings) {
  const personas = (settings && settings.personas) || {};
  const map = personas.map || {};
  const custom = personas.custom || {};
  const value = map[step];
  if (value === undefined || value === null || value === '') {
    const id = DEFAULT_PERSONA_MAP[step] || '';
    return { id, mode: modeForStep(step), source: 'default', file: id ? neutralPath(id) : '' };
  }
  if (isBuiltIn(value)) {
    return { id: value, mode: modeForStep(step), source: 'override', file: neutralPath(value) };
  }
  const alias = custom[value];
  if (alias) {
    return { id: value, mode: modeForStep(step), source: 'custom', file: alias };
  }
  return { id: value, mode: modeForStep(step), source: 'custom', file: value };
}

module.exports = {
  PERSONAS,
  SPAWN_STEPS,
  DEFAULT_PERSONA_MAP,
  isBuiltIn,
  modeForStep,
  neutralPath,
  resolvePersona,
};