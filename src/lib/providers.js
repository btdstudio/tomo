'use strict';
// Tomo provider registry. One entry per supported AI coding harness: where its
// agent files live (project + user scope) and how a canonical persona profile
// is cast into that harness's agent format. This is the single onboarding point
// for a new harness — add one entry here and both the build (adapters/common
// casts) and install.js (targeted per-provider installs) pick it up.
//
// The cast formats do NOT converge: opencode markdown agents require
// `description`/`mode` (and unknown keys leak through to the provider as model
// options, so a Claude `name` or `tools` field is dangerous there), while
// Claude Code subagents require a `name` frontmatter field. Hence one cast
// renderer per provider, all fed from the same canonical persona profile.

const fs = require('fs');
const os = require('os');
const path = require('path');

function configBase() {
  return process.env.XDG_CONFIG_HOME || path.join(os.homedir(), '.config');
}

// Personas that review rather than write: their casts deny edits.
const READONLY_PERSONAS = new Set(['reviewer', 'security']);

// opencode markdown agent (project .opencode/agents/<name>.md): the filename is
// the agent name, so the cast carries description + mode and per-persona edit
// denial. Never emit a `name`/`tools` key here — unknown frontmatter keys are
// passed to the provider as model options.
function castOpenCode(p) {
  let fm = `description: ${p.fm.description}\nmode: subagent`;
  if (READONLY_PERSONAS.has(p.name)) {
    fm += '\npermission:\n  edit: deny\n  bash: deny';
  }
  return `---\n${fm}\n---\n\n${p.body}\n`;
}

// Claude Code subagent (.claude/agents/<name>.md): identity comes from the
// `name` frontmatter field; tools is omitted when the persona inherits all.
function castClaude(p) {
  let fm = `name: ${p.name}\ndescription: ${p.fm.description}`;
  if (p.fm.tools) fm += `\ntools: ${p.fm.tools}`;
  return `---\n${fm}\n---\n\n${p.body}\n`;
}

function castCopilot(p) {
  let fm = `name: ${p.name}\ndescription: ${p.fm.description}`;
  if (READONLY_PERSONAS.has(p.name)) fm += '\ntools: ["read", "search"]';
  return `---\n${fm}\n---\n\n${p.body}`;
}

// Google Antigravity CLI (agy) custom subagent (.agents/agents/<name>/agent.md
// or ~/.gemini/config/agents/): name + description are the identity, mainAgent
// hides tomo personas from the primary-agent dropdown, subagent makes them
// invokable via invoke_subagent. The `tools` list is exact — an unknown tool
// name can hang the subagent — so only documented names are ever emitted, and
// read-only personas (reviewer/security) get nothing that can write or run
// commands. `tools` is omitted when a persona inherits all tools (the profile's
// own `tools:` frontmatter holds Claude-flavored names, so it is not reused).
const ANTIGRAVITY_READONLY_TOOLS = 'tools:\n  - view_file\n  - grep_search\n';
function castAntigravity(p) {
  let fm = `name: ${p.name}\ndescription: ${p.fm.description}\nsubagent: true\nmainAgent: false`;
  if (READONLY_PERSONAS.has(p.name)) fm += `\n${ANTIGRAVITY_READONLY_TOOLS.trimEnd()}`;
  return `---\n${fm}\n---\n\n${p.body}\n`;
}

// OpenAI Codex subagent (.codex/agents/<name>.toml or ~/.codex/agents/): TOML,
// not markdown. name + description are the identity; developer_instructions is
// the canonical body verbatim. Every persona declares its sandbox explicitly:
// read-only personas (reviewer/security) get "read-only" so Codex cannot edit
// from them; the rest (analyst/architect/coder) get "workspace-write" so the
// implement loop can write. Codex subagents inherit the parent sandbox when the
// key is omitted, so leaving it off would make the coder read-only by default.
function castCodex(p) {
  const esc = (v) => v.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
  const sandbox = READONLY_PERSONAS.has(p.name) ? 'read-only' : 'workspace-write';
  let out = `name = "${esc(p.name)}"\ndescription = "${esc(p.fm.description)}"\nsandbox_mode = "${sandbox}"\n`;
  out += `developer_instructions = """\n${p.body.replace(/\\/g, '\\\\').replace(/"""/g, '\\"""')}\n"""\n`;
  return out;
}

