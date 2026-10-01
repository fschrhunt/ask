/* Making ask yours: hooks, commands, packages, and the contract version every executable gets. */
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';
import { ask, calls, env, tmp } from './helpers.js';

/* Installs an executable in ASK_HOME/KIND (or in DIR/KIND) whose body is Node code. */
function install(kind, name, body, dir = env.ASK_HOME) {
  mkdirSync(join(dir, kind), { recursive: true });
  writeFileSync(join(dir, kind, name), `#!/usr/bin/env node\n${body}\n`, { mode: 0o755 });
}

/*
 * A hook handling `events`, whose answer to an event is `answer(input)`, written as Node code over
 * `input` (the parsed stdin). It appends each call to $HOME/hook-calls.
 */
function hook(name, events, answer) {
  install(
    'hooks',
    name,
    `import { appendFileSync, readFileSync } from 'node:fs';
const event = process.argv[2];
if (event === 'events') { console.log(${JSON.stringify(events.join('\n'))}); process.exit(0); }
const input = JSON.parse(readFileSync(0, 'utf8'));
appendFileSync(process.env.HOME + '/hook-calls', JSON.stringify({ name: ${JSON.stringify(name)}, event, input, run: process.env.ASK_RUN }) + '\\n');
const answer = (${answer})(input, event);
if (answer !== undefined) console.log(typeof answer === 'string' ? answer : JSON.stringify(answer));`,
  );
}
const hookCalls = () => (existsSync(join(tmp, 'hook-calls')) ? readFileSync(join(tmp, 'hook-calls'), 'utf8').trim().split('\n').map((l) => JSON.parse(l)) : []);

test('task hooks run in name order, each changing the task the next one sees', async () => {
  hook('a-route', ['task'], `(i) => ({ task: { model: 'fake:big', prompt: i.task.prompt + ' (routed)' } })`);
  hook('b-context', ['task'], `(i) => ({ task: { prompt: 'Context first. ' + i.task.prompt } })`);
  const r = await ask(['-m', 'fake:small', '-w', 'fix it']);
  assert.equal(r.code, 0);
  assert.equal(calls()[0].model, 'big');
  assert.equal(calls()[0].stdin, 'Context first. fix it (routed)');
  assert.equal(hookCalls()[1].input.task.prompt, 'fix it (routed)');
});

test('a task hook can refuse a task, which then never runs', async () => {
  hook('guard', ['task'], `(i) => (i.task.write ? { refuse: 'no writes here' } : undefined)`);
  const r = await ask(['-m', 'fake:small', '-w', 'delete everything']);
  assert.equal(r.code, 1);
  assert.equal(calls().length, 0);
  assert.match(r.stderr, / · failed · .* · refused by hook guard: no writes here$/m);
});

test('a hook that crashes or prints nonsense changes nothing and leaves a note', async () => {
  hook('broken', ['task'], `() => { throw new Error('boom') }`);
  hook('chatty', ['task'], `() => 'not json'`);
  hook('typo', ['task'], `() => ({ task: { write: 'yes', dir: '/' } })`);
  const r = await ask(['-m', 'fake:small', 'look']);
  assert.equal(r.code, 0);
  assert.equal(calls()[0].access, 'read');
  assert.match(r.stderr, /^ask \w{6} · hook broken · failed: .*boom/m);
  assert.match(r.stderr, /^ask \w{6} · hook chatty · failed: printed something other than JSON$/m);
  assert.match(r.stderr, /^ask \w{6} · hook typo · ignored a change to "write"$/m);
  assert.match(r.stderr, /^ask \w{6} · hook typo · ignored a change to "dir"$/m);
});

test('a result hook can ask the same agent for a follow-up, and the rounds make one result', async () => {
  hook('verify', ['result'], `(i) => (i.result.answer.includes('try again') ? undefined : { followup: 'try again', note: 'tests fail' })`);
  const r = await ask(['batch', '-m', 'fake:small', '-'], { input: '[{"prompt": "fix the bug"}]' });
  const [result] = JSON.parse(r.stdout);
  const [first, second] = calls();
  assert.equal(second.session, `s-${first.pid}`);
  assert.equal(second.stdin, 'try again');
  assert.equal(result.answer, 'fake: try again');
  assert.equal(result.followups, 1);
  assert.deepEqual(result.usage, { input: 20, output: 10, cached: 4, cost: 0.02 });
  assert.match(r.stderr, /^ask \w{6} · hook verify · tests fail$/m);
  assert.match(r.stderr, /^ask \w{6} · hook verify · follow-up: try again$/m);
});

