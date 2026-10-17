const { test, after } = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const render = require('../scripts/render.js');
const installer = require('../scripts/install.js');
const providers = require('../src/lib/providers.js');

const ROOT = render.ROOT;
const TMP = fs.mkdtempSync(path.join(os.tmpdir(), 'tomo-install-'));

after(() => {
  fs.rmSync(TMP, { recursive: true, force: true });
});

function snapshot(dir, ids) {
  const out = [];
  for (const p of installer.copyPairs(dir, ids || providers.providerIds())) {
    const rel = path.relative(dir, p[1]);
    out.push([rel, fs.readFileSync(p[1], 'utf8')]);
  }
  return out;
}

// The canonical seed list for a project install under the chosen providers.
function expectedProject(project, ids) {
  const list = [
    ...render.readCommands().flatMap((cmd) =>
      ids
        .filter((id) => providers.PROVIDERS[id].commands !== false)
        .map((id) =>
          path.join(providers.PROVIDERS[id].projectCommandDir(project), providers.PROVIDERS[id].commandFilename(cmd.name))),
    ),
    ...['state.js', 'settings.js', 'issues.js', 'personas.js'].map((lib) =>
      path.join(project, '.tomo', 'lib', lib),
    ),
    path.join(project, '.tomo', 'templates', 'requirements.md'),
    path.join(project, '.tomo', 'templates', 'design.md'),
    ...render.readPersonas().flatMap((p) => [
      ...ids.map((id) => path.join(providers.PROVIDERS[id].projectAgentDir(project), providers.PROVIDERS[id].filename(p.name))),
      path.join(project, '.tomo', 'personas', `${p.name}.md`),
    ]),
  ];
  if (ids.length === 1) list.push(path.join(project, '.tomo', 'settings.json'));
  return list;
}

test('per-project install copies wrappers and seeds .tomo/ for all providers', () => {
  const project = path.join(TMP, 'proj');
  const ids = providers.providerIds();
  const written = installer.install({ project });
  assert.deepStrictEqual([...written].sort(), expectedProject(project, ids).sort());

  for (const cmd of render.readCommands()) {
    for (const id of ids.filter((x) => providers.PROVIDERS[x].commands !== false)) {
      const prov = providers.PROVIDERS[id];
      const dest = path.join(prov.projectCommandDir(project), prov.commandFilename(cmd.name));
      assert.strictEqual(fs.readFileSync(dest, 'utf8'), fs.readFileSync(render.providerCommandPath(cmd.name, id), 'utf8'));
    }
  }

  assert.ok(!fs.existsSync(path.join(project, '.tomo', 'commands')), 'bodies are inlined; no runtime content store');
  assert.ok(!fs.existsSync(path.join(project, '.tomo', 'gate.md')), 'gate is inlined; no runtime gate file');

  for (const p of render.readPersonas()) {
    for (const id of ids) {
      const prov = providers.PROVIDERS[id];
      const dest = path.join(prov.projectAgentDir(project), prov.filename(p.name));
      assert.strictEqual(fs.readFileSync(dest, 'utf8'), fs.readFileSync(render.personaCastPath(p.name, id), 'utf8'));
    }
    const neutral = path.join(project, '.tomo', 'personas', `${p.name}.md`);
    const canonical = fs.readFileSync(path.join(render.ROOT, 'src', 'personas', `${p.name}.md`), 'utf8');
    assert.strictEqual(fs.readFileSync(neutral, 'utf8'), canonical);
  }

  const installedLib = require(path.join(project, '.tomo', 'lib', 'state.js'));
  assert.ok(fs.existsSync(path.join(project, '.tomo', 'lib', 'personas.js')), 'personas.js helper ships');
  const st = installedLib.defaultState();
  assert.strictEqual(typeof installedLib.setGate, 'function');
  assert.strictEqual(st.status, '');
  assert.strictEqual(st.gates.requirements, '');
});

