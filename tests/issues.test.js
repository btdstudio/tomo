const { test } = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const issues = require('../src/lib/issues.js');

function tmpRoot() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'tomo-issues-test-'));
}

test('writeNewIssue creates issues/1.md with the controlled frontmatter', () => {
  const root = tmpRoot();
  const created = issues.writeNewIssue(root, {}, { title: 'Build the thing', labels: ['docs', 'ui'], body: 'Wire up the store.' });
  assert.strictEqual(created.id, 1);
  assert.strictEqual(created.status, 'open');
  assert.strictEqual(created.title, 'Build the thing');
  assert.deepStrictEqual(created.labels, ['docs', 'ui']);
  assert.strictEqual(created.body, 'Wire up the store.');
  assert.ok(created.path.endsWith(path.join('issues', '1.md')));
  const saved = fs.readFileSync(created.path, 'utf8');
  assert.match(saved, /^---\nid: 1\nstatus: open\ntitle: Build the thing\nlabels: \[docs, ui\]\n---\n\nWire up the store\.\n$/);
});

test('ids are sequential and never reused', () => {
  const root = tmpRoot();
  const a = issues.writeNewIssue(root, {}, { title: 'a' });
  const b = issues.writeNewIssue(root, {}, { title: 'b' });
  const c = issues.writeNewIssue(root, {}, { title: 'c' });
  assert.deepStrictEqual([a.id, b.id, c.id], [1, 2, 3]);
});

test('listIssues returns issues sorted by id, skipping non-numeric files', () => {
  const root = tmpRoot();
  issues.writeNewIssue(root, {}, { title: 'first' });
  issues.writeNewIssue(root, {}, { title: 'second' });
  fs.writeFileSync(path.join(root, 'issues', 'README.md'), 'not an issue\n');
  const list = issues.listIssues(root, {});
  assert.deepStrictEqual(list.map((i) => i.id), [1, 2]);
  assert.deepStrictEqual(list.map((i) => i.title), ['first', 'second']);
});

test('readIssue returns full detail; missing issue throws', () => {
  const root = tmpRoot();
  const created = issues.writeNewIssue(root, {}, { title: 'T', body: 'Body' });
  const got = issues.readIssue(root, {}, created.id);
  assert.strictEqual(got.title, 'T');
  assert.strictEqual(got.body, 'Body');
  assert.throws(() => issues.readIssue(root, {}, 99), /not found/);
});

test('issuesDir setting relocates the store', () => {
  const root = tmpRoot();
  const created = issues.writeNewIssue(root, { issuesDir: 'tickets' }, { title: 'x' });
  assert.ok(created.path.endsWith(path.join('tickets', '1.md')));
  assert.strictEqual(issues.localIssuesDir(root, { issuesDir: 'tickets' }), path.join(root, 'tickets'));
});

test('empty store lists as [] and next id starts at 1', () => {
  const root = tmpRoot();
  assert.deepStrictEqual(issues.listIssues(root, {}), []);
  assert.strictEqual(issues.nextId(root, {}), 1);
});

test('writeIssue refuses to clobber an existing file (AC7)', () => {
  const root = tmpRoot();
  const dir = issues.localIssuesDir(root, {});
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(dir, '5.md'), 'existing content');
  const real = fs.readdirSync;
  fs.readdirSync = () => ['3.md', '4.md'];
  try {
    assert.throws(() => issues.writeNewIssue(root, {}, { title: 'x' }), /not overwriting/);
  } finally {
    fs.readdirSync = real;
  }
  assert.strictEqual(fs.readFileSync(path.join(dir, '5.md'), 'utf8'), 'existing content');
});

test('parseIssue round-trips status closed and empty labels', () => {
  const root = tmpRoot();
  const created = issues.writeNewIssue(root, {}, { title: 'long', labels: [] });
  const raw = `---\nid: ${created.id}\nstatus: closed\ntitle: long\nlabels: []\n---\n`;
  fs.writeFileSync(created.path, raw);
  const got = issues.readIssue(root, {}, created.id);
  assert.strictEqual(got.status, 'closed');
  assert.deepStrictEqual(got.labels, []);
});