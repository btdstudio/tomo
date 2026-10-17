'use strict';
// Tomo local issue store module. One <id>.md file per issue under the
// settings `issuesDir` (default `issues`), with a controlled YAML-subset
// frontmatter this module both writes and reads: scalar `id`, `status`
// (open|closed), `title`, and an inline `labels` list. No YAML parser, no
// third-party dependencies. The GitHub path intentionally lives nowhere here —
// `gh` passthrough is described by the command bodies.

const fs = require('fs');
const path = require('path');

const DEFAULT_ISSUES_DIR = 'issues';
const OPEN = 'open';
const CLOSED = 'closed';

// Resolve the local store directory against the project root.
function localIssuesDir(root, settings) {
  const dir = (settings && settings.issuesDir) || DEFAULT_ISSUES_DIR;
  return path.join(root, dir);
}

// Absolute path to a single issue file inside `dir`.
function issuePath(dir, id) {
  return path.join(dir, `${id}.md`);
}

// Split the controlled frontmatter off the raw file text.
// Returns { fm, body } with fm keyed lowercase; unknown keys ignored.
function parseFrontmatter(raw) {
  const lines = raw.replace(/\r\n/g, '\n').split('\n');
  if (!(lines[0] && lines[0].trim() === '---')) return { fm: {}, body: raw };
  const end = lines.indexOf('---', 1);
  if (end < 2) return { fm: {}, body: raw };
  const fm = {};
  for (const line of lines.slice(1, end)) {
    const i = line.indexOf(':');
    if (i === -1) continue;
    fm[line.slice(0, i).trim().toLowerCase()] = line.slice(i + 1).trim();
  }
  const body = lines.slice(end + 1).join('\n').replace(/^\n/, '').replace(/\n$/, '');
  return { fm, body };
}

// Parse frontmatter into a plain issue object.
function parseIssue(raw, fallbackId) {
  const { fm, body } = parseFrontmatter(raw);
  let labels = [];
  if (Array.isArray(fm.labels)) labels = fm.labels;
  else if (fm.labels)
    labels = fm.labels
      .replace(/^\[|\]$/g, '')
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
  return {
    id: Number(fm.id) || fallbackId || 0,
    status: fm.status === CLOSED ? CLOSED : OPEN,
    title: fm.title || '',
    labels,
    body,
  };
}

// Serialize an issue to the controlled frontmatter + body text.
function toFrontmatter(issue) {
  const labels = Array.isArray(issue.labels) && issue.labels.length
    ? `[${issue.labels.join(', ')}]`
    : '[]';
  const title = String(issue.title || '').replace(/\s*\r?\n+\s*/g, ' ').trim();
  const body = String(issue.body || '');
  return `---\nid: ${issue.id}\nstatus: ${issue.status}\ntitle: ${title}\nlabels: ${labels}\n---\n\n${body}`;
}

// Highest numeric id present in the store + 1; 1 when the store is empty.
function nextId(root, settings) {
  const dir = localIssuesDir(root, settings);
  if (!fs.existsSync(dir)) return 1;
  let max = 0;
  for (const file of fs.readdirSync(dir)) {
    const m = file.match(/^(\d+)\.md$/);
    if (m) max = Math.max(max, Number(m[1]));
  }
  return max + 1;
}

// All issues in the store, sorted by id. Skips non-<id>.md files so a
// README or notes file in the dir is left alone.
function listIssues(root, settings) {
  const dir = localIssuesDir(root, settings);
  if (!fs.existsSync(dir)) return [];
  const issues = [];
  for (const file of fs.readdirSync(dir)) {
    const m = file.match(/^(\d+)\.md$/);
    if (!m) continue;
    const id = Number(m[1]);
    const p = path.join(dir, file);
    const issue = parseIssue(fs.readFileSync(p, 'utf8'), id);
    issue.path = p;
    issues.push(issue);
  }
  issues.sort((a, b) => a.id - b.id);
  return issues;
}

// Read one issue. Throws if the store has no such issue.
function readIssue(root, settings, id) {
  const p = issuePath(localIssuesDir(root, settings), Number(id));
  if (!fs.existsSync(p)) {
    throw new Error(`tomo: local issue ${id} not found at ${p}`);
  }
  const issue = parseIssue(fs.readFileSync(p, 'utf8'), Number(id));
  issue.path = p;
  return issue;
}

// Write one issue file, refusing to clobber an existing one.
function writeIssue(root, settings, issue) {
  const dir = localIssuesDir(root, settings);
  fs.mkdirSync(dir, { recursive: true });
  const p = issuePath(dir, issue.id);
  if (fs.existsSync(p)) {
    throw new Error(`tomo: local issue ${issue.id} already exists at ${p}; not overwriting`);
  }
  fs.writeFileSync(p, toFrontmatter(issue) + '\n');
  const written = parseIssue(fs.readFileSync(p, 'utf8'), issue.id);
  written.path = p;
  return written;
}

// Create a new issue: next sequential id, status open. Never overwrites.
function writeNewIssue(root, settings, { title, labels, body } = {}) {
  const id = nextId(root, settings);
  return writeIssue(root, settings, { id, status: OPEN, title: title || '', labels: labels || [], body: body || '' });
}

module.exports = {
  DEFAULT_ISSUES_DIR,
  OPEN,
  CLOSED,
  localIssuesDir,
  issuePath,
  parseFrontmatter,
  parseIssue,
  toFrontmatter,
  nextId,
  listIssues,
  readIssue,
  writeIssue,
  writeNewIssue,
};