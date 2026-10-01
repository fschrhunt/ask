/* What makes a run a subagent: follow-ups (-c), what a write run changed, worktrees, show and stop. */
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';
import { ask, calls, env, start, tmp, until, writeJson } from './helpers.js';

/* The run id from a status line, or the folder of the newest run. */
const runId = (stderr) => /^ask (\w{6}) /m.exec(stderr)[1];
const latestRun = () => readdirSync(join(env.ASK_HOME, 'runs')).sort().pop();

/* A git repository in tmp/repo with one committed file, a.txt. */
function repo() {
  const dir = join(tmp, 'repo');
  mkdirSync(dir);
  const git = (...args) => execFileSync('git', ['-C', dir, ...args], { stdio: 'ignore' });
  git('init', '-q', '-b', 'main');
  git('config', 'user.email', 'test@example.com');
  git('config', 'user.name', 'Test');
  writeFileSync(join(dir, 'a.txt'), 'one\n');
  git('add', '.');
  git('commit', '-qm', 'first');
  return { dir, git: (...args) => execFileSync('git', ['-C', dir, ...args], { encoding: 'utf8' }) };
}

test('-c continues the agent session where it ran, with the same model', async () => {
  mkdirSync(join(tmp, 'work'));
  const first = await ask(['-m', 'fake:big#high', '-C', join(tmp, 'work'), 'remember PELICAN']);
  const second = await ask(['-c', runId(first.stderr), 'what word?']);
  assert.equal(second.code, 0);
  const [a, b] = calls();
  assert.equal(b.session, `s-${a.pid}`);
  assert.equal(b.cwd, join(tmp, 'work'));
  assert.equal(b.model, 'big');
  assert.equal(b.effort, 'high');
  assert.equal(b.stdin, 'what word?');
  assert.match(second.stderr, new RegExp(`continues ${runId(first.stderr)}`));
});

test('a follow-up keeps write access unless it says -r, and may change the model within the harness', async () => {
  const first = await ask(['-m', 'fake:big', '-w', 'change it']);
  await ask(['-c', runId(first.stderr), 'more']);
  await ask(['-c', runId(first.stderr), '-r', '-m', 'fake:small', 'look']);
  assert.deepEqual(calls().map((c) => [c.access, c.model]), [['write', 'big'], ['write', 'big'], ['read', 'small']]);
});

test('a follow-up to another harness, or to a run with no session, is refused', async () => {
  writeFileSync(join(env.ASK_HOME, 'harnesses', 'other'), '#!/bin/sh\necho hi\n', { mode: 0o755 });
  const fake = await ask(['-m', 'fake:small', 'hi']);
  const other = await ask(['-m', 'other:x', 'hi']);
  const switched = await ask(['-c', runId(fake.stderr), '-m', 'other:x', 'more']);
  assert.equal(switched.code, 2);
  assert.match(switched.stderr, /must use the same harness/);
  const sessionless = await ask(['-c', runId(other.stderr), 'more']);
  assert.equal(sessionless.code, 2);
  assert.match(sessionless.stderr, /reported no session/);
});

test('a run that timed out can still be continued', async () => {
  const first = await ask(['-m', 'fake:small', '-t', '0.5', 'hang'], { extra: { FAKE_HANG: 'hang' } });
  assert.match(first.stderr, /timed out/);
  const second = await ask(['-c', runId(first.stderr), 'finish']);
  assert.equal(second.code, 0);
  assert.equal(calls()[1].session, `s-${calls()[0].pid}`);
});

test('a batch task is continued as RUN/TASK, from a follow-up or another batch', async () => {
  const batch = await ask(['batch', '-m', 'fake:small', '-'], { input: '[{"id": "a", "prompt": "x"}, {"id": "b", "prompt": "y"}]' });
  const id = runId(batch.stderr);
  const bare = await ask(['-c', id, 'more']);
  assert.equal(bare.code, 2);
  assert.match(bare.stderr, new RegExp(`continue one of them, like ${id}/a`));
  await ask(['-c', `${id}/b`, 'more']);
  await ask(['batch', '-'], { input: JSON.stringify([{ continue: `${id}/a`, prompt: 'again' }]) });
  const by = (prompt) => calls().find((c) => c.stdin.endsWith(prompt));
  assert.equal(by('more').session, `s-${by('y').pid}`);
  assert.equal(by('again').session, `s-${by('x').pid}`);
});

test('a write run reports the files it changed, not ones already changed before it', async () => {
  const { dir } = repo();
  writeFileSync(join(dir, 'a.txt'), 'edited before\n');
  writeFileSync(join(dir, 'untouched.txt'), 'new before\n');
  const r = await ask(['batch', '-w', '-m', 'fake:small', '-C', dir, '-'], { input: '[{"prompt": "go"}]', extra: { FAKE_WRITE: 'b.txt=new' } });
  const [result] = JSON.parse(r.stdout);
  assert.deepEqual(result.changes, [{ path: 'b.txt', change: 'added' }]);
  assert.equal(result.commits, 0);
  assert.match(r.stderr, / · ok · [\d.]+s · 1 file changed · /);
});

