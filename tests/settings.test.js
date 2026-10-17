const { test } = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const settings = require('../src/lib/settings.js');

// Isolate the global settings file so tests never touch ~/.config/tomo.
const TEST_CONFIG_HOME = fs.mkdtempSync(path.join(os.tmpdir(), 'tomo-config-test-'));
const prevXdg = process.env.XDG_CONFIG_HOME;
process.env.XDG_CONFIG_HOME = TEST_CONFIG_HOME;

function tmpRoot() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'tomo-settings-test-'));
}

test('defaults apply when no settings files exist', () => {
  const root = tmpRoot();
  const { global, project, effective } = settings.loadSettings(root);
  assert.deepStrictEqual(global, {});
  assert.deepStrictEqual(project, {});
  assert.deepStrictEqual(effective, settings.defaultSettings());
});

test('project supersedes global, global supersedes defaults', () => {
  const root = tmpRoot();
  const globalPath = settings.globalSettingsPath();
  const projectPath = settings.projectSettingsPath(root);
  fs.mkdirSync(path.dirname(globalPath), { recursive: true });
  fs.mkdirSync(path.dirname(projectPath), { recursive: true });
  fs.writeFileSync(globalPath, JSON.stringify({ baseSource: 'issue:12', gates: 'auto', provider: 'opencode', issues: 'local', issuesDir: 'tickets' }));
  fs.writeFileSync(projectPath, JSON.stringify({ gates: 'prompt', issuesDir: 'issues/custom' }));
  const { effective } = settings.loadSettings(root);
  assert.strictEqual(effective.baseSource, 'issue:12');
  assert.strictEqual(effective.gates, 'prompt');
  assert.strictEqual(effective.provider, 'opencode');
  assert.strictEqual(effective.artifactDir, '.tomo/specs');
  assert.strictEqual(effective.issues, 'local');
  assert.strictEqual(effective.issuesDir, 'issues/custom');
});

test('invalid gates value throws on merge', () => {
  assert.throws(() => settings.mergeSettings({ gates: 'aggressive' }), /invalid gates/);
});

test('personas map and custom merge per key across global and project', () => {
  const global = { personas: { map: { plan: 'security', trace: 'reviewer' }, custom: { lead: 'personas/lead.md' } } };
  const project = { personas: { map: { plan: 'architect' }, custom: { pm: 'personas/pm.md' } } };
  const eff = settings.mergeSettings(global, project);
  assert.deepStrictEqual(eff.personas, {
    map: { plan: 'architect', trace: 'reviewer' },
    custom: { lead: 'personas/lead.md', pm: 'personas/pm.md' },
  }, 'project wins per step, global keys survive, aliases accumulate');
});

test('personas defaults are empty and loadSettings layers them', () => {
  assert.deepStrictEqual(settings.defaultSettings().personas, { map: {}, custom: {} });
  const root = tmpRoot();
  const globalPath = settings.globalSettingsPath();
  fs.mkdirSync(path.dirname(globalPath), { recursive: true });
  fs.writeFileSync(globalPath, JSON.stringify({ personas: { map: { debt: 'security' } } }));
  const { effective } = settings.loadSettings(root);
  assert.strictEqual(effective.personas.map.debt, 'security');
  assert.deepStrictEqual(effective.personas.custom, {});
});

test('invalid issues value falls back to github without throwing', () => {
  const root = tmpRoot();
  const projectPath = settings.projectSettingsPath(root);
  fs.mkdirSync(path.dirname(projectPath), { recursive: true });
  fs.writeFileSync(projectPath, JSON.stringify({ issues: 'bogus' }));
  const { effective } = settings.loadSettings(root);
  assert.strictEqual(effective.issues, 'github');
});

test('unreadable JSON settings file throws with a helpful message', () => {
  const root = tmpRoot();
  const p = settings.projectSettingsPath(root);
  fs.mkdirSync(path.dirname(p), { recursive: true });
  fs.writeFileSync(p, '{oops');
  assert.throws(() => settings.loadSettings(root), /cannot parse settings/);
});

test('global path honors XDG_CONFIG_HOME', () => {
  const alt = fs.mkdtempSync(path.join(os.tmpdir(), 'tomo-xdg-alt-'));
  process.env.XDG_CONFIG_HOME = alt;
  try {
    assert.ok(settings.globalSettingsPath().startsWith(alt));
  } finally {
    process.env.XDG_CONFIG_HOME = TEST_CONFIG_HOME;
  }
});