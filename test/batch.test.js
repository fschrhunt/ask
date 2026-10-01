/* Batches: parallel runs, task files, recorded runs, --resume and `ask runs`. */
import assert from 'node:assert/strict';
import { join } from 'node:path';
import { test } from 'node:test';
import { tmp, env, ask, calls, writeJson, runDirs, runFile } from './helpers.js';

test('batch keeps task order and records tasks.json and results.json', async () => {
  const tasks = [
    { id: 'slow', prompt: 'one slow', model: 'fake:small' },
    { id: 'fast', prompt: 'two', model: 'fake:big' },
  ];
  const r = await ask(['batch', '-j', '2', '-'], { input: JSON.stringify(tasks), extra: { FAKE_SLOW: 'slow' } });
  assert.equal(r.code, 0);
  const results = JSON.parse(r.stdout);
  assert.deepEqual(results.map((x) => [x.id, x.ok, x.answer]), [['slow', true, 'fake: one slow'], ['fast', true, 'fake: two']]);
  assert.equal(runDirs().length, 1);
  assert.deepEqual(runFile('tasks.json').map((t) => t.id), ['slow', 'fast']);
  assert.deepEqual(runFile('results.json').map((x) => x.id), ['slow', 'fast']);
});

test('batch accepts JSON lines, with -m as the default model and a task model winning', async () => {
  const lines = ['{"prompt": "a"}', '{"prompt": "b", "model": "fake:big"}'].join('\n');
  const r = await ask(['batch', '-m', 'fake:small'], { input: lines });
  assert.deepEqual(JSON.parse(r.stdout).map((x) => [x.id, x.model]), [['1', 'fake:small'], ['2', 'fake:big']]);
});

test('a batch task whose write is not a boolean is a usage error', async () => {
  const r = await ask(['batch', '-m', 'fake:small'], { input: '[{"prompt": "a", "write": "false"}]' });
  assert.equal(r.code, 2);
  assert.match(r.stderr, /"write" must be true or false/);
  assert.equal(calls().length, 0);
});

test('batch -w is the default write access, and a task may override it', async () => {
  const tasks = [{ prompt: 'a' }, { prompt: 'b', write: false }];
  await ask(['batch', '-w', '-m', 'fake:small'], { input: JSON.stringify(tasks) });
  assert.deepEqual(calls().map((call) => call.access).sort(), ['read', 'write']);
  assert.deepEqual(runFile('tasks.json').map((t) => t.write), [true, false]);
});

test('a batch task without a model is a usage error that lists the models', async () => {
  const r = await ask(['batch'], { input: '[{"prompt": "a"}]' });
  assert.equal(r.code, 2);
  assert.match(r.stderr, /task 1 needs a model/);
  assert.match(r.stderr, /fake:small/);
});

test('an empty batch is a usage error', async () => {
  for (const input of ['', '[]']) {
    const r = await ask(['batch', '-m', 'fake:small'], { input });
    assert.equal(r.code, 2, JSON.stringify(input));
    assert.match(r.stderr, /no tasks/);
  }
});

test('a failed batch task is reported and the batch exits 1', async () => {
  const tasks = [{ prompt: 'fine' }, { prompt: 'break' }];
  const r = await ask(['batch', '-m', 'fake:small'], { input: JSON.stringify(tasks), extra: { FAKE_FAIL: 'break' } });
  assert.equal(r.code, 1);
  const [ok, failed] = JSON.parse(r.stdout);
  assert.equal(ok.ok, true);
  assert.equal(failed.ok, false);
  assert.equal(failed.error, 'fake boom');
});

test('batch --resume reruns only the tasks that did not finish', async () => {
  const tasks = [{ prompt: 'fine' }, { prompt: 'break' }];
  await ask(['batch', '-m', 'fake:small'], { input: JSON.stringify(tasks), extra: { FAKE_FAIL: 'break' } });
  assert.equal(calls().length, 2);
  const r = await ask(['batch', '--resume', join(env.ASK_HOME, 'runs', runDirs()[0])]);
  assert.equal(r.code, 0);
  assert.equal(calls().length, 3);
  assert.match(calls()[2].stdin, /break$/);
  assert.deepEqual(JSON.parse(r.stdout).map((x) => x.ok), [true, true]);
});

test('batch --resume reruns a failed task even when a finished one shares its id', async () => {
  const tasks = [{ id: 'x', prompt: 'fine' }, { id: 'x', prompt: 'break' }];
  await ask(['batch', '-m', 'fake:small'], { input: JSON.stringify(tasks), extra: { FAKE_FAIL: 'break' } });
  const r = await ask(['batch', '--resume', join(env.ASK_HOME, 'runs', runDirs()[0])]);
  assert.equal(calls().length, 3);
  assert.match(calls()[2].stdin, /break$/);
  assert.deepEqual(JSON.parse(r.stdout).map((x) => x.answer), ['fake: fine', 'fake: break']);
});

test('batch --resume of an unknown run is a usage error', async () => {
  const r = await ask(['batch', '--resume', join(tmp, 'nope')]);
  assert.equal(r.code, 2);
  assert.match(r.stderr, /no run to resume/);
});

test('runs lists recorded runs with their counts', async () => {
  const tasks = [{ prompt: 'fine' }, { prompt: 'break' }];
  await ask(['batch', '-m', 'fake:small'], { input: JSON.stringify(tasks), extra: { FAKE_FAIL: 'break' } });
  const r = await ask(['runs']);
  assert.equal(r.stdout.trim().split('\t').slice(1).join(','), '1 ok,1 failed,0 pending');
  assert.ok(r.stdout.startsWith(join(env.ASK_HOME, 'runs')));
});

test('runs shows an unfinished run whose ask is gone as stopped, with how to resume', async () => {
  const dir = join(env.ASK_HOME, 'runs', '20260101T000000-999999999');
  writeJson(join(dir, 'tasks.json'), [{ id: '1', model: 'fake:small', prompt: 'p' }, { id: '2', model: 'fake:small', prompt: 'p' }]);
  writeJson(join(dir, 'results.json'), [{ id: '1', ok: true }, null]);
  const r = await ask(['runs']);
  assert.match(r.stdout, /1 ok\t0 failed\t1 stopped \(ask batch --resume .*999999999\)/);
});
