'use strict';
// Tomo shared state module. Owns the .tomo/ workspace contract: state.json
// phase machine, log.md changeset appends, SLIP.md handoff, and specs dir.
//
// Command bodies reference this module's contract (paths, schema, transitions)
// so the SDD loop behaves identically in every harness. Installed copies land
// in .tomo/lib/ alongside the commands; tests pin this exact behavior.
//
// Layout:
//   .tomo/state.json   phase machine: { change, updatedAt, changes } where
//                      changes maps each change folder to its own
//                      { phase, status, gates, sources } record — the single
//                      store, so swapping specs preserves every change's
//                      progress. The active machine is changes[change];
//                      loadState resolves it into { phase, status, gates,
//                      sources, change, updatedAt }. Legacy files that still
//                      carry the machine at top level keep loading.
//   .tomo/specs/       per-change folders: <id>-<slug>/requirements.md,
//                      design.md, tasks.json, plus ad-hoc outputs (findings,
//                      debt, RFCs) folded into the active change
//   .tomo/log.md       one timestamped line per state change / gate
//   .tomo/SLIP.md      handoff context for compact/resume

const fs = require('fs');
const path = require('path');

// The gated SDD phases in order, then the run phases. Later phases
// (documentation) may be added without breaking the schema.
const PHASES = ['requirements', 'design', 'plan', 'implement'];

// Per-phase gate status. '' = not started. Gated phases only.
const GATE_STATES = ['', 'draft', 'await-approval', 'approved'];

// Run-phase status for `implement` (driven by /tomo implement, not a gate).
// '' = not started, 'in-progress' = loop running or resumable, 'complete' = done.
const IMPLEMENT_STATES = ['', 'in-progress', 'complete'];

function workspace(root) {
  return path.join(root, '.tomo');
}

function statePath(root) {
  return path.join(workspace(root), 'state.json');
}

function specsDir(root) {
  return path.join(workspace(root), 'specs');
}

// Path to one change folder: specs/<id>-<slug>/. Empty `change` resolves to the
// specs base itself (used as a guard for ad-hoc outputs when nothing is active).
function changeDir(root, change) {
  return change ? path.join(specsDir(root), change) : specsDir(root);
}

function logPath(root) {
  return path.join(workspace(root), 'log.md');
}

function slipPath(root) {
  return path.join(workspace(root), 'SLIP.md');
}

// Artifact inside a specific change folder.
function artifactPath(root, change, name) {
  return path.join(changeDir(root, change), name);
}

// Next sequential change id: highest numeric `<id>-` prefix under specs + 1.
function nextChangeId(root) {
  if (!fs.existsSync(specsDir(root))) return 1;
  let max = 0;
  for (const entry of fs.readdirSync(specsDir(root))) {
    const m = entry.match(/^(\d+)-/);
    if (m) max = Math.max(max, Number(m[1]));
  }
  return max + 1;
}