// The tomo config dir (global settings.json lives here). It sits outside every
// project workspace, so harnesses gate reads of it behind an approval prompt;
// install seeds a per-harness read grant for it.
function tomoConfigDir() {
  return path.join(configBase(), 'tomo');
}

// Home-relative form (`~/...`) for permission patterns, which both opencode and
// Claude Code expand; falls back to the absolute path outside $HOME.
function homeRelative(p) {
  const home = os.homedir();
  if (p === home) return '~';
  if (p.startsWith(home + path.sep)) return `~/${path.relative(home, p).split(path.sep).join('/')}`;
  return p;
}

// Claude Code command (.claude/commands/tomo/<name>.md): the inlined body plus
// an `allowed-tools` grant so the bodies' reads of the global tomo settings
// (outside the project) never prompt. The grant lasts only for the invoking
// turn. Claude's permission syntax wants `//abs` for absolute paths.
function claudeReadGrant() {
  const rel = homeRelative(tomoConfigDir());
  return `Read(${rel.startsWith('~') ? rel : `/${rel}`}/**)`;
}

function castClaudeCommand(cmd) {
  return `---\ndescription: ${cmd.fm.description}\nallowed-tools: ${claudeReadGrant()}\n---\n\n${cmd.body}\n`;
}

// Antigravity commands are Skills (a folder with a SKILL.md whose name —
// frontmatter `name` or folder name — becomes the slash command). Skills have
// no documented $ARGUMENTS substitution, so besides the name-fronted cast a
// one-line shim tells the model how the harness conveys the slash-command
// trailing text; the inlined body itself is never changed.
function castAntigravityCommand(cmd) {
  const name = `tomo-${cmd.name}`;
  const shim =
    `The user invoked this skill as \`/${name}\`; text typed after the command name is its arguments. ` +
    `Treat that text as \`$ARGUMENTS\` in the body below (absent text means no arguments were provided).`;
  return `---\nname: ${name}\ndescription: ${cmd.fm.description}\n---\n\n${shim}\n\n${cmd.body}\n`;
}

function castCopilotCommand(cmd) {
  const name = `tomo-${cmd.name}`;
  if (!/^tomo-[a-z]+(?:-[a-z]+)*$/.test(name)) {
    throw new Error(`tomo: Copilot skill name "${name}" must be lowercase and hyphenated`);
  }
  const shim =
    `The user invoked this skill as \`/${name}\`; text typed after the command name is its arguments. ` +
    `Treat that text as \`$ARGUMENTS\` in the body below (absent text means no arguments were provided).`;
  return `---\nname: ${name}\ndescription: ${cmd.fm.description}\n---\n\n${shim}\n\n${cmd.body}\n`;
}


