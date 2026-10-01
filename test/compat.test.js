/*
 * Compatibility: run records from released versions stay readable. test/fixtures holds them as
 * released; they are never rewritten, and a change that breaks them needs a migration and a note in
 * docs/compatibility.md.
 */
import assert from 'node:assert/strict';
import { cpSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { ask, calls, env } from './helpers.js';

const FIXTURES = join(dirname(fileURLToPath(import.meta.url)), 'fixtures');
const installRun = () => cpSync(join(FIXTURES, 'run-v1'), join(env.ASK_HOME, 'runs', '20261001T120000000-abc123'), { recursive: true });

test('a version 1 run record can be shown, listed and continued', async () => {
  installRun();
  const shown = await ask(['show', 'abc123']);
  assert.equal(shown.stdout, 'The token expiry is compared in seconds against milliseconds.\n');
  assert.match(shown.stderr, /^ask abc123 · Fake 1\.0 · ok · 41\.2s · 1 file changed · 52\.1k in · 2\.0k out · \$0\.3100$/m);
  assert.match((await ask(['runs'])).stdout, /^abc123 .* ok +Fake 1\.0 +41\.2s +Why does the login test fail\?$/m);
  const followup = await ask(['-c', 'abc123', 'Fix it.']);
  assert.equal(followup.code, 0);
  assert.equal(calls()[0].session, 's-fixture');
  assert.equal(calls()[0].access, 'write');
});