test('follow-ups stop after three, so a hook that is never satisfied cannot loop forever', async () => {
  hook('never', ['result'], `() => ({ followup: 'again' })`);
  const r = await ask(['-m', 'fake:small', 'go']);
  assert.equal(calls().length, 4);
  assert.match(r.stderr, /hook never · asked for a follow-up after 3; stopping/);
});

test('a result hook can fail a result', async () => {
  hook('strict', ['result'], `() => ({ fail: 'no tests were run' })`);
  const r = await ask(['-m', 'fake:small', 'go']);
  assert.equal(r.code, 1);
  assert.equal(r.stdout, '');
  assert.match(r.stderr, / · failed · .* · failed by hook strict: no tests were run$/m);
});

test('a hook gets only the events it declares, and --no-hooks skips them all', async () => {
  hook('after', ['result'], `() => undefined`);
  await ask(['-m', 'fake:small', 'go']);
  assert.deepEqual(hookCalls().map((c) => c.event), ['result']);
  assert.match(hookCalls()[0].run, /^\w{6}$/);
  await ask(['-m', 'fake:small', '--no-hooks', 'go']);
  assert.equal(hookCalls().length, 1);
});

test('a command runs as ask NAME with its arguments and exit code, and can run ask itself', async () => {
  install(
    'commands',
    'twice',
    `// ask-command: ask the same question twice
import { execFileSync } from 'node:child_process';
for (let i = 0; i < 2; i++) process.stdout.write(execFileSync(process.env.ASK_BIN, ['-m', 'fake:small', ...process.argv.slice(2)], { stdio: ['ignore', 'pipe', 'ignore'] }));
process.exitCode = 3;`,
  );
  const r = await ask(['twice', 'hello']);
  assert.equal(r.code, 3);
  assert.equal(r.stdout, 'fake: hello\nfake: hello\n');
  assert.match((await ask(['--help'])).stdout, /your commands\n {2}ask twice {2}ask the same question twice/);
});

test("ask's own commands win over yours of the same name", async () => {
  install('commands', 'models', `console.log('mine')`);
  assert.match((await ask(['models'])).stdout, /^fake:small$/m);
});

test('a package from git adds agents, hooks and commands; yours win; install alone updates it', async () => {
  const repo = join(tmp, 'src', 'team', 'tools');
  mkdirSync(repo, { recursive: true });
  const git = (...args) => execFileSync('git', ['-C', repo, ...args], { stdio: 'ignore' });
  git('init', '-q', '-b', 'main');
  install('agents', 'fake', `console.log('package agent')`, repo);
  install('commands', 'hello', `console.log('hello v1')`, repo);
  git('add', '.');
  git('-c', 'user.email=t@t', '-c', 'user.name=t', 'commit', '-qm', 'v1');

  const installed = await ask(['install', repo]);
  assert.match(installed.stderr, /installed local\/team\/tools: agents: fake · commands: hello/);
  assert.equal((await ask(['hello'])).stdout, 'hello v1\n');
  assert.equal((await ask(['-m', 'fake:small', 'hi'])).stdout, 'fake: hi\n');
  assert.match((await ask(['packages'])).stdout, /^local\/team\/tools\tagents: fake · commands: hello$/m);

  install('commands', 'hello', `console.log('hello v2')`, repo);
  git('-c', 'user.email=t@t', '-c', 'user.name=t', 'commit', '-qam', 'v2');
  assert.match((await ask(['install'])).stderr, /local\/team\/tools: updated/);
  assert.equal((await ask(['hello'])).stdout, 'hello v2\n');

  assert.match((await ask(['remove', 'tools'])).stderr, /removed local\/team\/tools/);
  assert.equal((await ask(['hello'])).code, 2);
});

test('every agent, hook and command gets the contract version as ASK_CONTRACT', async () => {
  hook('seen', ['task'], `() => ({ note: 'contract ' + process.env.ASK_CONTRACT })`);
  const r = await ask(['-m', 'fake:small', 'go']);
  assert.equal(calls()[0].contract, '1');
  assert.match(r.stderr, /hook seen · contract 1/);
});
