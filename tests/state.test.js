const { test } = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const state = require('../src/lib/state.js');

function tmpRoot() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'tomo-test-'));
}

test('loadState returns default when no state.json exists', () => {
  const st = state.loadState(tmpRoot());
  assert.deepStrictEqual(st, state.defaultState());
});

test('saveState then loadState roundtrips and stamps updatedAt', () => {
  const root = tmpRoot();
  const st = state.defaultState();
  state.setGate(st, 'requirements', 'await-approval');
  st.sources = ['specs/prd.md'];
  const before = { ...st };
  state.saveState(root, st);
  assert.ok(st.updatedAt, 'updatedAt must be stamped');
  const loaded = state.loadState(root);
  assert.strictEqual(loaded.phase, 'requirements');
  assert.strictEqual(loaded.status, 'await-approval');
  assert.deepStrictEqual(loaded.sources, ['specs/prd.md']);
  assert.deepStrictEqual(loaded.gates, before.gates);
});

test('setGate updates phase, status and the per-phase gate map', () => {
  const st = state.defaultState();
  state.setGate(st, 'design', 'draft');
  assert.strictEqual(st.phase, 'design');
  assert.strictEqual(st.status, 'draft');
  assert.strictEqual(st.gates.design, 'draft');
  assert.strictEqual(st.gates.requirements, '');
});

test('setGate rejects unknown phase and status', () => {
  const st = state.defaultState();
  assert.throws(() => state.setGate(st, 'bogus', 'draft'), /unknown phase/);
  assert.throws(() => state.setGate(st, 'plan', 'nope'), /unknown gate status/);
});

test('defaultState seeds every phase including the implement run phase', () => {
  assert.deepStrictEqual(state.PHASES, ['requirements', 'design', 'plan', 'implement']);
  assert.deepStrictEqual(state.IMPLEMENT_STATES, ['', 'in-progress', 'complete']);
  const gates = state.defaultState().gates;
  assert.deepStrictEqual(gates, { requirements: '', design: '', plan: '', implement: '' });
});

test('implement uses run-phase statuses, not gate statuses', () => {
  const st = state.defaultState();
  state.setGate(st, 'implement', 'in-progress');
  assert.strictEqual(st.phase, 'implement');
  assert.strictEqual(st.status, 'in-progress');
  assert.strictEqual(st.gates.implement, 'in-progress');
  state.setGate(st, 'implement', 'complete');
  assert.strictEqual(st.status, 'complete');
  assert.strictEqual(st.gates.implement, 'complete');
  assert.throws(() => state.setGate(st, 'implement', 'draft'), /unknown implement status/);
  assert.throws(() => state.setGate(st, 'implement', 'approved'), /unknown implement status/);
});

test('saveState writes the slim shape; loadState resolves from changes[change]', () => {
  const root = tmpRoot();
  fs.mkdirSync(path.join(state.specsDir(root), '1-alpha'), { recursive: true });
  fs.mkdirSync(path.join(state.specsDir(root), '2-beta'), { recursive: true });

  const a = state.defaultState();
  a.change = '1-alpha';
  state.setGate(a, 'requirements', 'approved');
  a.sources = ['prd.md'];
  state.saveState(root, a);

  const b = state.switchTo(root, '2-beta');
  state.setGate(b, 'design', 'draft');
  state.saveState(root, b);

  const raw = JSON.parse(fs.readFileSync(state.statePath(root), 'utf8'));
  assert.deepStrictEqual(Object.keys(raw), ['change', 'updatedAt', 'changes']);
  assert.strictEqual(raw.change, '2-beta');
  assert.strictEqual(raw.changes['1-alpha'].gates.requirements, 'approved');
  assert.strictEqual(raw.changes['2-beta'].gates.design, 'draft');

  const active = state.loadState(root);
  assert.strictEqual(active.change, '2-beta');
  assert.strictEqual(active.phase, 'design');
  assert.strictEqual(active.status, 'draft');

  const switched = state.switchTo(root, '1-alpha');
  assert.strictEqual(switched.phase, 'requirements');
  assert.strictEqual(switched.status, 'approved');
  assert.deepStrictEqual(switched.sources, ['prd.md']);
});

test('a legacy three-gate state.json still loads with implement backfilled', () => {
  const root = tmpRoot();
  fs.mkdirSync(state.specsDir(root), { recursive: true });
  fs.writeFileSync(
    state.statePath(root),
    JSON.stringify({
      phase: 'plan',
      status: 'approved',
      gates: { requirements: 'approved', design: 'approved', plan: 'approved' },
      sources: ['legacy'],
      change: '1-legacy',
    }),
  );
  const loaded = state.loadState(root);
  assert.strictEqual(loaded.gates.implement, '');
  assert.strictEqual(loaded.gates.plan, 'approved');
});

