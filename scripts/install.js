#!/usr/bin/env node
// Install Tomo into a project (or into harness-global command dirs).
//
//   node scripts/install.js --help            show usage
//   node scripts/install.js [--project <dir>] per-project install (default: cwd)
//   node scripts/install.js --global          symlink wrappers into user dirs
//
// Provider targeting: on a TTY the installer asks which harness(es) you use and
// materializes that provider's command wrappers and persona casts into that
// harness's dirs; a --provider flag (or a non-TTY run) defaults to all. Adding
// a new harness is one entry in src/lib/providers.js.
//
// Build is folded into install: the first thing every install does is render
// the wrappers and persona casts (scripts/render.js) into the gitignored
// adapters/common/ cache, then it places those files — copy for a project,
// symlink for a global install. Nothing generated is ever committed.
//
// Per-project install copies the fresh wrappers into each chosen provider's
// command dir (e.g. <project>/.opencode/command/tomo-<name>.md or
// <project>/.github/skills/tomo-<name>/SKILL.md) and seeds <project>/.tomo/
// with lib + templates + personas. The wrappers carry their canonical body
// inline, so no command reads a body file at runtime.
// --global instead symlinks the wrappers from the cache into your user-level
// harness command dirs, making the /tomo commands available in every project:
//   $XDG_CONFIG_HOME/opencode/command/tomo-<name>.md  (default ~/.config/opencode)
//   ~/.claude/commands/tomo/<name>.md
// and seeds read grants for the tomo config dir (global settings.json lives
// there, outside every project) into opencode's and Antigravity's configs.
// Global symlinks point into this checkout's cache, so keep it where you
// cloned it and re-run install after editing content/.
//
// Idempotent: re-running overwrites in place with identical bytes.
// Zero runtime dependencies; works with or without node_modules.

const fs = require('fs');
const os = require('os');
const path = require('path');
const readline = require('readline/promises');
const render = require('./render.js');
const providers = require('../src/lib/providers.js');

// The tomo config dir (global settings.json) — outside every project, so it
// is what the harness read grants cover.
function globalTomoDir() {
  return providers.tomoConfigDir();
}

function globalOpenCodeDir() {
  return providers.PROVIDERS.opencode.globalCommandDir();
}

function globalClaudeDir() {
  return providers.PROVIDERS.claude.globalCommandDir();
}

function globalOpenCodeAgentsDir() {
  return providers.PROVIDERS.opencode.globalAgentDir();
}

function globalClaudeAgentsDir() {
  return providers.PROVIDERS.claude.globalAgentDir();
}

function projectOpenCodeDir(project) {
  return providers.PROVIDERS.opencode.projectCommandDir(project);
}

function projectClaudeDir(project) {
  return providers.PROVIDERS.claude.projectCommandDir(project);
}

function projectOpenCodeAgentsDir(project) {
  return providers.PROVIDERS.opencode.projectAgentDir(project);
}

function projectClaudeAgentsDir(project) {
  return providers.PROVIDERS.claude.projectAgentDir(project);
}

// [source, destination] pairs for a per-project install. Both command wrappers
// and persona casts are materialized only for the chosen providers; the neutral
// .tomo/personas/ copies always ship (lens adoption is provider-free).
function copyPairs(project, selectedProviders) {
  const ids = selectedProviders && selectedProviders.length ? selectedProviders : providers.providerIds();
  const pairs = [];
  for (const cmd of render.readCommands()) {
    for (const id of ids) {
      const prov = providers.PROVIDERS[id];
      if (prov.commands === false) continue;
      pairs.push([render.providerCommandPath(cmd.name, id), path.join(prov.projectCommandDir(project), prov.commandFilename(cmd.name))]);
    }
  }
  for (const lib of ['state.js', 'settings.js', 'issues.js', 'personas.js']) {
    pairs.push([
      path.join(render.ROOT, 'src', 'lib', lib),
      path.join(project, '.tomo', 'lib', lib),
    ]);
  }
  for (const t of fs.readdirSync(path.join(render.ROOT, 'src', 'templates')).sort()) {
    pairs.push([
      path.join(render.ROOT, 'src', 'templates', t),
      path.join(project, '.tomo', 'templates', t),
    ]);
  }
  for (const p of render.readPersonas()) {
    for (const id of ids) {
      const prov = providers.PROVIDERS[id];
      pairs.push([
        render.personaCastPath(p.name, prov.id),
        path.join(prov.projectAgentDir(project), prov.filename(p.name)),
      ]);
    }
    // Neutral copy for lens adoption (.tomo/personas/<name>.md).
    pairs.push([
      path.join(render.ROOT, 'src', 'personas', `${p.name}.md`),
      path.join(project, '.tomo', 'personas', `${p.name}.md`),
    ]);
  }
  return pairs;
}

