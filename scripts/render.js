// Render the harness-facing command wrappers and persona casts from the
// canonical content store. This is a library for scripts/install.js and the
// tests — there is no separate build step: install folds rendering in, writes
// the output into the gitignored cache (adapters/common/), and then places it
// in each harness's command/agent dirs (copy for a project, symlink for a
// global install).
//
// The source of truth is the content store — the commands' prompt bodies live
// in exactly one place, so there is nothing to hand-sync between harnesses:
//   content/commands/<name>.md   the canonical command body (edit these)
//   content/gate.md              the shared approval-gate contract
// The command contract lives in src/commands/<name>.md as YAML frontmatter
// only:
//   name  — must equal the filename
//   description — one-line picker text (avoid colons)
//   args — argument hint; the body may use $ARGUMENTS
//   gate — none|block|prompt|auto
// Rendering INLINES the canonical body into every wrapper (gated bodies carry
// a `<!-- tomo:include gate.md -->` directive that expands to content/gate.md).
// Wrappers used to be pointers the model resolved at runtime, but reading a
// body from a content store outside the project workspace trips every
// harness's external-path permission prompt; inlined, a /tomo command needs
// zero file reads to start. The cost: editing content/ takes effect only after
// re-running `npm run install` (which re-renders the cache the global
// symlinks point at).
//   adapters/common/tomo-<name>.md   = description frontmatter + inlined body
// adapters/common/ is a gitignored cache — never commit it. install.js copies
// or symlinks these files into each harness's command directory:
//   .opencode/command/tomo-<name>.md    -> /tomo-<name>   (opencode)
//   .claude/commands/tomo/<name>.md     -> /tomo <name>   (Claude Code)
//
// Inlined bodies become harness prompt templates, where some syntax is live:
// opencode and Claude Code run !`cmd` shell injections, attach @path file
// references, and substitute positional $1..$9. checkContent() rejects bodies
// that contain any of those so a canonical body can never execute by accident.
//
// Zero runtime dependencies; works with or without node_modules.

const fs = require('fs');
const path = require('path');
const providers = require('../src/lib/providers.js');

const ROOT = path.join(__dirname, '..');
const COMMANDS_DIR = path.join(ROOT, 'src', 'commands');
const PERSONAS_DIR = path.join(ROOT, 'src', 'personas');
const CONTENT_DIR = path.join(ROOT, 'content');
const CONTENT_COMMANDS_DIR = path.join(CONTENT_DIR, 'commands');
const GATE_CONTENT_PATH = path.join(CONTENT_DIR, 'gate.md');
const COMMON_DIR = path.join(ROOT, 'adapters', 'common');

const PERSONA_REQUIRED = ['name', 'description', 'focus'];

const VALID_GATES = new Set(['none', 'block', 'prompt', 'auto']);
const REQUIRED = ['name', 'description', 'args', 'gate'];

const rel = (p) => path.relative(ROOT, p);

function parseFrontmatter(src) {
  const m = src.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n?/);
  if (!m) throw new Error('command has no YAML frontmatter');
  const fm = {};
  const lines = m[1].split(/\r?\n/);
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (!line.trim()) continue;
    const colon = line.indexOf(':');
    if (colon === -1) throw new Error(`frontmatter line is not "key: value": ${line}`);
    const key = line.slice(0, colon).trim();
    let value = line.slice(colon + 1).trim();
    if (value === '>') {
      const folded = [];
      while (i + 1 < lines.length && lines[i + 1].trim() && !lines[i + 1].includes(':')) {
        folded.push(lines[++i].trim());
      }
      value = folded.join(' ');
    } else if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    fm[key] = value;
  }
  const body = src.slice(m[0].length).replace(/^\n/, '').trimEnd();
  return { fm, body };
}

function readCommands() {
  if (!fs.existsSync(COMMANDS_DIR)) throw new Error(`missing ${rel(COMMANDS_DIR)}`);
  const commands = [];
  for (const file of fs.readdirSync(COMMANDS_DIR).sort()) {
    if (!file.endsWith('.md')) continue;
    const name = path.basename(file, '.md');
    const { fm, body } = parseFrontmatter(fs.readFileSync(path.join(COMMANDS_DIR, file), 'utf8'));
    for (const key of REQUIRED) {
      if (!(key in fm)) throw new Error(`${file}: missing frontmatter field "${key}"`);
    }
    for (const key of ['name', 'description', 'gate']) {
      if (fm[key] === '') throw new Error(`${file}: empty frontmatter field "${key}"`);
    }
    if (fm.name !== name) throw new Error(`${file}: frontmatter name "${fm.name}" != filename "${name}"`);
    if (!VALID_GATES.has(fm.gate)) throw new Error(`${file}: invalid gate "${fm.gate}"`);
    if (body) throw new Error(`${file}: command contracts are frontmatter only; put the body in content/commands/`);
    commands.push({ name, fm, body });
  }
  if (!commands.length) throw new Error('no canonical commands found in src/commands');
  return commands;
}

