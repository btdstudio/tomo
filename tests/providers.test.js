const { test } = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const render = require('../scripts/render.js');
const providers = require('../src/lib/providers.js');

test('registry exposes the five built-in providers with a complete contract', () => {
  assert.deepStrictEqual(providers.providerIds(), ['opencode', 'claude', 'copilot', 'antigravity', 'codex']);
  for (const id of providers.providerIds()) {
    const prov = providers.PROVIDERS[id];
    assert.strictEqual(prov.id, id);
    assert.ok(prov.label, `${id}: label`);
    assert.strictEqual(typeof prov.projectCommandDir, 'function');
    assert.strictEqual(typeof prov.globalCommandDir, 'function');
    assert.strictEqual(typeof prov.projectAgentDir, 'function');
    assert.strictEqual(typeof prov.globalAgentDir, 'function');
    assert.strictEqual(typeof prov.commandFilename, 'function');
    assert.strictEqual(typeof prov.filename, 'function');
    assert.strictEqual(typeof prov.cast, 'function');
    if (id === 'antigravity') {
      assert.strictEqual(prov.filename('coder'), 'coder/agent.md');
      assert.strictEqual(prov.commandFilename('requirements'), 'tomo-requirements/SKILL.md');
      assert.strictEqual(typeof prov.commandCast, 'function');
    } else if (id === 'copilot') {
      assert.strictEqual(prov.filename('coder'), 'coder.agent.md');
      assert.strictEqual(prov.commandFilename('requirements'), 'tomo-requirements/SKILL.md');
      assert.strictEqual(typeof prov.commandCast, 'function');
    } else if (id === 'codex') {
      assert.strictEqual(prov.filename('coder'), 'coder.toml');
      assert.strictEqual(prov.commandFilename('requirements'), 'tomo-requirements/SKILL.md');
      assert.strictEqual(typeof prov.commandCast, 'function');
    } else {
      assert.strictEqual(prov.filename('coder'), 'coder.md');
    }
  }
  assert.strictEqual(providers.isProvider('antigravity'), true);
  assert.strictEqual(providers.isProvider('gemini'), false);
});

test('skill and subagent casts use valid provider formats', () => {
  for (const p of render.readPersonas()) {
    const content = providers.PROVIDERS.antigravity.cast(p);
    assert.match(content, /^---\nname: [^\n]+\ndescription: [^\n]+\nsubagent: true\nmainAgent: false\n/s, `${p.name}: subagent frontmatter`);
    const readonly = ['reviewer', 'security'].includes(p.name);
    assert.strictEqual(content.includes('tools:\n  - view_file\n  - grep_search\n'), readonly, `${p.name}: read-only casts carry read-only tools`);
    if (!readonly) assert.ok(!/^\s*tools:/.test(content), `${p.name}: antigravity cast must not carry a tools key`);
    const names = (content.match(/^\s*-\s+([a-z_]+)\s*$/gm) || []).map((m) => m.trim().slice(2));
    for (const tool of names) {
      assert.ok(['view_file', 'grep_search', 'run_command', 'replace_file_content'].includes(tool),
        `${p.name}: tool "${tool}" is not a documented Antigravity tool name`);
    }
  }
  for (const cmd of render.readCommands().map(render.inlineCommand)) {
    const cast = providers.PROVIDERS.antigravity.commandCast(cmd);
    assert.match(cast, new RegExp(`^---\\nname: tomo-${cmd.name}\\ndescription: [^\\n]+\\n---`), `${cmd.name}: skill frontmatter`);
    assert.ok(cast.includes(cmd.body), `${cmd.name}: skill must carry the inlined body verbatim`);
    assert.ok(cast.includes('$ARGUMENTS'), `${cmd.name}: skill cast documents how arguments reach the body`);
  }
  for (const cmd of render.readCommands().map(render.inlineCommand)) {
    const cast = providers.PROVIDERS.copilot.commandCast(cmd);
    assert.match(cast, new RegExp(`^---\\nname: tomo-${cmd.name}\\ndescription: [^\\n]+\\n---`), `${cmd.name}: Copilot skill frontmatter`);
    assert.match(cast.match(/^---\n([\s\S]*?)\n---/)[1], /^name: [a-z]+(?:-[a-z]+)*\ndescription: /,
      `${cmd.name}: skill name must be lowercase and hyphenated`);
    assert.ok(cast.includes(cmd.body), `${cmd.name}: skill must retain the inlined body`);
    assert.ok(cast.includes(`/tomo-${cmd.name}`), `${cmd.name}: skill cast documents slash invocation`);
    assert.ok(cast.includes('$ARGUMENTS'), `${cmd.name}: skill cast documents argument forwarding`);
  }
  assert.throws(
    () => providers.PROVIDERS.copilot.commandCast({ name: 'Bad_Name', fm: { description: 'test' }, body: 'body' }),
    /must be lowercase and hyphenated/,
  );
});