test('per-project install with one provider ships only that harness commands and personas', () => {
  for (const id of providers.providerIds()) {
    const project = path.join(TMP, `proj-${id}`);
    const written = installer.install({ project, providers: [id] });
    assert.deepStrictEqual([...written].sort(), expectedProject(project, [id]).sort());

    const prov = providers.PROVIDERS[id];
    const cmd = render.readCommands()[0];
    const wrapper = path.join(prov.projectCommandDir(project), prov.commandFilename(cmd.name));
    if (prov.commands === false) {
      assert.ok(!fs.existsSync(wrapper), `${id}: no command wrapper should exist`);
      assert.ok(!fs.existsSync(prov.projectCommandDir(project)), `${id}: command dir should not exist`);
    } else {
      assert.ok(fs.existsSync(wrapper), `${id}: own command wrapper should exist`);
    }
    assert.ok(fs.existsSync(path.join(prov.projectAgentDir(project), prov.filename('coder'))), `${id}: own persona cast should exist`);
    for (const otherId of providers.providerIds().filter((x) => x !== id)) {
      const other = providers.PROVIDERS[otherId];
      if (other.projectCommandDir(project) !== prov.projectCommandDir(project)) {
        assert.ok(!fs.existsSync(other.projectCommandDir(project)), `${id} install must not create ${otherId} command dir`);
      }
      assert.ok(!fs.existsSync(path.join(other.projectAgentDir(project), other.filename('coder'))), `${id} install must not materialize ${otherId} persona casts`);
    }

    // Single-provider install records the effective harness in settings.
    const settings = JSON.parse(fs.readFileSync(path.join(project, '.tomo', 'settings.json'), 'utf8'));
    assert.strictEqual(settings.provider, id);
  }
});

test('seeding provider keeps existing settings keys', () => {
  const project = path.join(TMP, 'seeded');
  const settingsPath = path.join(project, '.tomo', 'settings.json');
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true });
  fs.writeFileSync(settingsPath, JSON.stringify({ gates: 'prompt', personas: { map: { plan: 'security' } } }, null, 2));
  installer.install({ project, providers: ['claude'] });
  const settings = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
  assert.strictEqual(settings.provider, 'claude');
  assert.strictEqual(settings.gates, 'prompt');
  assert.deepStrictEqual(settings.personas, { map: { plan: 'security' } });
});

test('re-running install is idempotent', () => {
  const project = path.join(TMP, 'proj');
  installer.install({ project });
  const before = snapshot(project);
  installer.install({ project });
  const after = snapshot(project);
  assert.deepStrictEqual(after, before, 'second install must not change installed files');
});

test('global (user-level) paths live in each harness user dir', () => {
  assert.ok(installer.globalOpenCodeDir().endsWith(path.join('opencode', 'command')), 'opencode global dir');
  assert.ok(installer.globalClaudeDir().endsWith(path.join('commands', 'tomo')), 'claude global dir must nest under tomo/');
});

test('global install symlinks every wrapper into all harness dirs', () => {
  const gd = {
    opencode: path.join(TMP, 'g', 'opencode', 'command'),
    claude: path.join(TMP, 'g', 'claude', 'commands', 'tomo'),
    copilot: path.join(TMP, 'g', 'copilot', 'skills'),
    antigravity: path.join(TMP, 'g', 'agy', 'skills'),
    codex: path.join(TMP, 'g', 'codex', 'skills'),
    opencodeAgents: path.join(TMP, 'g', 'opencode', 'agents'),
    claudeAgents: path.join(TMP, 'g', 'claude', 'agents'),
    copilotAgents: path.join(TMP, 'g', 'copilot', 'agents'),
    antigravityAgents: path.join(TMP, 'g', 'agy', 'agents'),
    codexAgents: path.join(TMP, 'g', 'codex', 'agents'),
    antigravitySettings: path.join(TMP, 'g', 'agy', 'settings.json'),
    opencodeConfig: path.join(TMP, 'g', 'opencode'),
    tomo: path.join(TMP, 'g', 'tomo'),
  };
  // A stale content-store link from a pre-inlining install is retired.
  fs.mkdirSync(gd.tomo, { recursive: true });
  fs.symlinkSync(render.CONTENT_DIR, path.join(gd.tomo, 'content'), 'dir');
  const written = installer.install({ project: TMP, global: true, globalDirs: gd });
  const commands = render.readCommands();
  const personas = render.readPersonas();
  const ids = providers.providerIds();
  const cmdIds = ids.filter((id) => providers.PROVIDERS[id].commands !== false);
  assert.strictEqual(written.length, 2 + commands.length * cmdIds.length + personas.length * ids.length, 'opencode + antigravity grants + one link per wrapper (command-capable harnesses) and persona per harness');

  assert.ok(!fs.existsSync(path.join(gd.tomo, 'content')), 'stale content-store link must be removed');

  const agySettings = JSON.parse(fs.readFileSync(gd.antigravitySettings, 'utf8'));
  assert.deepStrictEqual(agySettings.permissions.allow, [`read_file(${gd.tomo})`],
    'global antigravity install seeds a read grant for the tomo config dir');
  const ocConfig = JSON.parse(fs.readFileSync(path.join(gd.opencodeConfig, 'opencode.json'), 'utf8'));
  assert.deepStrictEqual(ocConfig.permission.external_directory, { [`${providers.homeRelative(gd.tomo)}/**`]: 'allow' },
    'global opencode install seeds an external_directory grant for the tomo config dir');

  for (const cmd of commands) {
    for (const id of cmdIds) {
      const prov = providers.PROVIDERS[id];
      const link = path.join(gd[id], prov.commandFilename(cmd.name));
      assert.strictEqual(fs.lstatSync(link).isSymbolicLink(), true, `${link} must be a symlink`);
      assert.strictEqual(fs.realpathSync(link), render.providerCommandPath(cmd.name, id), `${link} must point at its provider command cast`);
    }
  }

  for (const p of personas) {
    for (const id of ids) {
      const prov = providers.PROVIDERS[id];
      const link = path.join(gd[`${id}Agents`], prov.filename(p.name));
      assert.strictEqual(fs.lstatSync(link).isSymbolicLink(), true, `${link} must be a symlink`);
      assert.strictEqual(fs.realpathSync(link), render.personaCastPath(p.name, id), `${link} must point at the persona cast`);
    }
  }
});

