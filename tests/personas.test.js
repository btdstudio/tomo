const { test } = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const render = require('../scripts/render.js');
const personas = require('../src/lib/personas.js');

const ROOT = render.ROOT;

test('canonical personas parse with name/description/focus and a body', () => {
  const list = render.readPersonas();
  assert.deepStrictEqual(list.map((p) => p.name), personas.PERSONAS, 'built-in ids match canonical files');
  for (const p of list) {
    assert.strictEqual(p.fm.name, p.name, `${p.name}: name must match filename`);
    assert.ok(p.fm.description, `${p.name}: missing description`);
    assert.ok(p.fm.focus, `${p.name}: missing focus`);
    assert.ok(p.body.length > 0, `${p.name}: empty body`);
    assert.ok(!/^---/.test(p.body), `${p.name}: body must not carry frontmatter`);
  }
});

test('every persona renders a valid opencode cast', () => {
  for (const p of render.readPersonas()) {
    const content = render.renderPersonaOpenCode(p);
    assert.match(content, /^---\ndescription: [^\n]+\nmode: subagent\n/s, `${p.name}: opencode cast needs description + mode`);
    assert.ok(!/^\s*name:/.test(content.split('---')[1] || ''), `${p.name}: opencode cast must not carry a name key (provider pass-through)`);
    assert.ok(!/^\s*tools:/.test(content.split('---')[1] || ''), `${p.name}: opencode cast must not carry a tools key`);
    const readonly = ['reviewer', 'security'].includes(p.name);
    assert.strictEqual(content.includes('edit: deny'), readonly, `${p.name}: read-only casts deny edits, others do not`);
  }
});

test('every persona renders a valid claude cast', () => {
  for (const p of render.readPersonas()) {
    const content = render.renderPersonaClaude(p);
    assert.match(content, new RegExp(`^---\\nname: ${p.name}\\ndescription: [^\\n]+\\n(?:tools: [^\\n]+\\n)?---`), `${p.name}: claude cast needs name + description`);
    const fm = content.slice(content.indexOf('---') + 4, content.indexOf('---', 4));
    if (p.fm.tools) {
      assert.match(fm, new RegExp(`\ntools: ${p.fm.tools}\n`), `${p.name}: claude cast carries the persona's tools`);
    } else {
      assert.ok(!fm.includes('tools:'), `${p.name}: claude cast omits tools when it inherits all`);
    }
  }
});

test('persona casts never diverge from the canonical body', () => {
  const renderCast = {
    opencode: render.renderPersonaOpenCode,
    claude: render.renderPersonaClaude,
    copilot: render.renderPersonaCopilot,
    antigravity: render.renderPersonaAntigravity,
    codex: render.renderPersonaCodex,
  };
  for (const p of render.readPersonas()) {
    const canonical = p.body.replace(/\s+$/s, '');
    for (const cast of Object.keys(renderCast)) {
      const rendered = renderCast[cast](p);
      const body = cast === 'codex'
        ? rendered.slice(rendered.indexOf('developer_instructions = """\n') + 'developer_instructions = """\n'.length, rendered.lastIndexOf('\n"""')).trimEnd()
        : rendered.slice(rendered.indexOf('\n---\n') + 6).trimEnd();
      assert.strictEqual(body, canonical, `${p.name}.${cast}: cast body must be the verbatim canonical body`);
    }
  }
});

test('persona casts are part of render determinism', () => {
  const written = render.writeAll();
  const renderCast = {
    opencode: render.renderPersonaOpenCode,
    claude: render.renderPersonaClaude,
    copilot: render.renderPersonaCopilot,
    antigravity: render.renderPersonaAntigravity,
    codex: render.renderPersonaCodex,
  };
  for (const p of render.readPersonas()) {
    for (const id of Object.keys(renderCast)) {
      const file = render.personaCastPath(p.name, id);
      assert.ok(written.includes(file), `${p.name}.${id}: cache must include the cast`);
      assert.strictEqual(
        fs.readFileSync(file, 'utf8'),
        renderCast[id](p),
        `${p.name}.${id}: cache cast must match the registry renderer`,
      );
    }
  }
});

test('resolvePersona defaults, overrides, custom paths, and unknowns', () => {
  const settings = { personas: { map: {}, custom: {} } };
  assert.deepStrictEqual(personas.resolvePersona('requirements', settings), {
    id: 'analyst', mode: 'lens', source: 'default', file: '.tomo/personas/analyst.md',
  });
  assert.deepStrictEqual(personas.resolvePersona('implement', settings), {
    id: 'coder', mode: 'spawn', source: 'default', file: '.tomo/personas/coder.md',
  });
  assert.deepStrictEqual(personas.resolvePersona('implement-security', settings), {
    id: 'security', mode: 'spawn', source: 'default', file: '.tomo/personas/security.md',
  });
  assert.deepStrictEqual(personas.resolvePersona('plan', { personas: { map: { plan: 'security' }, custom: {} } }), {
    id: 'security', mode: 'lens', source: 'override', file: '.tomo/personas/security.md',
  });
  assert.deepStrictEqual(personas.resolvePersona('design', {
    personas: { map: { design: 'lead' }, custom: { lead: 'personas/lead.md' } },
  }), {
    id: 'lead', mode: 'lens', source: 'custom', file: 'personas/lead.md',
  });
  assert.deepStrictEqual(personas.resolvePersona('trace', { personas: { map: { trace: 'personas/local.md' }, custom: {} } }), {
    id: 'personas/local.md', mode: 'lens', source: 'custom', file: 'personas/local.md',
  });
  assert.deepStrictEqual(personas.resolvePersona('bogus', settings), {
    id: '', mode: 'lens', source: 'default', file: '',
  }, 'unknown step resolves empty');
});

test('modeForStep and isBuiltIn agree with the map', () => {
  assert.ok(personas.isBuiltIn('coder'));
  assert.ok(!personas.isBuiltIn('lead'));
  for (const step of Object.keys(personas.DEFAULT_PERSONA_MAP)) {
    assert.ok(
      personas.modeForStep(step) === (step === 'implement' || step.startsWith('implement-') ? 'spawn' : 'lens'),
      `${step}: mode should match the spawn set`,
    );
  }
});