test('provider dirs and filenames land in each harness scope', () => {
  const oc = providers.PROVIDERS.opencode;
  const cl = providers.PROVIDERS.claude;
  const cp = providers.PROVIDERS.copilot;
  const ag = providers.PROVIDERS.antigravity;
  const cx = providers.PROVIDERS.codex;
  assert.ok(oc.projectCommandDir('/p').endsWith(path.join('p', '.opencode', 'command')));
  assert.ok(cl.projectCommandDir('/p').endsWith(path.join('p', '.claude', 'commands', 'tomo')));
  assert.ok(cp.projectCommandDir('/p').endsWith(path.join('p', '.github', 'skills')));
  assert.ok(ag.projectCommandDir('/p').endsWith(path.join('p', '.agents', 'skills')));
  assert.ok(oc.projectAgentDir('/p').endsWith(path.join('p', '.opencode', 'agents')));
  assert.ok(cl.projectAgentDir('/p').endsWith(path.join('p', '.claude', 'agents')));
  assert.ok(cp.projectAgentDir('/p').endsWith(path.join('p', '.github', 'agents')));
  assert.ok(ag.projectAgentDir('/p').endsWith(path.join('p', '.agents', 'agents')));
  assert.ok(oc.globalCommandDir().endsWith(path.join('opencode', 'command')));
  assert.ok(cl.globalCommandDir().endsWith(path.join('.claude', 'commands', 'tomo')));
  assert.ok(cp.globalCommandDir().endsWith(path.join('.copilot', 'skills')));
  assert.ok(ag.globalCommandDir().endsWith(path.join('.gemini', 'antigravity-cli', 'skills')));
  assert.ok(oc.globalAgentDir().endsWith(path.join('opencode', 'agents')));
  assert.ok(cl.globalAgentDir().endsWith(path.join('.claude', 'agents')));
  assert.ok(cp.globalAgentDir().endsWith(path.join('.copilot', 'agents')));
  assert.ok(ag.globalAgentDir().endsWith(path.join('.gemini', 'config', 'agents')));
  assert.strictEqual(oc.commandFilename('requirements'), 'tomo-requirements.md', 'opencode command files are prefixed');
  assert.strictEqual(cl.commandFilename('requirements'), 'requirements.md', 'claude command files are bare');
  assert.strictEqual(cp.filename('requirements'), 'requirements.agent.md', 'copilot agent files are suffixed');
  assert.strictEqual(cp.commandFilename('requirements'), 'tomo-requirements/SKILL.md', 'copilot commands are skill folders');
  assert.strictEqual(ag.commandFilename('requirements'), 'tomo-requirements/SKILL.md', 'antigravity commands are skill folders');
  assert.strictEqual(ag.filename('coder'), 'coder/agent.md', 'antigravity agents are a folder of one');
  assert.match(ag.projectCommandDir('/p'), /skills$/, 'antigravity skills land under .agents/skills');
  assert.ok(cx.projectCommandDir('/p').endsWith(path.join('p', '.agents', 'skills')));
  assert.ok(cx.projectAgentDir('/p').endsWith(path.join('p', '.codex', 'agents')));
  assert.ok(cx.globalCommandDir().endsWith(path.join('.agents', 'skills')));
  assert.ok(cx.globalAgentDir().endsWith(path.join('.codex', 'agents')));
  assert.strictEqual(cx.filename('coder'), 'coder.toml', 'codex agents are TOML');
  assert.strictEqual(cx.commandFilename('requirements'), 'tomo-requirements/SKILL.md', 'codex commands are skill folders');
});

