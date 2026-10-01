/* The command line: arguments, usage errors, answers and status lines, JSON checks, timeouts and stopping. */
import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';
import { tmp, env, start, ask, calls, alive, until, writeJson } from './helpers.js';

test('no arguments print the help', async () => {
  const r = await ask([]);
  assert.equal(r.code, 0);
  assert.match(r.stdout, /ask -m MODEL/);
});

test('a missing -m is a usage error that lists the models', async () => {
  writeJson(join(env.ASK_HOME, 'models.json'), { opencode: ['p/m'] });
  const r = await ask(['hello']);
  assert.equal(r.code, 2);
  assert.match(r.stderr, /needs a model/);
  assert.match(r.stderr, /claude:opus/);
  assert.match(r.stderr, /opencode:p\/m/);
  assert.equal(calls().length, 0);
});

test('a malformed model or an unknown harness is a usage error', async () => {
  const bad = await ask(['-m', 'opus', 'hello']);
  assert.equal(bad.code, 2);
  assert.match(bad.stderr, /expected harness:id/);
  const unknown = await ask(['-m', 'gemini:x', 'hello']);
  assert.equal(unknown.code, 2);
  assert.match(unknown.stderr, /no harness "gemini"; installed: claude, codex, opencode/);
});

test('-r and -w together are refused', async () => {
  const r = await ask(['-m', 'claude:opus', '-r', '-w', 'hello']);
  assert.equal(r.code, 2);
  assert.match(r.stderr, /choose one of -r/);
  assert.equal(calls().length, 0);
});

test('a models.json that does not parse is a usage error', async () => {
  mkdirSync(env.ASK_HOME, { recursive: true });
  writeFileSync(join(env.ASK_HOME, 'models.json'), '{');
  const r = await ask(['models']);
  assert.equal(r.code, 2);
  assert.match(r.stderr, /cannot parse/);
});

test('a JSON string answer is printed as JSON', async () => {
  const r = await ask(['-m', 'opencode:p/m', '--json', 'go'], { extra: { FAKE_ANSWER: '"hello"' } });
  assert.equal(r.stdout, '"hello"\n');
});

test('--json strips a code fence around the answer', async () => {
  const r = await ask(['-m', 'opencode:p/m', '--json', 'go'], { extra: { FAKE_ANSWER: '```json\n{"a": 1}\n```' } });
  assert.deepEqual(JSON.parse(r.stdout), { a: 1 });
  assert.match(calls()[0].stdin, /Answer ONLY with JSON/);
});

test('an answer that is not valid JSON fails under --json', async () => {
  const r = await ask(['-m', 'opencode:p/m', '--json', 'go'], { extra: { FAKE_ANSWER: 'sure thing' } });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /not valid JSON/);
});

test('-C sets the directory the model works in', async () => {
  mkdirSync(join(tmp, 'work'));
  await ask(['-m', 'claude:opus', '-C', join(tmp, 'work'), 'hi']);
  assert.equal(calls()[0].cwd, join(tmp, 'work'));
});

test('the prompt is read from stdin when none is given', async () => {
  const r = await ask(['-m', 'claude:opus'], { input: 'from stdin\n' });
  assert.equal(r.stdout, 'claude: from stdin\n');
});

test('a hung model times out and its process is killed', async () => {
  const r = await ask(['-m', 'claude:opus', '-t', '0.5', 'hang'], { extra: { FAKE_HANG: 'hang' } });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /timed out/);
  assert.equal(alive(calls()[0].pid), false);
});

test('stopping ask with SIGINT stops the models it started', async () => {
  const run = start(['-m', 'codex:gpt-x', 'hang'], { extra: { FAKE_HANG: 'hang' } });
  await until(() => calls().length === 1);
  const { pid } = calls()[0];
  run.child.kill('SIGINT');
  const r = await run;
  assert.equal(r.code, 130);
  await until(() => !alive(pid));
});

test('a failing model exits 1 with its message', async () => {
  const r = await ask(['-m', 'codex:gpt-x', 'break'], { extra: { FAKE_FAIL: 'break' } });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /failed after .*codex boom/);
  assert.equal(r.stdout, '');
});

test('stopping ask kills a model that ignores SIGTERM before exiting', async () => {
  const run = start(['-m', 'codex:gpt-x', 'hang'], { extra: { FAKE_HANG: 'hang', FAKE_IGNORE_TERM: '1' } });
  await until(() => calls().length === 1);
  const { pid } = calls()[0];
  run.child.kill('SIGINT');
  const r = await run;
  assert.equal(r.code, 130);
  assert.equal(alive(pid), false);
});