// Short kebab-case slug so change folders read as <id>-<slug>.
function slugify(text) {
  return String(text || '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 40);
}

function defaultState() {
  const gates = {};
  for (const phase of PHASES) gates[phase] = '';
  return { phase: '', status: '', gates, sources: [], change: '', updatedAt: '' };
}

function loadState(root) {
  const p = statePath(root);
  if (!fs.existsSync(p)) return defaultState();
  let parsed;
  try {
    parsed = JSON.parse(fs.readFileSync(p, 'utf8'));
  } catch (e) {
    throw new Error(`tomo: cannot parse ${p}: ${e.message}`);
  }
  const changes = parsed && parsed.changes && typeof parsed.changes === 'object' ? parsed.changes : {};
  const st = defaultState();
  if (parsed && typeof parsed === 'object') Object.assign(st, parsed);
  // The active change's record under `changes` is the source of truth
  // (empty `change` resolves to the `changes[""]` record).
  const rec = changes[parsed && parsed.change ? parsed.change : ''];
  if (rec && typeof rec === 'object') {
    if (rec.phase) st.phase = rec.phase;
    if (rec.status) st.status = rec.status;
    if (rec.gates && typeof rec.gates === 'object') Object.assign(st.gates, rec.gates);
    if (Array.isArray(rec.sources)) st.sources = rec.sources.slice();
  }
  const gates = st.gates || {};
  for (const phase of PHASES) if (!(phase in gates)) gates[phase] = '';
  st.gates = gates;
  return st;
}

function saveState(root, state) {
  state.updatedAt = new Date().toISOString();
  fs.mkdirSync(workspace(root), { recursive: true });
  const p = statePath(root);
  const changes = readChanges(root);
  const record = {
    phase: state.phase || '',
    status: state.status || '',
    gates: state.gates || {},
    sources: state.sources || [],
  };
  changes[state.change || ''] = record;
  fs.writeFileSync(
    p,
    JSON.stringify({ change: state.change || '', updatedAt: state.updatedAt, changes }, null, 2) + '\n',
  );
}

// The per-change phase-machine records in state.json (empty when absent).
function readChanges(root) {
  const p = statePath(root);
  if (!fs.existsSync(p)) return {};
  try {
    const parsed = JSON.parse(fs.readFileSync(p, 'utf8'));
    if (parsed && parsed.changes && typeof parsed.changes === 'object') return parsed.changes;
  } catch (e) {
    // unreadable/missing map — treat as empty
  }
  return {};
}

// Change folders under .tomo/specs/, annotated with any recorded progress.
function knownChanges(root) {
  const changes = readChanges(root);
  const out = [];
  const sd = specsDir(root);
  if (fs.existsSync(sd)) {
    for (const name of fs.readdirSync(sd)) {
      const m = name.match(/^(\d+)-/);
      if (!m) continue;
      const rec = changes[name] || {};
      out.push({
        name,
        id: Number(m[1]),
        phase: rec.phase || '',
        status: rec.status || '',
        gates: rec.gates || {},
      });
    }
  }
  out.sort((a, b) => a.id - b.id);
  return out;
}

// Point the workspace at a different change folder and resume its phase
// machine. The target folder must exist under .tomo/specs/. Saves state.
function switchTo(root, name) {
  if (!name || !/^\d+-/.test(name) || !fs.existsSync(changeDir(root, name))) {
    throw new Error(`tomo: unknown change "${name}" — run /tomo switch to list specs`);
  }
  const st = defaultState();
  st.change = name;
  const rec = readChanges(root)[name];
  if (rec) {
    if (rec.phase) st.phase = rec.phase;
    if (rec.status) st.status = rec.status;
    if (rec.gates && typeof rec.gates === 'object') Object.assign(st.gates, rec.gates);
    if (Array.isArray(rec.sources)) st.sources = rec.sources.slice();
  }
  saveState(root, st);
  return st;
}

// Advance the phase machine: starts the given phase and sets its status.
// Implement is a run phase and uses IMPLEMENT_STATES; gated phases use
// GATE_STATES. Does not enforce order (the gates do), but validates names.
function setGate(state, phase, status) {
  if (!PHASES.includes(phase)) throw new Error(`unknown phase: ${phase}`);
  const allowed = phase === 'implement' ? IMPLEMENT_STATES : GATE_STATES;
  if (!allowed.includes(status)) {
    const label = phase === 'implement' ? 'implement status' : 'gate status';
    throw new Error(`unknown ${label}: ${status}`);
  }
  state.phase = phase;
  state.status = status;
  state.gates[phase] = status;
  return state;
}

// Append one timestamped line to .tomo/log.md. The changeset paper trail.
function appendLog(root, line) {
  fs.mkdirSync(workspace(root), { recursive: true });
  fs.appendFileSync(logPath(root), `- ${new Date().toISOString()} ${line}\n`);
}

module.exports = {
  PHASES,
  GATE_STATES,
  IMPLEMENT_STATES,
  workspace,
  statePath,
  specsDir,
  changeDir,
  logPath,
  slipPath,
  artifactPath,
  nextChangeId,
  slugify,
  defaultState,
  loadState,
  saveState,
  readChanges,
  knownChanges,
  switchTo,
  setGate,
  appendLog,
};