// The directive a gated canonical body uses to pull in the shared gate.
const GATE_INCLUDE = '<!-- tomo:include gate.md -->';

// The fully inlined prompt for a command: its canonical body with the gate
// directive expanded. Bodies that never mention $ARGUMENTS get a trailing
// arguments line so slash-command arguments still reach the model in every
// harness (opencode only substitutes placeholders that are present).
function inlineBody(cmd) {
  const content = readContentCommands();
  if (!(cmd.name in content)) {
    throw new Error(`missing ${rel(contentCommandPath(cmd.name))} (canonical body)`);
  }
  let body = content[cmd.name];
  if (body.includes(GATE_INCLUDE)) {
    const gate = fs.readFileSync(GATE_CONTENT_PATH, 'utf8').trimEnd();
    body = body.split(GATE_INCLUDE).join(gate);
  }
  if (!body.includes('$ARGUMENTS')) body += '\n\nArguments (may be empty): $ARGUMENTS';
  return body;
}

// The command object with its body replaced by the inlined prompt — what every
// provider's commandCast renders from.
function inlineCommand(cmd) {
  return { ...cmd, body: inlineBody(cmd) };
}

function frontmatter(description) {
  return `---\ndescription: ${description}\n---\n`;
}

// opencode consumes the plain wrapper; providers whose command format diverges
// (Claude's allowed-tools grant, Copilot/Antigravity Skills) own a commandCast.
function render(cmd) {
  return `${frontmatter(cmd.fm.description).trimEnd()}\n\n${inlineBody(cmd)}\n`;
}