test('global antigravity install seeds a read grant, preserving existing settings', () => {
  const gd = {
    antigravity: path.join(TMP, 'ga', 'agy', 'skills'),
    antigravityAgents: path.join(TMP, 'ga', 'agy', 'agents'),
    antigravitySettings: path.join(TMP, 'ga', 'agy', 'settings.json'),
    tomo: path.join(TMP, 'ga', 'tomo'),
  };
  fs.mkdirSync(path.dirname(gd.antigravitySettings), { recursive: true });
  fs.writeFileSync(
    gd.antigravitySettings,
    JSON.stringify({ toolPermission: 'request-review', permissions: { deny: ['command(rm -rf)'] } }, null, 2),
  );
  installer.install({ project: TMP, global: true, globalDirs: gd, providers: ['antigravity'] });
  installer.install({ project: TMP, global: true, globalDirs: gd, providers: ['antigravity'] });

  const settings = JSON.parse(fs.readFileSync(gd.antigravitySettings, 'utf8'));
  assert.deepStrictEqual(settings.permissions.allow, [`read_file(${gd.tomo})`], 'grant added exactly once');
  assert.deepStrictEqual(settings.permissions.deny, ['command(rm -rf)'], 'existing deny rules survive');
  assert.strictEqual(settings.toolPermission, 'request-review', 'existing keys survive');
});

