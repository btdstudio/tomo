const { test } = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const render = require('../scripts/render.js');

const ROOT = render.ROOT;
const VALID_GATES = ['none', 'block', 'prompt', 'auto'];
const REQUIRED_KEYS = ['name', 'description', 'args', 'gate'];

test('command contracts parse with valid frontmatter and naming', () => {
  const commands = render.readCommands();
  assert.ok(commands.length >= 2, 'expected at least help and status');
  for (const cmd of commands) {
    assert.strictEqual(cmd.name, cmd.fm.name, `${cmd.name}: name must match filename`);
    for (const key of REQUIRED_KEYS) {
      assert.ok(key in cmd.fm, `${cmd.name}: missing frontmatter field "${key}"`);
    }
    for (const key of ['name', 'description', 'gate']) {
      assert.ok(cmd.fm[key], `${cmd.name}: empty frontmatter field "${key}"`);
    }
    assert.ok(VALID_GATES.includes(cmd.fm.gate), `${cmd.name}: invalid gate "${cmd.fm.gate}"`);
    assert.strictEqual(cmd.body, '', `${cmd.name}: contract must be frontmatter only`);
    assert.match(cmd.name, /^[\w-]+$/, `${cmd.name}: unsupported characters`);
  }
});

test('content store mirrors the command registry (no missing/orphan/gate drift)', () => {
  const problems = render.checkContent();
  assert.deepStrictEqual(problems, [], 'content store is out of sync:\n' + problems.join('\n'));

  const content = render.readContentCommands();
  for (const name of Object.keys(content)) {
    assert.ok(content[name].length > 0, `${name}: empty canonical body`);
  }
});

function vocabCode() {
  return {
    settings: require('../src/lib/settings.js'),
    personas: require('../src/lib/personas.js'),
    lib: (m) => require(path.join(ROOT, 'src', 'lib', `${m}.js`)),
  };
}

function vocabInput() {
  const content = render.readContentCommands();
  content.gate = fs.readFileSync(render.GATE_CONTENT_PATH, 'utf8');
  return { commands: render.readCommands(), content, code: vocabCode() };
}

test('vocabulary check is green on the committed content store', () => {
  const { commands, content, code } = vocabInput();
  assert.deepStrictEqual(render.checkVocabulary(commands, content, code), []);
});

test('vocabulary check flags a help index missing a command', () => {
  const { commands, content, code } = vocabInput();
  content.help = content.help.replace('/tomo design', '/tomo designX');
  assert.ok(render.checkVocabulary(commands, content, code).some((p) => p.includes('"/tomo design"')));
});

test('vocabulary check flags a settings enumeration missing a key', () => {
  const { commands, content, code } = vocabInput();
  content.init = content.init.replace('`personas`', '`personasX`');
  assert.ok(render.checkVocabulary(commands, content, code).some((p) => p.includes('"personas"')));
});

test('vocabulary check flags a lib symbol that is not exported', () => {
  const { commands, content, code } = vocabInput();
  content.arch = content.arch.replace('`resolvePersona(', '`resolvePersonaX(');
  assert.ok(render.checkVocabulary(commands, content, code).some((p) => p.includes('resolvePersonaX')));
});

test('vocabulary check flags a persona default that drifted', () => {
  const { commands, content, code } = vocabInput();
  content.trace = content.trace.replace('**reviewer**', '**architect**');
  assert.ok(render.checkVocabulary(commands, content, code).some((p) => p.includes('"trace" default persona')));
});

test('wrapper inlines the canonical body; gated commands inline the gate', () => {
  const content = render.readContentCommands();
  const gate = fs.readFileSync(render.GATE_CONTENT_PATH, 'utf8').trimEnd();
  for (const cmd of render.readCommands()) {
    const wrapper = render.render(cmd);
    assert.match(wrapper, /^---\ndescription: [^\n]+\n---\n\n/s);
    assert.strictEqual(wrapper, `---\ndescription: ${cmd.fm.description}\n---\n\n${render.inlineBody(cmd)}\n`);
    assert.ok(!wrapper.includes(render.GATE_INCLUDE), `${cmd.name}: gate directive must be expanded`);
    assert.strictEqual(wrapper.includes(gate), cmd.fm.gate !== 'none', `${cmd.name}: gate inlined iff gated`);
    for (const part of content[cmd.name].split(render.GATE_INCLUDE)) {
      assert.ok(wrapper.includes(part.trim()), `${cmd.name}: wrapper must carry the canonical body verbatim`);
    }
    assert.ok(!/content store|\.tomo\/commands\//.test(wrapper), `${cmd.name}: wrapper must not send the model to read a body`);
  }
});

test('live harness template syntax is rejected', () => {
  for (const bad of ['run !`ls`', 'use $1 here', 'see @src/x.md', '@.tomo/state.json first']) {
    assert.ok(render.liveSyntaxProblems('x', bad).length > 0, `must reject: ${bad}`);
  }
  for (const ok of ['feat(api)!: x', 'mail a@b.com', '$ARGUMENTS', '`.tomo/lib/state.js`']) {
    assert.deepStrictEqual(render.liveSyntaxProblems('x', ok), [], `must accept: ${ok}`);
  }
});

test('wrappers are provider-agnostic: no per-harness generated copies', () => {
  render.writeAll();
  for (const cmd of render.readCommands()) {
    assert.strictEqual(
      render.render(cmd),
      fs.readFileSync(render.commonPath(cmd.name), 'utf8'),
      `${cmd.name}: common cache wrapper must equal the single render`,
    );
  }
  for (const dir of [
    path.join(ROOT, 'adapters', 'opencode', 'command'),
    path.join(ROOT, 'adapters', 'claude', 'commands', 'tomo'),
  ]) {
    assert.ok(!fs.existsSync(dir), `per-harness wrapper dir must not be checked in: ${dir}`);
  }
});

test('$ARGUMENTS passes through untouched in the common wrapper', () => {
  for (const cmd of render.readCommands()) {
    const content = render.render(cmd);
    assert.match(content, /\$ARGUMENTS/, `${cmd.name}: wrapper must keep $ARGUMENTS for substitution`);
  }
});

test('writeAll regenerates the gitignored cache deterministically', () => {
  const written = render.writeAll();
  for (const cmd of render.readCommands()) {
    const p = render.commonPath(cmd.name);
    assert.ok(written.includes(p), `${cmd.name}: cache must include the wrapper`);
    assert.strictEqual(render.render(cmd), fs.readFileSync(p, 'utf8'), `${cmd.name}: cache wrapper must match render`);
  }
  for (const cast of render.generatePersonas()) {
    const p = render.personaCastPath(cast.name, cast.cast);
    assert.ok(written.includes(p), `${cast.name}.${cast.cast}: cache must include the persona cast`);
    assert.strictEqual(cast.content, fs.readFileSync(p, 'utf8'), `${cast.name}.${cast.cast}: cache cast must match render`);
  }
  const again = render.writeAll();
  assert.deepStrictEqual([...again].sort(), [...written].sort(), 'regeneration must be deterministic');
});