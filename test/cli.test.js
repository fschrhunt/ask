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
  writeJson(join(env.ASK_HOME, 'models.json'), { fake: ['extra'] });
  const r = await ask(['hello']);
  assert.equal(r.code, 2);
  assert.match(r.stderr, /needs a model/);
  assert.match(r.stderr, /fake:small/);
  assert.match(r.stderr, /fake:extra/);
  assert.equal(calls().length, 0);
});

test('a malformed model or an unknown harness is a usage error', async () => {
  const bad = await ask(['-m', 'opus', 'hello']);
  assert.equal(bad.code, 2);
  assert.match(bad.stderr, /expected harness:id/);
  const unknown = await ask(['-m', 'gemini:x', 'hello']);
  assert.equal(unknown.code, 2);
  assert.match(unknown.stderr, /no harness "gemini" in .*; installed: fake;/);
});

test('-r and -w together are refused', async () => {
  const r = await ask(['-m', 'fake:small', '-r', '-w', 'hello']);
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
  const r = await ask(['-m', 'fake:big', '--json', 'go'], { extra: { FAKE_ANSWER: '"hello"' } });
  assert.equal(r.stdout, '"hello"\n');
});

test('--json strips a code fence around the answer', async () => {
  const r = await ask(['-m', 'fake:big', '--json', 'go'], { extra: { FAKE_ANSWER: '```json\n{"a": 1}\n```' } });
  assert.deepEqual(JSON.parse(r.stdout), { a: 1 });
  assert.match(calls()[0].stdin, /Answer ONLY with JSON/);
});

test('an answer that is not valid JSON fails under --json', async () => {
  const r = await ask(['-m', 'fake:big', '--json', 'go'], { extra: { FAKE_ANSWER: 'sure thing' } });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /not valid JSON/);
});

test('-C sets the directory the model works in', async () => {
  mkdirSync(join(tmp, 'work'));
  await ask(['-m', 'fake:small', '-C', join(tmp, 'work'), 'hi']);
  assert.equal(calls()[0].cwd, join(tmp, 'work'));
});

test('the prompt is read from stdin when none is given', async () => {
  const r = await ask(['-m', 'fake:small'], { input: 'from stdin\n' });
  assert.equal(r.stdout, 'fake: from stdin\n');
});

test('a hung model times out and its process is killed', async () => {
  const r = await ask(['-m', 'fake:small', '-t', '0.5', 'hang'], { extra: { FAKE_HANG: 'hang' } });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /timed out/);
  assert.equal(alive(calls()[0].pid), false);
});

test('stopping ask with SIGINT stops the models it started', async () => {
  const run = start(['-m', 'fake:big', 'hang'], { extra: { FAKE_HANG: 'hang' } });
  await until(() => calls().length === 1);
  const { pid } = calls()[0];
  run.child.kill('SIGINT');
  const r = await run;
  assert.equal(r.code, 130);
  await until(() => !alive(pid));
});

test('a failing model exits 1 with its message', async () => {
  const r = await ask(['-m', 'fake:big', 'break'], { extra: { FAKE_FAIL: 'break' } });
  assert.equal(r.code, 1);
  assert.match(r.stderr, / · failed · .*fake boom$/m);
  assert.equal(r.stdout, '');
});

test('stopping ask kills a model that ignores SIGTERM before exiting', async () => {
  const run = start(['-m', 'fake:big', 'hang'], { extra: { FAKE_HANG: 'hang', FAKE_IGNORE_TERM: '1' } });
  await until(() => calls().length === 1);
  const { pid } = calls()[0];
  run.child.kill('SIGINT');
  const r = await run;
  assert.equal(r.code, 130);
  assert.equal(alive(pid), false);
});

test('read runs start with the grounding preamble; write runs get the prompt as given', async () => {
  await ask(['-m', 'fake:small', 'where is main?']);
  await ask(['-m', 'fake:small', '-w', 'fix it']);
  const [read, write] = calls();
  assert.match(read.stdin, /^Answer from the files in your working directory/);
  assert.match(read.stdin, /where is main\?$/);
  assert.equal([read.access, write.access].join(), 'read,write');
  assert.equal(write.stdin, 'fix it');
});

