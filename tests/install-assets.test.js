const { test } = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');
const render = require('../scripts/render.js');
const providers = require('../src/lib/providers.js');

// Every asset a canonical command body references via .tomo/lib/,
// .tomo/templates/, or .tomo/personas/ must have a committed source in
// src/lib/, src/templates/, or src/personas/. install.js ships those sources
// into .tomo/; if a command names an asset that does not exist, install would
// ship a dangling reference.
test('canonical bodies only reference installed asset sources that exist', () => {
  const content = render.readContentCommands();
  const re = /\.tomo\/(lib|templates|personas)\/([A-Za-z0-9_./-]+?\.(?:js|md))(?=\s|`|\)|$)/g;
  let seen = 0;
  for (const name of Object.keys(content)) {
    for (const m of content[name].matchAll(re)) {
      seen++;
      const kind = m[1]; // lib | templates | personas
      const file = m[2];
      const expected = path.join(render.ROOT, 'src', kind, file);
      assert.ok(
        fs.existsSync(expected),
        `${name}: references .tomo/${kind}/${file} but src/${kind}/${file} is missing`,
      );
    }
  }
  assert.ok(seen > 0, 'expected at least one asset reference to check');
});

// Every project install writes generated files into each provider's command and
// agent dirs. AGENTS.md says nothing generated is ever committed, so each of
// those target dirs must be covered by a .gitignore rule — otherwise a consumer
// project's first `npm run install` leaves an unresolvable dirty tree. Use
// `git check-ignore` (the same matcher git uses) so a future provider that adds
// an uncovered dir fails here, rather than surprising a contributor.
function isGitIgnored(rel) {
  try {
    execFileSync('git', ['check-ignore', '-q', '--', rel], { cwd: render.ROOT, stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

test('every provider project install target is gitignored', () => {
  const sampleCommand = render.readCommands()[0].name;
  const samplePersona = render.readPersonas()[0].name;
  for (const prov of Object.values(providers.PROVIDERS)) {
    const commandPath = path.join(prov.projectCommandDir('.'), prov.commandFilename(sampleCommand));
    assert.ok(isGitIgnored(commandPath), `${prov.id}: command target not ignored: ${commandPath}`);
    const agentPath = path.join(prov.projectAgentDir('.'), prov.filename(samplePersona));
    assert.ok(isGitIgnored(agentPath), `${prov.id}: agent target not ignored: ${agentPath}`);
  }
});