// Resolve each provider's target dirs for a global install, honoring injected
// test dirs (globalDirs[<id>] for commands, globalDirs[<id>Agents] for agents).
function globalDirMap(globalDirs, kind) {
  const suffix = kind === 'agents' ? 'Agents' : '';
  const out = {};
  for (const id of providers.providerIds()) {
    const prov = providers.PROVIDERS[id];
    const fallback = kind === 'agents' ? prov.globalAgentDir() : prov.globalCommandDir();
    out[id] = (globalDirs && globalDirs[`${id}${suffix}`]) || fallback;
  }
  return out;
}

// Antigravity only auto-allows reads inside the active project workspace, so
// reads of the tomo config dir (global settings.json) would prompt for
// approval. Seed a merge-aware read_file grant for it into the CLI's
// settings.json so those non-workspace reads run silently. Idempotent:
// re-running leaves identical bytes and other keys (toolPermission, deny/ask
// rules, ...) are preserved.
function seedAntigravityGrants({ settingsPath, tomoDir }) {
  const grant = `read_file(${tomoDir})`;
  let settings = {};
  if (fs.existsSync(settingsPath)) {
    try {
      settings = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
    } catch (e) {
      throw new Error(`tomo: cannot parse ${settingsPath}: ${e.message}`);
    }
    if (settings === null || typeof settings !== 'object' || Array.isArray(settings)) {
      throw new Error(`tomo: antigravity settings must be a JSON object: ${settingsPath}`);
    }
  }
  const perms = (settings.permissions = settings.permissions || {});
  perms.allow = perms.allow && Array.isArray(perms.allow) ? [...perms.allow] : [];
  if (perms.allow.includes(grant)) return { grant, path: settingsPath, changed: false };
  perms.allow.push(grant);
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true });
  fs.writeFileSync(settingsPath, JSON.stringify(settings, null, 2) + '\n');
  return { grant, path: settingsPath, changed: true };
}

// Parse opencode's JSON/JSONC config without dependencies: strip // and /* */
// comments and trailing commas outside strings. Reports whether comments were
// present, because rewriting the file would silently drop them.
function parseJsonc(src) {
  let out = '';
  let hadComments = false;
  for (let i = 0; i < src.length; i++) {
    const c = src[i];
    if (c === '"') {
      let j = i + 1;
      while (j < src.length && src[j] !== '"') j += src[j] === '\\' ? 2 : 1;
      out += src.slice(i, j + 1);
      i = j;
    } else if (c === '/' && src[i + 1] === '/') {
      hadComments = true;
      while (i < src.length && src[i] !== '\n') i++;
      out += '\n';
    } else if (c === '/' && src[i + 1] === '*') {
      hadComments = true;
      const end = src.indexOf('*/', i + 2);
      i = end === -1 ? src.length : end + 1;
    } else {
      out += c;
    }
  }
  // Trailing commas: a comma followed only by whitespace before } or ].
  let clean = '';
  for (let i = 0; i < out.length; i++) {
    const c = out[i];
    if (c === '"') {
      let j = i + 1;
      while (j < out.length && out[j] !== '"') j += out[j] === '\\' ? 2 : 1;
      clean += out.slice(i, j + 1);
      i = j;
    } else if (c === ',' && /^\s*[}\]]/.test(out.slice(i + 1))) {
      // drop it
    } else {
      clean += c;
    }
  }
  return { value: JSON.parse(clean), hadComments };
}