test('antigravity grant seeding creates the settings file when absent', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'tomo-grant-'));
  try {
    const settingsPath = path.join(root, 'settings.json');
    const tomoDir = path.join(root, 'tomo');
    const first = installer.seedAntigravityGrants({ settingsPath, tomoDir });
    const second = installer.seedAntigravityGrants({ settingsPath, tomoDir });
    assert.strictEqual(first.changed, true);
    assert.strictEqual(second.changed, false, 'repeat seeding is a no-op');
    assert.deepStrictEqual(JSON.parse(fs.readFileSync(settingsPath, 'utf8')), {
      permissions: { allow: [`read_file(${tomoDir})`] },
    });
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test('global install can target a single provider', () => {
  const gd = {
    opencode: path.join(TMP, 'go', 'opencode', 'command'),
    claude: path.join(TMP, 'go', 'claude', 'commands', 'tomo'),
    opencodeAgents: path.join(TMP, 'go', 'opencode', 'agents'),
    claudeAgents: path.join(TMP, 'go', 'claude', 'agents'),
    opencodeConfig: path.join(TMP, 'go', 'opencode'),
    tomo: path.join(TMP, 'go', 'tomo'),
  };
  const written = installer.install({ project: TMP, global: true, globalDirs: gd, providers: ['opencode'] });
  const commands = render.readCommands();
  const personas = render.readPersonas();
  assert.strictEqual(written.length, 1 + commands.length + personas.length, 'opencode grant + wrappers + personas');

  const cmd = commands[0];
  const prov = providers.PROVIDERS.opencode;
  assert.strictEqual(fs.lstatSync(path.join(gd.opencode, prov.commandFilename(cmd.name))).isSymbolicLink(), true);
  assert.strictEqual(fs.lstatSync(path.join(gd.opencodeAgents, 'coder.md')).isSymbolicLink(), true);
  assert.ok(!fs.existsSync(gd.claude), 'claude command dir must not be created');
  assert.ok(!fs.existsSync(gd.claudeAgents), 'claude agent dir must not be created');
});

test('global agent dirs live in each harness user dir', () => {
  assert.ok(installer.globalOpenCodeAgentsDir().endsWith(path.join('opencode', 'agents')), 'opencode global agents dir');
  assert.ok(installer.globalClaudeAgentsDir().endsWith(path.join('.claude', 'agents')), 'claude global agents dir');
});

test('copilot install ships skill wrappers and agent casts', () => {
  const project = path.join(TMP, 'proj-copilot-only');
  const written = installer.install({ project, providers: ['copilot'] });
  assert.deepStrictEqual([...written].sort(), expectedProject(project, ['copilot']).sort());

  const prov = providers.PROVIDERS.copilot;
  for (const cmd of render.readCommands()) {
    const skill = path.join(prov.projectCommandDir(project), prov.commandFilename(cmd.name));
    assert.strictEqual(fs.readFileSync(skill, 'utf8'), fs.readFileSync(render.providerCommandPath(cmd.name, 'copilot'), 'utf8'));
    assert.match(fs.readFileSync(skill, 'utf8'), new RegExp(`^---\\nname: tomo-${cmd.name}\\n`));
  }
  for (const p of render.readPersonas()) {
    const cast = path.join(prov.projectAgentDir(project), prov.filename(p.name));
    assert.strictEqual(fs.readFileSync(cast, 'utf8'), fs.readFileSync(render.personaCastPath(p.name, 'copilot'), 'utf8'));
  }

  const settings = JSON.parse(fs.readFileSync(path.join(project, '.tomo', 'settings.json'), 'utf8'));
  assert.strictEqual(settings.provider, 'copilot');

  const gd = {
    copilot: path.join(TMP, 'gc', 'copilot', 'skills'),
    copilotAgents: path.join(TMP, 'gc', 'copilot', 'agents'),
    tomo: path.join(TMP, 'gc', 'tomo'),
  };
  const globalWritten = installer.install({ project: TMP, global: true, globalDirs: gd, providers: ['copilot'] });
  assert.strictEqual(globalWritten.length, render.readCommands().length + render.readPersonas().length,
    'copilot skills + agent casts');

  for (const cmd of render.readCommands()) {
    const link = path.join(gd.copilot, prov.commandFilename(cmd.name));
    assert.strictEqual(fs.lstatSync(link).isSymbolicLink(), true, `${link} must be a symlink`);
    assert.strictEqual(fs.realpathSync(link), render.providerCommandPath(cmd.name, 'copilot'), `${link} must point at the Copilot skill cast`);
  }
  for (const p of render.readPersonas()) {
    const link = path.join(gd.copilotAgents, prov.filename(p.name));
    assert.strictEqual(fs.lstatSync(link).isSymbolicLink(), true, `${link} must be a symlink`);
    assert.strictEqual(fs.realpathSync(link), render.personaCastPath(p.name, 'copilot'), `${link} must point at the copilot persona cast`);
  }
});

test('opencode grant seeding merges JSONC, stays idempotent, and never drops comments', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'tomo-oc-'));
  try {
    const tomoDir = path.join(os.homedir(), '.config', 'tomo');
    const pattern = '~/.config/tomo/**';

    // Created when absent.
    const fresh = path.join(root, 'fresh');
    const a = installer.seedOpenCodeGrants({ configDir: fresh, tomoDir });
    assert.strictEqual(a.changed, true);
    assert.strictEqual(a.pattern, pattern);
    assert.strictEqual(installer.seedOpenCodeGrants({ configDir: fresh, tomoDir }).changed, false, 'idempotent');

    // Trailing commas are tolerated; existing rules keep their order and the grant lands last.
    const trailing = path.join(root, 'trailing');
    fs.mkdirSync(trailing);
    fs.writeFileSync(path.join(trailing, 'opencode.json'),
      '{\n  "$schema": "https://opencode.ai/config.json",\n  "permission": { "external_directory": "ask", "bash": "allow", },\n}\n');
    assert.strictEqual(installer.seedOpenCodeGrants({ configDir: trailing, tomoDir }).changed, true);
    const cfg = JSON.parse(fs.readFileSync(path.join(trailing, 'opencode.json'), 'utf8'));
    assert.deepStrictEqual(cfg.permission.external_directory, { '*': 'ask', [pattern]: 'allow' });
    assert.deepStrictEqual(Object.keys(cfg.permission.external_directory), ['*', pattern]);
    assert.strictEqual(cfg.permission.bash, 'allow');

    // Comments: never rewritten; caller gets a snippet.
    const commented = path.join(root, 'commented');
    fs.mkdirSync(commented);
    const src = '{\n  // keep me\n  "$schema": "https://opencode.ai/config.json"\n}\n';
    fs.writeFileSync(path.join(commented, 'opencode.jsonc'), src);
    const c = installer.seedOpenCodeGrants({ configDir: commented, tomoDir });
    assert.strictEqual(c.changed, false);
    assert.strictEqual(c.manual, true);
    assert.ok(c.snippet.includes(pattern));
    assert.strictEqual(fs.readFileSync(path.join(commented, 'opencode.jsonc'), 'utf8'), src);

    // Strings containing comment/comma lookalikes survive parsing.
    assert.deepStrictEqual(installer.parseJsonc('{"a": "x // y, }", "b": [1,],}').value, { a: 'x // y, }', b: [1] });
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
