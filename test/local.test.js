/* The harness contract, through harnesses in $ASK_HOME/harnesses written as shell scripts. */
import assert from 'node:assert/strict';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';
import { ask, env, tmp, writeJson } from './helpers.js';

/* Installs a local harness whose run is `body` (sh); `models` prints "small<TAB>Small One" and "big". */
function harness(name, body) {
  const dir = join(env.ASK_HOME, 'harnesses');
  mkdirSync(dir, { recursive: true });
  writeFileSync(join(dir, name), `#!/bin/sh\nPATH=/usr/bin:/bin:$PATH\nif [ "$1" = models ]; then printf 'small\\tSmall One\\nbig\\n'; exit 0; fi\n${body}\n`, { mode: 0o755 });
}

test('a harness is listed with its models.json ids, next to the others', async () => {
  harness('echo', 'cat');
  writeJson(join(env.ASK_HOME, 'models.json'), { echo: ['extra'] });
  const r = await ask(['models']);
  assert.deepEqual(r.stdout.trim().split('\n').filter((id) => id.startsWith('echo:')), ['echo:small', 'echo:big', 'echo:extra']);
});

test('a harness gets the prompt on stdin, the run in its env and the directory as cwd', async () => {
  harness('echo', 'cat > "$HOME/prompt"; echo "$ASK_MODEL $ASK_EFFORT $ASK_ACCESS $(pwd)" > "$HOME/env"; echo answer');
  const r = await ask(['-m', 'echo:small#high', '-w', '-C', tmp, 'do it']);
  assert.equal(r.code, 0);
  assert.equal(r.stdout, 'answer\n');
  assert.equal(readFileSync(join(tmp, 'prompt'), 'utf8'), 'do it');
  assert.equal(readFileSync(join(tmp, 'env'), 'utf8').trim(), `small high write ${tmp}`);
  assert.match(r.stderr, /^ask: Small One \(high\) /);
});

test('a harness gets --schema as a file in ASK_SCHEMA', async () => {
  harness('echo', 'cat "$ASK_SCHEMA" > "$HOME/schema"; echo \'{"a": 1}\'');
  writeJson(join(tmp, 's.json'), { type: 'object' });
  const r = await ask(['-m', 'echo:small', '--schema', join(tmp, 's.json'), 'go']);
  assert.equal(r.code, 0);
  assert.deepEqual(JSON.parse(readFileSync(join(tmp, 'schema'), 'utf8')), { type: 'object' });
});

test('a report names the model and gives usage and a note', async () => {
  harness('echo', `echo '{"name": "Echo 2", "input": 1200, "output": 30, "cost": 0.5, "note": "partial"}' > "$ASK_REPORT"; echo ok`);
  const r = await ask(['-m', 'echo:big', 'go']);
  assert.match(r.stderr, /^ask: Echo 2 [\d.]+s 1\.2k in 30 out \$0\.5000; partial$/m);
});

test("a failed harness's last stderr line is the error", async () => {
  harness('echo', 'echo noise >&2; echo "quota exceeded" >&2; exit 3');
  const r = await ask(['-m', 'echo:big', 'go']);
  assert.equal(r.code, 1);
  assert.match(r.stderr, /failed after .*: quota exceeded$/m);
});