test('parseProviderSpec accepts all, ids, and rejects unknown ids', () => {
  assert.deepStrictEqual(providers.parseProviderSpec(), ['opencode', 'claude', 'copilot', 'antigravity', 'codex']);
  assert.deepStrictEqual(providers.parseProviderSpec(''), ['opencode', 'claude', 'copilot', 'antigravity', 'codex']);
  assert.deepStrictEqual(providers.parseProviderSpec('all'), ['opencode', 'claude', 'copilot', 'antigravity', 'codex']);
  assert.deepStrictEqual(providers.parseProviderSpec('opencode'), ['opencode']);
  assert.deepStrictEqual(providers.parseProviderSpec('claude'), ['claude']);
  assert.deepStrictEqual(providers.parseProviderSpec('copilot'), ['copilot']);
  assert.deepStrictEqual(providers.parseProviderSpec('antigravity'), ['antigravity']);
  assert.deepStrictEqual(providers.parseProviderSpec('codex'), ['codex']);
  assert.deepStrictEqual(providers.parseProviderSpec('opencode, claude'), ['opencode', 'claude']);
  assert.deepStrictEqual(providers.parseProviderSpec('claude,claude'), ['claude', 'claude']);
  assert.throws(() => providers.parseProviderSpec('gemini'), /unknown provider "gemini"/);
});

test('render casts are delegated to the registry (one render path)', () => {
  const delegated = {
    opencode: render.renderPersonaOpenCode,
    claude: render.renderPersonaClaude,
    copilot: render.renderPersonaCopilot,
    antigravity: render.renderPersonaAntigravity,
    codex: render.renderPersonaCodex,
  };
  for (const p of render.readPersonas()) {
    for (const id of providers.providerIds()) {
      assert.strictEqual(
        delegated[id](p),
        providers.PROVIDERS[id].cast(p),
        `${p.name}: render must delegate to the ${id} registry renderer`,
      );
      assert.ok(providers.isProvider(id), `${id}: isProvider`);
    }
  }
});

test('opencode casts are read-only-safe and lexically clean of claude keys', () => {
  for (const p of render.readPersonas()) {
    const content = providers.PROVIDERS.opencode.cast(p);
    assert.match(content, /^---\ndescription: [^\n]+\nmode: subagent\n/s);
    assert.ok(!/^\s*name:/.test(content), `${p.name}: opencode cast must not carry a name key`);
    assert.ok(!/^\s*tools:/.test(content), `${p.name}: opencode cast must not carry a tools key`);
    const readonly = ['reviewer', 'security'].includes(p.name);
    assert.strictEqual(content.includes('edit: deny'), readonly);
  }
});

test('claude commands inline the body and pre-approve tomo config reads', () => {
  for (const cmd of render.readCommands().map(render.inlineCommand)) {
    const cast = providers.PROVIDERS.claude.commandCast(cmd);
    assert.strictEqual(cast, `---\ndescription: ${cmd.fm.description}\nallowed-tools: ${providers.claudeReadGrant()}\n---\n\n${cmd.body}\n`);
  }
  assert.match(providers.claudeReadGrant(), /^Read\((~|\/\/).*\/tomo\/\*\*\)$/);
});

test('claude casts carry identity and optional tools', () => {
  for (const p of render.readPersonas()) {
    const content = providers.PROVIDERS.claude.cast(p);
    assert.match(content, /^---\nname: [^\n]+\ndescription: [^\n]+\n/s);
    assert.strictEqual(typeof p.fm.tools, 'string');
  }
});