// opencode asks before any tool touches a path outside the project
// (`external_directory` defaults to "ask"), and "always" only lasts a session.
// Seed an allow rule for the tomo config dir into the global opencode config:
// opencode.json if present, else opencode.jsonc, else a new opencode.json.
// The rule is appended last because opencode's last matching rule wins. A file
// with comments is never rewritten (that would drop them); the caller gets the
// snippet to paste instead. Idempotent.
function seedOpenCodeGrants({ configDir, tomoDir }) {
  const pattern = `${providers.homeRelative(tomoDir)}/**`;
  const snippet = JSON.stringify({ permission: { external_directory: { [pattern]: 'allow' } } }, null, 2);
  const candidates = ['opencode.json', 'opencode.jsonc'].map((f) => path.join(configDir, f));
  const configPath = candidates.find((f) => fs.existsSync(f)) || candidates[0];
  let config = { $schema: 'https://opencode.ai/config.json' };
  if (fs.existsSync(configPath)) {
    let parsed;
    try {
      parsed = parseJsonc(fs.readFileSync(configPath, 'utf8'));
    } catch (e) {
      return { pattern, path: configPath, changed: false, manual: true, snippet, reason: `cannot parse: ${e.message}` };
    }
    if (parsed.value === null || typeof parsed.value !== 'object' || Array.isArray(parsed.value)) {
      return { pattern, path: configPath, changed: false, manual: true, snippet, reason: 'config is not a JSON object' };
    }
    config = parsed.value;
    const covered = granted(config.permission, pattern);
    if (covered) return { pattern, path: configPath, changed: false };
    if (parsed.hadComments) {
      return { pattern, path: configPath, changed: false, manual: true, snippet, reason: 'file has comments' };
    }
  }
  let perm = config.permission;
  if (perm === undefined) perm = {};
  if (typeof perm === 'string') perm = { '*': perm };
  if (perm === null || typeof perm !== 'object' || Array.isArray(perm)) {
    return { pattern, path: configPath, changed: false, manual: true, snippet, reason: 'unexpected permission shape' };
  }
  let ext = perm.external_directory;
  if (ext === undefined) ext = {};
  if (typeof ext === 'string') ext = { '*': ext };
  ext = { ...ext };
  delete ext[pattern];
  ext[pattern] = 'allow';
  config.permission = { ...perm, external_directory: ext };
  fs.mkdirSync(configDir, { recursive: true });
  fs.writeFileSync(configPath, JSON.stringify(config, null, 2) + '\n');
  return { pattern, path: configPath, changed: true };
}

// Whether an opencode permission block already allows the pattern (an exact
// allow rule, or a blanket "allow" for everything / external_directory).
function granted(perm, pattern) {
  if (perm === 'allow') return true;
  if (!perm || typeof perm !== 'object') return false;
  const ext = perm.external_directory;
  if (ext === 'allow') return true;
  if (ext === undefined && perm['*'] === 'allow') return true;
  if (!ext || typeof ext !== 'object') return false;
  const keys = Object.keys(ext);
  return ext[pattern] === 'allow' && keys[keys.length - 1] === pattern;
}