// Template syntax opencode/Claude Code would act on inside an inlined body.
const LIVE_SYNTAX = [
  [/!`/, 'shell injection (!`cmd`)'],
  [/\$[0-9]/, 'positional argument ($N)'],
  [/(^|\s)@[\w./~-]/m, 'file reference (@path)'],
];

function liveSyntaxProblems(label, text) {
  const out = [];
  for (const [re, what] of LIVE_SYNTAX) {
    if (re.test(text)) out.push(`${label}: contains harness template syntax: ${what}`);
  }
  return out;
}

// --- Content store --------------------------------------------------------
//
// The canonical command bodies live in content/commands/ and the shared gate
// in content/gate.md. src/commands holds the matching frontmatter contracts;
// checkContent() keeps the two in lockstep (missing/orphan bodies, a gate
// include that disagrees with the contract's gate, and live harness template
// syntax are all drift).

function contentCommandPath(name) {
  return path.join(CONTENT_COMMANDS_DIR, `${name}.md`);
}

function readContentCommands() {
  const out = {};
  if (!fs.existsSync(CONTENT_COMMANDS_DIR)) return out;
  for (const file of fs.readdirSync(CONTENT_COMMANDS_DIR).sort()) {
    if (!file.endsWith('.md')) continue;
    const name = path.basename(file, '.md');
    out[name] = fs.readFileSync(path.join(CONTENT_COMMANDS_DIR, file), 'utf8').trimEnd();
  }
  return out;
}

// --- Vocabulary consistency ------------------------------------------------
//
// Command bodies restate code-owned lists (the settings keys, the command
// index, the lib exports they call, the persona defaults). When the code moves
// and the prose does not, the drift is a live bug the render/placement tests
// cannot see — they check files, not words. checkVocabulary() compares the
// prose against the code that owns each list and returns a problem per drift,
// so a body that enumerates a code-owned list wrong fails the suite.

// The lib modules the bodies reference as `.tomo/lib/<name>.js`. A body names
// a symbol as `.tomo/lib/<m>.js` immediately followed by a backticked
// identifier (with an optional `(` or ` with ` / ` + ` connector, e.g. gate.md's
// "with `setGate` + `saveState`").
const LIB_MODULES = new Set(['state', 'settings', 'issues', 'personas']);

function checkVocabulary(commands, content, code) {
  const problems = [];

  // help.md is the command index: every registered command must appear as a
  // /tomo invocation, or the model has no way to discover it.
  const help = content.help;
  if (help) {
    for (const cmd of commands) {
      if (!new RegExp(`/tomo[ -]${cmd.name}\\b`).test(help)) {
        problems.push(`help: command index missing "/tomo ${cmd.name}"`);
      }
    }
  }

  // init.md and settings.md enumerate the effective settings; they must list
  // every key defaultSettings() owns (a renamed/added key must fail here).
  const keys = Object.keys(code.settings.defaultSettings());
  for (const name of ['init', 'settings']) {
    const body = content[name];
    if (!body) continue;
    for (const key of keys) {
      if (!body.includes('`' + key + '`')) {
        problems.push(`${name}: settings enumeration missing "${key}" (defaultSettings has ${keys.length} keys)`);
      }
    }
  }

  // Every `.tomo/lib/<module>.js` + symbol the prose names must be a real
  // export of that module (a rename in src/lib/ must break the build here).
  const symbolRe = /`\.tomo\/lib\/([a-z]+)\.js`\s*(?:\(\s*|\bwith\b\s+)?`([A-Za-z_][A-Za-z0-9_]*)(?:`\s*\+\s*`([A-Za-z_][A-Za-z0-9_]*)`)?/g;
  for (const [name, body] of Object.entries(content)) {
    for (const m of body.matchAll(symbolRe)) {
      const mod = m[1];
      if (!LIB_MODULES.has(mod)) {
        problems.push(`${name}: unknown lib module "${mod}" (.tomo/lib/${mod}.js)`);
        continue;
      }
      const exports = code.lib(mod);
      for (const sym of [m[2], m[3]].filter(Boolean)) {
        if (!(sym in exports)) {
          problems.push(`${name}: .tomo/lib/${mod}.js has no export "${sym}"`);
        }
      }
    }
  }

  // Each persona paragraph states "By default this is the **X**" — that X must
  // match DEFAULT_PERSONA_MAP for the step resolvePersona is resolving.
  const defaultMap = code.personas.DEFAULT_PERSONA_MAP;
  const personaRe = /resolvePersona\(\s*'([a-z-]+)',\s*settings\)[\s\S]*?By default this is the \*\*([a-z]+)\*\*/g;
  for (const [name, body] of Object.entries(content)) {
    for (const m of body.matchAll(personaRe)) {
      const step = m[1];
      const persona = m[2];
      const expected = defaultMap[step];
      if (expected && expected !== persona) {
        problems.push(`${name}: "${step}" default persona is "${persona}" but DEFAULT_PERSONA_MAP says "${expected}"`);
      }
    }
  }

  return problems;
}

function checkContent() {
  const problems = [];
  if (!fs.existsSync(GATE_CONTENT_PATH)) {
    problems.push(`missing: ${rel(GATE_CONTENT_PATH)} (the shared approval gate)`);
  } else {
    problems.push(...liveSyntaxProblems(rel(GATE_CONTENT_PATH), fs.readFileSync(GATE_CONTENT_PATH, 'utf8')));
  }
  const commands = readCommands();
  const content = readContentCommands();
  const names = new Set(commands.map((c) => c.name));
  for (const cmd of commands) {
    if (!(cmd.name in content)) {
      problems.push(`missing: ${rel(contentCommandPath(cmd.name))} (canonical body)`);
      continue;
    }
    const body = content[cmd.name];
    const includes = body.split(GATE_INCLUDE).length - 1;
    if (cmd.fm.gate !== 'none' && includes !== 1) {
      problems.push(`${cmd.name}: gated command body must include the gate exactly once (${GATE_INCLUDE})`);
    }
    if (cmd.fm.gate === 'none' && includes > 0) {
      problems.push(`${cmd.name}: ungated command body must not include the gate`);
    }
    if (/\bgate\.md\b/.test(body.split(GATE_INCLUDE).join(''))) {
      problems.push(`${cmd.name}: body must not point at gate.md for a runtime read; use ${GATE_INCLUDE}`);
    }
    problems.push(...liveSyntaxProblems(rel(contentCommandPath(cmd.name)), body));
  }
  for (const name of Object.keys(content)) {
    if (!names.has(name)) {
      problems.push(`orphan: ${rel(contentCommandPath(name))} (no matching command)`);
    }
  }

  // The shared gate also names lib symbols, and the command bodies restate
  // code-owned lists — fold the vocabulary check in so drift fails the suite.
  const vocabContent = { ...content };
  if (fs.existsSync(GATE_CONTENT_PATH)) vocabContent.gate = fs.readFileSync(GATE_CONTENT_PATH, 'utf8');
  problems.push(...checkVocabulary(commands, vocabContent, {
    settings: require('../src/lib/settings.js'),
    personas: require('../src/lib/personas.js'),
    lib: (mod) => require(path.join(ROOT, 'src', 'lib', `${mod}.js`)),
  }));

  return problems;
}

// --- Personas -------------------------------------------------------------
//
// Canonical persona files in src/personas/ carry home-grown frontmatter
// (name, description, `tools`, focus) plus a body that is the verbatim system
// prompt. Harness agent formats have NOT converged, so each provider in
// src/lib/providers.js owns its cast renderer: opencode wants { description,
// mode } (unknown keys pass through to the provider as model options), while
// Claude Code subagents require { name, description }. Rendering writes every
// provider's cast into the adapters/common/ cache; install.js then materializes
// only the provider(s) the user chooses.

function readPersonas() {
  if (!fs.existsSync(PERSONAS_DIR)) return [];
  const personas = [];
  for (const file of fs.readdirSync(PERSONAS_DIR).sort()) {
    if (!file.endsWith('.md')) continue;
    const name = path.basename(file, '.md');
    const { fm, body } = parseFrontmatter(fs.readFileSync(path.join(PERSONAS_DIR, file), 'utf8'));
    for (const key of PERSONA_REQUIRED) {
      if (!(key in fm)) throw new Error(`persona ${file}: missing frontmatter field "${key}"`);
    }
    if (fm.name !== name) throw new Error(`persona ${file}: frontmatter name "${fm.name}" != filename "${name}"`);
    if (fm.description === '' || fm.focus === '') {
      throw new Error(`persona ${file}: empty frontmatter field`);
    }
    if (!body) throw new Error(`persona ${file}: empty body`);
    personas.push({ name, fm, body });
  }
  return personas;
}

// Delegating helpers kept for tests/back-compat; the cast logic lives in the
// provider registry (src/lib/providers.js), the single onboarding point.
function renderPersonaOpenCode(p) {
  return providers.PROVIDERS.opencode.cast(p);
}

function renderPersonaClaude(p) {
  return providers.PROVIDERS.claude.cast(p);
}

function renderPersonaCopilot(p) {
  return providers.PROVIDERS.copilot.cast(p);
}

function renderPersonaAntigravity(p) {
  return providers.PROVIDERS.antigravity.cast(p);
}

function renderPersonaCodex(p) {
  return providers.PROVIDERS.codex.cast(p);
}

function generatePersonas() {
  const out = [];
  for (const p of readPersonas()) {
    for (const id of providers.providerIds()) {
      out.push({ name: p.name, cast: id, content: providers.PROVIDERS[id].cast(p) });
    }
  }
  return out;
}

function personaCastPath(name, cast) {
  return path.join(COMMON_DIR, `persona-${name}.${cast}.md`);
}

function commonPath(name) {
  return path.join(COMMON_DIR, `tomo-${name}.md`);
}

// Cache path for a provider's command wrapper. A harness that consumes the
// plain wrapper (opencode) reuses commonPath(); a provider with its own
// `commandCast` (Claude's allowed-tools grant, Copilot and Antigravity Skills'
// `name` key) gets a cast-specific file in the same gitignored cache.
function providerCommandPath(name, id) {
  return providers.PROVIDERS[id].commandCast
    ? path.join(COMMON_DIR, `tomo-${name}.${id}.md`)
    : commonPath(name);
}

function customCommandProviders() {
  return providers.providerIds().filter((id) => providers.PROVIDERS[id].commandCast);
}

function generate() {
  return readCommands().map((cmd) => ({
    name: cmd.name,
    fm: cmd.fm,
    content: render(cmd),
  }));
}

function writeAll() {
  const written = [];
  fs.mkdirSync(COMMON_DIR, { recursive: true });
  for (const g of generate()) {
    fs.writeFileSync(commonPath(g.name), g.content);
    written.push(commonPath(g.name));
  }
  for (const cmd of readCommands()) {
    for (const id of customCommandProviders()) {
      const pth = providerCommandPath(cmd.name, id);
      fs.writeFileSync(pth, providers.PROVIDERS[id].commandCast(inlineCommand(cmd)));
      written.push(pth);
    }
  }
  for (const p of generatePersonas()) {
    const pth = personaCastPath(p.name, p.cast);
    fs.writeFileSync(pth, p.content);
    written.push(pth);
  }
  return written;
}

module.exports = {
  ROOT,
  parseFrontmatter,
  readCommands,
  readContentCommands,
  contentCommandPath,
  checkContent,
  checkVocabulary,
  inlineBody,
  inlineCommand,
  liveSyntaxProblems,
  GATE_INCLUDE,
  render,
  commonPath,
  generate,
  readPersonas,
  renderPersonaOpenCode,
  renderPersonaClaude,
  renderPersonaCopilot,
  renderPersonaAntigravity,
  renderPersonaCodex,
  generatePersonas,
  personaCastPath,
  providerCommandPath,
  customCommandProviders,
  writeAll,
  rel,
  CONTENT_DIR,
  CONTENT_COMMANDS_DIR,
  GATE_CONTENT_PATH,
  COMMON_DIR,
};