#!/usr/bin/env node
// Claude Code SessionStart hook: emit a one-line tomo resume anchor when the
// project has a .tomo/ workspace, so every session starts oriented. Silent
// otherwise. Zero dependencies; Node >= 20.
//
// The active machine lives in state.json `changes[<change>]`; legacy files may
// still carry it at top level — both are handled here.

const fs = require('fs');
const path = require('path');

const STATE = path.join(process.cwd(), '.tomo', 'state.json');

try {
  const state = JSON.parse(fs.readFileSync(STATE, 'utf8'));
  const rec =
    state.change && state.changes && state.changes[state.change]
      ? state.changes[state.change]
      : state;
  const gates = rec.gates || {};
  const phase = rec.phase || 'no phase';
  const status = rec.status || '';
  const gatedOpen = ['requirements', 'design', 'plan'].filter((g) => gates[g] !== 'approved');
  let resume;
  if (gatedOpen.length > 0) {
    resume = `next gate: ${gatedOpen[0]} (${gates[gatedOpen[0]] || 'not started'})`;
  } else if (gates.implement === 'complete') {
    resume = 'spec complete — /tomo commit or /tomo switch';
  } else if (gates.implement === 'in-progress') {
    resume = 'implement in-progress — /tomo implement resumes';
  } else {
    resume = 'all gates approved — /tomo implement to run the plan';
  }
  console.log(`Tomo: ${phase} (${status}) — ${resume}`);
} catch (e) {
  // No .tomo/ workspace; stay silent.
}