function install({ project = process.cwd(), global = false, globalDirs, providers: selected, notices = [] } = {}) {
  // Fold the build into install: always regenerate the gitignored
  // adapters/common/ cache before placing files, so the wrappers and casts an
  // install symlinks/copies are fresh from the content store + command
  // contracts no matter when the checkout last rendered.
  render.writeAll();

  const ids = selected && selected.length ? selected : providers.providerIds();

  const written = [];
  if (global) {
    const cmdDirs = globalDirMap(globalDirs, 'commands');
    const agentDirs = globalDirMap(globalDirs, 'agents');
    // Earlier installs linked this checkout's content/ into the tomo config
    // dir for pointer wrappers to resolve at runtime. Wrappers now inline
    // their bodies, so retire that link (only ever a symlink we created).
    const tomoDir = (globalDirs && globalDirs.tomo) || globalTomoDir();
    const staleContent = path.join(tomoDir, 'content');
    try {
      if (fs.lstatSync(staleContent).isSymbolicLink()) fs.unlinkSync(staleContent);
    } catch (e) {
      // not present — fine
    }
    if (ids.includes('opencode')) {
      const configDir = (globalDirs && globalDirs.opencodeConfig) || providers.PROVIDERS.opencode.globalConfigDir();
      const seeded = seedOpenCodeGrants({ configDir, tomoDir });
      if (seeded.changed) written.push(seeded.path);
      if (seeded.manual) notices.push(seeded);
    }
    if (ids.includes('antigravity')) {
      const settingsPath =
        (globalDirs && globalDirs.antigravitySettings) || providers.PROVIDERS.antigravity.settingsPath();
      const seeded = seedAntigravityGrants({ settingsPath, tomoDir });
      if (seeded.changed) written.push(seeded.path);
    }
    for (const id of ids) {
      if (providers.PROVIDERS[id].commands !== false) fs.mkdirSync(cmdDirs[id], { recursive: true });
      fs.mkdirSync(agentDirs[id], { recursive: true });
    }
    for (const cmd of render.readCommands()) {
      for (const id of ids) {
        const prov = providers.PROVIDERS[id];
        if (prov.commands === false) continue;
        const link = path.join(cmdDirs[id], prov.commandFilename(cmd.name));
        try {
          fs.unlinkSync(link);
        } catch (e) {
          // not present yet — fine
        }
        // Nested filenames (e.g. Antigravity role/SKILL.md) need their folder.
        fs.mkdirSync(path.dirname(link), { recursive: true });
        fs.symlinkSync(render.providerCommandPath(cmd.name, id), link);
        written.push(link);
      }
    }
    for (const p of render.readPersonas()) {
      for (const id of ids) {
        const prov = providers.PROVIDERS[id];
        const link = path.join(agentDirs[id], prov.filename(p.name));
        try {
          fs.unlinkSync(link);
        } catch (e) {
          // not present yet — fine
        }
        fs.mkdirSync(path.dirname(link), { recursive: true });
        fs.symlinkSync(render.personaCastPath(p.name, prov.id), link);
        written.push(link);
      }
    }
  } else {
    for (const [src, dest] of copyPairs(project, ids)) {
      fs.mkdirSync(path.dirname(dest), { recursive: true });
      fs.copyFileSync(src, dest);
      written.push(dest);
    }
    const seed = providers.seedSettingsProvider(project, ids);
    if (seed.changed) written.push(seed.settingsPath);
  }
  return written;
}

module.exports = {
  install,
  globalOpenCodeDir,
  globalClaudeDir,
  globalOpenCodeAgentsDir,
  globalClaudeAgentsDir,
  projectOpenCodeDir,
  projectClaudeDir,
  projectOpenCodeAgentsDir,
  projectClaudeAgentsDir,
  globalTomoDir,
  copyPairs,
  seedAntigravityGrants,
  seedOpenCodeGrants,
  parseJsonc,
};

// Interactive harness picker: only asked on a real TTY without --provider.
async function promptProviders() {
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
  try {
    const options = providers.providerIds().join(' | ');
    const answer = await rl.question(`Which AI coding harness(es) do you use? (all | ${options}) [all]: `);
    return providers.parseProviderSpec(answer.trim() || 'all');
  } finally {
    rl.close();
  }
}