test('copilot casts are valid agent profiles', () => {
  for (const p of render.readPersonas()) {
    const content = providers.PROVIDERS.copilot.cast(p);
    assert.match(
      content,
      /^---\nname: [^\n]+\ndescription: [^\n]+\n(?:tools: \[[^\]]+\]\n)?---/s,
      `${p.name}: copilot frontmatter shape`,
    );
    const readonly = ['reviewer', 'security'].includes(p.name);
    assert.strictEqual(content.includes('tools: ["read", "search"]'), readonly);
    if (!readonly) assert.ok(!/^\s*tools:/.test(content), `${p.name}: copilot cast must not carry a tools key`);
    const body = content.slice(content.indexOf('\n---\n') + 6).trimEnd();
    assert.strictEqual(body, p.body.replace(/\s+$/s, ''), `${p.name}: copilot body must match canonical`);
  }
});

test('codex casts are valid TOML subagents', () => {
  for (const p of render.readPersonas()) {
    const content = providers.PROVIDERS.codex.cast(p);
    assert.match(content, new RegExp('^name = "' + p.name + '"\\ndescription = "[^"]+"\\n'), `${p.name}: codex TOML identity`);
    assert.ok(content.includes('developer_instructions = """'), `${p.name}: codex cast carries developer_instructions`);
    const readonly = ['reviewer', 'security'].includes(p.name);
    assert.strictEqual(content.includes('sandbox_mode = "read-only"'), readonly, `${p.name}: read-only codex casts set sandbox_mode`);
    assert.strictEqual(content.includes('sandbox_mode = "workspace-write"'), !readonly, `${p.name}: write codex casts set workspace-write`);
  }
});

test('cast bodies are the verbatim canonical body', () => {
  for (const p of render.readPersonas()) {
    const canonical = p.body.replace(/\s+$/s, '');
    for (const id of providers.providerIds()) {
      const content = providers.PROVIDERS[id].cast(p);
      const body = id === 'codex'
        ? content.slice(content.indexOf('developer_instructions = """\n') + 'developer_instructions = """\n'.length, content.lastIndexOf('\n"""')).trimEnd()
        : content.slice(content.indexOf('\n---\n') + 6).trimEnd();
      assert.strictEqual(body, canonical, `${p.name}.${id}: body must match canonical`);
    }
  }
});

test('seedSettingsProvider writes single-provider tags and stays idempotent', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'tomo-providers-'));
  try {
    const p1 = path.join(root, 'a');
    const out = providers.seedSettingsProvider(p1, ['opencode']);
    assert.strictEqual(out.changed, true);
    assert.deepStrictEqual(JSON.parse(fs.readFileSync(out.settingsPath, 'utf8')), { provider: 'opencode' });

    // Multi-provider selects leave the tag alone.
    fs.writeFileSync(out.settingsPath, JSON.stringify({ provider: 'opencode', gates: 'auto' }));
    const out2 = providers.seedSettingsProvider(p1, ['opencode', 'claude']);
    assert.strictEqual(out2.changed, false);
    assert.strictEqual(JSON.parse(fs.readFileSync(out2.settingsPath, 'utf8')).gates, 'auto');

    // Same single provider is a no-op.
    const out3 = providers.seedSettingsProvider(p1, ['opencode']);
    assert.strictEqual(out3.changed, false);

    // Copilot is a single provider too.
    const p2 = path.join(root, 'b');
    const out4 = providers.seedSettingsProvider(p2, ['copilot']);
    assert.strictEqual(out4.changed, true);
    assert.deepStrictEqual(JSON.parse(fs.readFileSync(out4.settingsPath, 'utf8')), { provider: 'copilot' });

    // Antigravity seeds its own tag.
    const p3 = path.join(root, 'c');
    const out5 = providers.seedSettingsProvider(p3, ['antigravity']);
    assert.strictEqual(out5.changed, true);
    assert.deepStrictEqual(JSON.parse(fs.readFileSync(out5.settingsPath, 'utf8')), { provider: 'antigravity' });
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});