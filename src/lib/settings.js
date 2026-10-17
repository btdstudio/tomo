'use strict';
// Tomo settings module. Settings are mergeable JSON: global
// (~/.config/tomo/settings.json) then project (.tomo/settings.json); the
// project supersedes the global. Effective settings = defaults ← global ←
// project. There is no per-harness config — one contract drives every harness.

const fs = require('fs');
const os = require('os');
const path = require('path');
const state = require('./state.js');

const VALID_GATES = ['block', 'prompt', 'auto'];
const VALID_ISSUES = ['github', 'local'];

function defaultSettings() {
  return {
    baseSource: '', // default base requirement source: file | "issue:N" | "url:..."
    gates: 'block', // block | prompt | auto — approval-gate policy
    provider: '', // optional harness tag (opencode, claude, ...)
    artifactDir: '.tomo/specs',
    issues: 'github', // github | local — issue backend
    issuesDir: 'issues', // local issue store directory (relative to project root)
    personas: { map: {}, custom: {} }, // step→persona overrides; custom maps alias→file
  };
}

// personas merge key-wise (per step, per alias) so a project can override one
// step while inheriting the global map. Other settings stay shallow.
function mergePersonas(base, next) {
  const out = { map: {}, custom: {} };
  for (const group of ['map', 'custom']) {
    const b = (base && base[group]) || {};
    const n = (next && next[group]) || {};
    out[group] = { ...b, ...n };
  }
  return out;
}

function globalSettingsPath() {
  const base = process.env.XDG_CONFIG_HOME || path.join(os.homedir(), '.config');
  return path.join(base, 'tomo', 'settings.json');
}

function projectSettingsPath(root) {
  return path.join(state.workspace(root), 'settings.json');
}

function readIfExists(file) {
  if (!fs.existsSync(file)) return {};
  let parsed;
  try {
    parsed = JSON.parse(fs.readFileSync(file, 'utf8'));
  } catch (e) {
    throw new Error(`tomo: cannot parse settings ${file}: ${e.message}`);
  }
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(`tomo: settings must be a JSON object: ${file}`);
  }
  return parsed;
}

function mergeSettings(...layers) {
  const out = defaultSettings();
  for (const layer of layers) {
    for (const [key, value] of Object.entries(layer || {})) {
      if (value !== undefined && value !== null) {
        out[key] = key === 'personas' ? mergePersonas(out[key], value) : value;
      }
    }
  }
  if (!VALID_GATES.includes(out.gates)) {
    throw new Error(`tomo: invalid gates "${out.gates}" (expected ${VALID_GATES.join('|')})`);
  }
  if (!VALID_ISSUES.includes(out.issues)) {
    console.warn(`tomo: invalid issues "${out.issues}" (expected ${VALID_ISSUES.join('|')}); using "github"`);
    out.issues = 'github';
  }
  return out;
}

// Returns { global, project, effective }.
function loadSettings(root) {
  const global = readIfExists(globalSettingsPath());
  const project = readIfExists(projectSettingsPath(root));
  const effective = mergeSettings(global, project);
  return { global, project, effective };
}

module.exports = {
  VALID_GATES,
  VALID_ISSUES,
  defaultSettings,
  globalSettingsPath,
  projectSettingsPath,
  readIfExists,
  mergeSettings,
  mergePersonas,
  loadSettings,
};