test('a write run that commits reports its commits and the files they changed', async () => {
  const { dir } = repo();
  writeFileSync(join(dir, 'a.txt'), 'two\n');
  const r = await ask(['batch', '-w', '-m', 'fake:small', '-C', dir, '-'], { input: '[{"prompt": "go"}]', extra: { FAKE_COMMIT: '1' } });
  const [result] = JSON.parse(r.stdout);
  assert.deepEqual(result.changes, []);
  assert.equal(result.commits, 1);
  const edits = await ask(['batch', '-w', '-m', 'fake:small', '-C', dir, '-'], { input: '[{"prompt": "go"}]', extra: { FAKE_WRITE: 'a.txt=three', FAKE_COMMIT: '1' } });
  assert.deepEqual(JSON.parse(edits.stdout)[0].changes, [{ path: 'a.txt', change: 'modified' }]);
});

test('--worktree runs in a new worktree and branch, kept with the changes, leaving the checkout alone', async () => {
  const { dir, git } = repo();
  const r = await ask(['-m', 'fake:small', '-w', '--worktree', '-C', dir, 'go'], { extra: { FAKE_WRITE: 'b.txt=new' } });
  assert.equal(r.code, 0);
  const id = runId(r.stderr);
  const path = join(env.ASK_HOME, 'worktrees', id);
  assert.equal(calls()[0].cwd, path);
  assert.equal(readFileSync(join(path, 'b.txt'), 'utf8'), 'new');
  assert.ok(!existsSync(join(dir, 'b.txt')));
  assert.match(git('branch', '--list', `ask/${id}`), new RegExp(`ask/${id}`));
  assert.match(r.stderr, new RegExp(`branch ask/${id}`));
});

test('a worktree run that changed nothing removes its worktree and branch, and a follow-up recreates it', async () => {
  const { dir, git } = repo();
  const r = await ask(['-m', 'fake:small', '-w', '--worktree', '-C', dir, 'look']);
  const id = runId(r.stderr);
  assert.ok(!existsSync(join(env.ASK_HOME, 'worktrees', id)));
  assert.equal(git('branch', '--list', `ask/${id}`).trim(), '');
  await ask(['-c', id, 'now change it'], { extra: { FAKE_WRITE: 'c.txt=x' } });
  assert.equal(calls()[1].cwd, join(env.ASK_HOME, 'worktrees', id));
  assert.match(git('branch', '--list', `ask/${id}`), new RegExp(`ask/${id}`));
});

test('--worktree needs -w and a git repository', async () => {
  const read = await ask(['-m', 'fake:small', '--worktree', 'go']);
  assert.equal(read.code, 2);
  assert.match(read.stderr, /a worktree is for write runs; add -w/);
  const outside = await ask(['-m', 'fake:small', '-w', '--worktree', '-C', tmp, 'go']);
  assert.equal(outside.code, 1);
  assert.match(outside.stderr, /--worktree needs a git repository/);
});

test('show prints a run again: its answer, or with --json its whole result', async () => {
  const first = await ask(['-m', 'fake:small', 'hello']);
  const id = runId(first.stderr);
  const shown = await ask(['show', id]);
  assert.equal(shown.stdout, 'fake: hello\n');
  assert.match(shown.stderr, new RegExp(`^ask ${id} · Fake 1\\.0 · ok`));
  const full = JSON.parse((await ask(['show', id, '--json'])).stdout);
  assert.equal(full.run, id);
  assert.equal(full.session, `s-${calls()[0].pid}`);
});

test('stop stops a run that is going, and its agents', async () => {
  const run = start(['-m', 'fake:small', 'hang'], { extra: { FAKE_HANG: 'hang' } });
  await until(() => calls().length === 1);
  const id = latestRun().split('-').pop();
  const stopped = await ask(['stop', id]);
  assert.match(stopped.stderr, new RegExp(`^ask ${id} · stopped`));
  assert.equal((await run).code, 130);
  assert.equal((await ask(['stop', id])).code, 1);
});

test('a run another ask is running cannot be resumed at the same time', async () => {
  const run = start(['batch', '-m', 'fake:small', '-'], { input: '[{"prompt": "hang"}]', extra: { FAKE_HANG: 'hang' } });
  await until(() => calls().length === 1);
  const id = latestRun().split('-').pop();
  const second = await ask(['batch', '--resume', id]);
  assert.equal(second.code, 2);
  assert.match(second.stderr, new RegExp(`run ${id} is running`));
  run.child.kill('SIGTERM');
  await run;
});

test('a damaged results.json is reported, never taken as no results', async () => {
  const first = await ask(['batch', '-m', 'fake:small', '-'], { input: '[{"prompt": "a"}]' });
  const id = runId(first.stderr);
  const dir = join(env.ASK_HOME, 'runs', latestRun());
  writeFileSync(join(dir, 'results.json'), '[{"ok": tr');
  const r = await ask(['batch', '--resume', id]);
  assert.equal(r.code, 2);
  assert.match(r.stderr, /results\.json is damaged/);
  assert.equal(calls().length, 1);
});

test('--resume reruns a batch as recorded, refusing options that would change it', async () => {
  const first = await ask(['batch', '-m', 'fake:small', '-'], { input: '[{"prompt": "a"}]' });
  const r = await ask(['batch', '--resume', runId(first.stderr), '-r']);
  assert.equal(r.code, 2);
  assert.match(r.stderr, /takes only -j, not -r/);
});

test('a harness inherits no contract variable from an ask further up', async () => {
  writeJson(join(tmp, 's.json'), { type: 'object' });
  await ask(['-m', 'fake:small', 'hi'], { extra: { ASK_SCHEMA: join(tmp, 's.json'), ASK_SESSION: 'leaked' } });
  assert.equal(calls()[0].schema, undefined);
  assert.equal(calls()[0].session, undefined);
});