test('artifact paths live in a numbered change folder under .tomo/specs', () => {
  const root = tmpRoot();
  assert.strictEqual(
    state.artifactPath(root, '1-local-issue-store', 'requirements.md'),
    path.join(root, '.tomo', 'specs', '1-local-issue-store', 'requirements.md'),
  );
  assert.strictEqual(state.artifactPath(root, '', 'findings.md'), path.join(root, '.tomo', 'specs', 'findings.md'));
});

test('nextChangeId scans numbered folders and increments by one', () => {
  const root = tmpRoot();
  assert.strictEqual(state.nextChangeId(root), 1, 'no specs dir yet → 1');
  fs.mkdirSync(state.specsDir(root), { recursive: true });
  assert.strictEqual(state.nextChangeId(root), 1);
  fs.mkdirSync(path.join(state.specsDir(root), '1-wrappers'));
  fs.mkdirSync(path.join(state.specsDir(root), '2-tasks-layout'));
  fs.mkdirSync(path.join(state.specsDir(root), 'README.md'));
  assert.strictEqual(state.nextChangeId(root), 3, 'skips non-<id>- entries');
});

test('slugify lowercases to kebab-case and truncates', () => {
  assert.strictEqual(state.slugify('Local Issue Store!'), 'local-issue-store');
  assert.strictEqual(state.slugify('  ', ), '');
  assert.strictEqual(state.slugify('spec folder layout'), 'spec-folder-layout');
  assert.ok(state.slugify('a ' + 'x'.repeat(100)).length <= 40);
});

test('changeDir resolves to specs base when change is empty', () => {
  const root = tmpRoot();
  assert.strictEqual(state.changeDir(root, ''), state.specsDir(root));
  assert.strictEqual(state.changeDir(root, '3-thing'), path.join(state.specsDir(root), '3-thing'));
});

test('each change keeps its own phase machine; switchTo restores it', () => {
  const root = tmpRoot();
  fs.mkdirSync(path.join(state.specsDir(root), '1-local-issue-store'), { recursive: true });
  fs.mkdirSync(path.join(state.specsDir(root), '2-wrappers'), { recursive: true });

  const st = state.defaultState();
  st.change = '1-local-issue-store';
  state.setGate(st, 'plan', 'approved');
  st.sources = ['inline statement'];
  state.saveState(root, st);

  const other = state.switchTo(root, '2-wrappers');
  assert.strictEqual(other.change, '2-wrappers');
  assert.strictEqual(other.phase, '', 'fresh folder has no recorded progress');
  assert.strictEqual(other.status, '');

  const back = state.switchTo(root, '1-local-issue-store');
  assert.strictEqual(back.change, '1-local-issue-store');
  assert.strictEqual(back.phase, 'plan');
  assert.strictEqual(back.status, 'approved');
  assert.deepStrictEqual(back.sources, ['inline statement']);
  assert.strictEqual(back.gates.plan, 'approved');
});

test('switchTo rejects unknowns; knownChanges lists folders with progress', () => {
  const root = tmpRoot();
  assert.throws(() => state.switchTo(root, 'nope'), /unknown change/);
  assert.throws(() => state.switchTo(root, ''), /unknown change/);
  fs.mkdirSync(path.join(state.specsDir(root), '1-a'), { recursive: true });
  fs.mkdirSync(path.join(state.specsDir(root), '2-b'), { recursive: true });
  fs.mkdirSync(path.join(state.specsDir(root), 'README.md'));

  assert.deepStrictEqual(state.knownChanges(root).map((c) => c.name), ['1-a', '2-b']);

  const st = state.defaultState();
  st.change = '1-a';
  state.setGate(st, 'requirements', 'await-approval');
  state.saveState(root, st);

  const list = state.knownChanges(root);
  assert.strictEqual(list.find((c) => c.name === '1-a').status, 'await-approval');
  assert.strictEqual(list.find((c) => c.name === '2-b').status, '');
});

test('appendLog writes timestamped lines to .tomo/log.md', () => {
  const root = tmpRoot();
  state.appendLog(root, 'requirements → await-approval');
  state.appendLog(root, 'requirements approved');
  const log = fs.readFileSync(path.join(root, '.tomo', 'log.md'), 'utf8');
  const lines = log.trim().split('\n');
  assert.strictEqual(lines.length, 2);
  assert.match(lines[0], /^- \d{4}-\d{2}-\d{2}T/);
  assert.ok(lines[0].includes('await-approval'));
  assert.ok(lines[1].includes('approved'));
});