// Each entry: where the harness's command wrappers and agent (persona) files
// live (project + user scope), the output filenames for each kind, and the cast
// renderer (canonical persona object → full harness agent file content).
const PROVIDERS = {
  opencode: {
    id: 'opencode',
    label: 'OpenCode',
    projectCommandDir: (project) => path.join(project, '.opencode', 'command'),
    globalCommandDir: () => path.join(configBase(), 'opencode', 'command'),
    projectAgentDir: (project) => path.join(project, '.opencode', 'agents'),
    globalAgentDir: () => path.join(configBase(), 'opencode', 'agents'),
    // opencode's global config dir — a global install seeds an
    // external_directory grant for the tomo config dir into its config file.
    globalConfigDir: () => path.join(configBase(), 'opencode'),
    commandFilename: (name) => `tomo-${name}.md`,
    filename: (name) => `${name}.md`,
    cast: castOpenCode,
  },
  claude: {
    id: 'claude',
    label: 'Claude Code',
    projectCommandDir: (project) => path.join(project, '.claude', 'commands', 'tomo'),
    globalCommandDir: () => path.join(os.homedir(), '.claude', 'commands', 'tomo'),
    projectAgentDir: (project) => path.join(project, '.claude', 'agents'),
    globalAgentDir: () => path.join(os.homedir(), '.claude', 'agents'),
    commandFilename: (name) => `${name}.md`,
    filename: (name) => `${name}.md`,
    commandCast: castClaudeCommand,
    cast: castClaude,
  },
  copilot: {
    id: 'copilot',
    label: 'GitHub Copilot',
    projectCommandDir: (project) => path.join(project, '.github', 'skills'),
    globalCommandDir: () => path.join(os.homedir(), '.copilot', 'skills'),
    projectAgentDir: (project) => path.join(project, '.github', 'agents'),
    globalAgentDir: () => path.join(os.homedir(), '.copilot', 'agents'),
    commandFilename: (name) => `tomo-${name}/SKILL.md`,
    filename: (name) => `${name}.agent.md`,
    commandCast: castCopilotCommand,
    cast: castCopilot,
  },
  antigravity: {
    id: 'antigravity',
    label: 'Google Antigravity',
    // Skills and agents do not live under one root: commands are Skills at
    // ~/.gemini/antigravity-cli/skills/ while personas are Custom Agents at
    // ~/.gemini/config/agents/ (see the docs on subagents vs. skills).
    projectCommandDir: (project) => path.join(project, '.agents', 'skills'),
    globalCommandDir: () => path.join(os.homedir(), '.gemini', 'antigravity-cli', 'skills'),
    projectAgentDir: (project) => path.join(project, '.agents', 'agents'),
    globalAgentDir: () => path.join(os.homedir(), '.gemini', 'config', 'agents'),
    // Antigravity's own settings file — a global install seeds a read_file
    // grant here so the tomo config dir (outside any project workspace) is
    // readable without a per-file approval prompt.
    settingsPath: () => path.join(os.homedir(), '.gemini', 'antigravity-cli', 'settings.json'),
    commandFilename: (name) => `tomo-${name}/SKILL.md`,
    filename: (name) => `${name}/agent.md`,
    // Skills diverge from the plain wrapper: present here, install.js renders a
    // per-provider cast instead of reusing adapters/common/tomo-<name>.md.
    commandCast: castAntigravityCommand,
    cast: castAntigravity,
  },
  codex: {
    id: 'codex',
    label: 'OpenAI Codex',
    // Commands are Skills at .agents/skills/ (the same location Antigravity
    // uses, and the same SKILL.md format, so the cast is shared verbatim);
    // personas are TOML subagents at .codex/agents/. No read grant is seeded:
    // Codex's sandbox already allows reads outside the workspace (note that
    // "read-only" still grants full disk read, unlike opencode/Antigravity).
    projectCommandDir: (project) => path.join(project, '.agents', 'skills'),
    globalCommandDir: () => path.join(os.homedir(), '.agents', 'skills'),
    projectAgentDir: (project) => path.join(project, '.codex', 'agents'),
    globalAgentDir: () => path.join(os.homedir(), '.codex', 'agents'),
    commandFilename: (name) => `tomo-${name}/SKILL.md`,
    filename: (name) => `${name}.toml`,
    commandCast: castAntigravityCommand,
    cast: castCodex,
  },
};

function providerIds() {
  return Object.keys(PROVIDERS);
}

function isProvider(id) {
  return Object.prototype.hasOwnProperty.call(PROVIDERS, id);
}

// Parse a --provider spec: 'all'|'' → every provider; comma-separated ids.
// Unknown ids throw, so a typo fails loudly instead of silently skipping.
function parseProviderSpec(spec) {
  const raw = String(spec == null ? '' : spec).trim();
  if (!raw || raw === 'all') return providerIds();
  const ids = raw.split(',').map((s) => s.trim()).filter(Boolean);
  if (!ids.length) throw new Error('tomo: --provider needs a value: all | <id>[,<id>...]');
  for (const id of ids) {
    if (!isProvider(id)) {
      throw new Error(`tomo: unknown provider "${id}" (have: ${providerIds().join(', ')})`);
    }
  }
  return ids;
}

// Seed the effective harness tag into a project's .tomo/settings.json, keeping
// any other keys. Only a single unambiguous provider is written; when several
// are installed the tag is left as-is so reruns stay idempotent.
function seedSettingsProvider(project, providers) {
  const settingsPath = path.join(project, '.tomo', 'settings.json');
  let current = {};
  if (fs.existsSync(settingsPath)) {
    try {
      current = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
    } catch (e) {
      throw new Error(`tomo: cannot parse ${settingsPath}: ${e.message}`);
    }
  }
  const next = providers.length === 1 ? providers[0] : current.provider || '';
  if (next === (current.provider || '')) return { changed: false, settingsPath };
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true });
  fs.writeFileSync(settingsPath, JSON.stringify({ ...current, provider: next }, null, 2) + '\n');
  return { changed: true, settingsPath };
}

module.exports = {
  READONLY_PERSONAS,
  tomoConfigDir,
  homeRelative,
  claudeReadGrant,
  PROVIDERS,
  providerIds,
  isProvider,
  parseProviderSpec,
  seedSettingsProvider,
};