test('the answer goes to stdout and a status line with usage to stderr', async () => {
  const r = await ask(['-m', 'fake:small', 'hi']);
  assert.equal(r.stdout, 'fake: hi\n');
  assert.match(r.stderr, /^ask (\w{6}) · fake:small · read · .+ · started\nask \1 · Fake 1\.0 · ok · [\d.]+s · 10 in · 5 out · \$0\.0100\n$/);
});

test('a run that reports no usage prints no usage', async () => {
  const r = await ask(['-m', 'fake:small', 'hi'], { extra: { FAKE_NO_USAGE: '1' } });
  assert.equal(r.code, 0);
  assert.doesNotMatch(r.stderr, / in .* out/);
});

test('an answer from a harness that exited nonzero fails', async () => {
  const r = await ask(['-m', 'fake:small', 'hi'], { extra: { FAKE_EXIT: '1' } });
  assert.equal(r.code, 1);
  assert.match(r.stderr, / · failed · .*exit 1$/m);
  assert.equal(r.stdout, '');
});

test('--schema answers are checked, then printed as JSON', async () => {
  writeJson(join(tmp, 's.json'), { type: 'object', properties: { a: { type: 'number' } }, required: ['a'] });
  const good = await ask(['-m', 'fake:small', '--schema', join(tmp, 's.json'), 'go'], { extra: { FAKE_ANSWER: '{"a": 1}' } });
  assert.deepEqual(JSON.parse(good.stdout), { a: 1 });
  assert.match(calls()[0].stdin, /Answer ONLY with JSON matching this JSON Schema/);
  const bad = await ask(['-m', 'fake:small', '--schema', join(tmp, 's.json'), 'go'], { extra: { FAKE_ANSWER: '{"a": "one"}' } });
  assert.equal(bad.code, 1);
  assert.match(bad.stderr, /does not match the schema: \$\.a: expected number, got string/);
  assert.equal(bad.stdout, '');
});

test('a relative -C is resolved once, against where ask runs', async () => {
  mkdirSync(join(tmp, 'work'));
  const r = await ask(['-m', 'fake:small', '-C', 'work', 'hi']);
  assert.equal(r.code, 0);
  assert.equal(calls()[0].cwd, join(tmp, 'work'));
});

test('numbers are checked: -t, -j and a task timeout must be above 0', async () => {
  for (const args of [['-m', 'fake:small', '-t', 'abc', 'hi'], ['batch', '-j', '0', '-m', 'fake:small', '-']]) {
    const r = await ask(args, { input: '[{"prompt": "a"}]' });
    assert.equal(r.code, 2, args.join(' '));
    assert.match(r.stderr, /needs a (whole )?number above 0/);
  }
  const task = await ask(['batch', '-m', 'fake:small', '-'], { input: '[{"prompt": "a", "timeout": "soon"}]' });
  assert.match(task.stderr, /task 1: timeout must be a number of seconds above 0/);
  assert.equal(calls().length, 0);
});

test('words after -- are the prompt, even ones that look like options', async () => {
  const r = await ask(['-m', 'fake:small', '--', '-w', 'is a flag']);
  assert.equal(r.stdout, 'fake: -w is a flag\n');
  assert.equal(calls()[0].access, 'read');
});

test('schema checks use own properties and compare enums by value', async () => {
  writeJson(join(tmp, 'req.json'), { type: 'object', required: ['constructor'] });
  const missing = await ask(['-m', 'fake:small', '--schema', join(tmp, 'req.json'), 'go'], { extra: { FAKE_ANSWER: '{}' } });
  assert.match(missing.stderr, /missing "constructor"/);
  writeJson(join(tmp, 'enum.json'), { enum: [{ a: 1, b: 2 }] });
  const reordered = await ask(['-m', 'fake:small', '--schema', join(tmp, 'enum.json'), 'go'], { extra: { FAKE_ANSWER: '{"b": 2, "a": 1}' } });
  assert.equal(reordered.code, 0);
});

test('a report that is not an object is ignored, not fatal', async () => {
  writeFileSync(join(env.ASK_HOME, 'harnesses', 'odd'), '#!/bin/sh\necho null > "$ASK_REPORT"; echo fine\n', { mode: 0o755 });
  const r = await ask(['-m', 'odd:x', 'go']);
  assert.equal(r.code, 0);
  assert.equal(r.stdout, 'fine\n');
});