if (require.main === module) {
  const args = process.argv.slice(2);
  if (args.includes('--help') || args.includes('-h')) {
    const rel = path.relative(process.cwd(), __filename);
    const allProviders = providers.providerIds().join(' + ');
    console.log(`tomo install

  PROJECT install  copies the /tomo commands into the current project and seeds
                   ./${path.join('.tomo', '').replace(/\/$/, '')} with helpers:
      node ${rel}
      node ${rel} --project <dir>
      node ${rel} --provider opencode[,claude]   (or --provider all)

  GLOBAL install   symlinks the commands into your user-level harness dirs so
                   every project (and every new session) has /tomo:
      node ${rel} --global [--provider ...]

  PROVIDER         which harness's command wrappers and persona subagents to
                   materialize. Prompted on an interactive terminal; defaults to
                   all (${allProviders}).

  Global targets:
      opencode   ${globalOpenCodeDir()}
      Claude     ${globalClaudeDir()}
      GitHub Copilot   ${providers.PROVIDERS.copilot.globalCommandDir()}   (skills) + ${providers.PROVIDERS.copilot.globalAgentDir()}   (agents)
      Antigravity   ${providers.PROVIDERS.antigravity.globalCommandDir()}   (skills) + ${providers.PROVIDERS.antigravity.globalAgentDir()} (agents)
      Codex   ${providers.PROVIDERS.codex.globalCommandDir()}   (skills) + ${providers.PROVIDERS.codex.globalAgentDir()}   (agents)

  GLOBAL also seeds read grants for the tomo config dir (${globalTomoDir()},
  where global settings.json lives) into opencode's config
  (permission.external_directory) and Antigravity's settings.json, so those
  non-workspace reads don't prompt. Claude Code commands carry the grant in
  their allowed-tools frontmatter.

  GLOBAL only adds the slash commands — the per-project .tomo/ helpers seed a
  workspace the first time a phase runs, but a project install gives you the
  templates up front.`);
    process.exit(0);
  }
  let project = process.cwd();
  let globalFlag = false;
  let providerSpec = null;
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === '--global') globalFlag = true;
    else if (arg === '--project') project = path.resolve(args[++i]);
    else if (arg.startsWith('--provider=')) providerSpec = arg.slice('--provider='.length);
    else if (arg === '--provider') providerSpec = args[++i];
    else {
      console.error(`unknown argument: ${arg}`);
      process.exit(1);
    }
  }
  if (globalFlag && project !== process.cwd()) {
    console.error('--global and --project are mutually exclusive');
    process.exit(1);
  }
  (async () => {
    const ids = providerSpec != null
      ? providers.parseProviderSpec(providerSpec)
      : (process.stdin.isTTY && process.stdout.isTTY ? await promptProviders() : providers.providerIds());
    const label = globalFlag ? 'linked' : 'installed';
    const base = globalFlag ? require('os').homedir() : project;
    const notices = [];
    for (const p of install({ project, global: globalFlag, providers: ids, notices })) {
      console.log(label, path.relative(base, p) || path.basename(p));
    }
    const harness = ids.length === 1 ? ids[0] : `${ids.length} harnesses`;
    console.log(
      globalFlag
        ? `tomo wrappers symlinked into your user dirs for ${harness} — every project now has /tomo`
        : `tomo installed into ${project} (${harness}); .tomo/ seeded. Re-run to update.`,
    );
    for (const n of notices) {
      console.log(`
opencode: could not add the tomo read grant to ${n.path} (${n.reason}).
Add this to your opencode config so reads of ${n.pattern} don't prompt:
${n.snippet}`);
    }
    if (globalFlag && ids.includes('antigravity')) {
      console.log(`
antigravity: seeded a read_file grant for the tomo config dir in
  ${providers.PROVIDERS.antigravity.settingsPath()}
  so non-workspace reads of tomo's global settings no longer prompt.
If the SDD loop still prompts for terminal commands (find/grep/rg/test), add
read-only command grants via /permissions or that settings.json:
  command(git)  command(find)  command(grep)  command(rg)
  command(regex:npm run (build|lint|test))   (your project's test script)`);
    }
  })().catch((e) => {
    console.error(e.message);
    process.exit(1);
  });
}