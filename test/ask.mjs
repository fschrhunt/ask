/*
 * Hermetic tests for ask: the real `ask` runs against the fake claude, codex and opencode in
 * test/bin, with ASK_HOME, HOME and CODEX_HOME inside a temp dir. No network, no real models.
 * A fake records each call in $FAKE_LOG; see the fakes for the env vars that make them fail or hang.
 */
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { afterEach, beforeEach, test } from 'node:test';
import { fileURLToPath } from 'node:url';

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..');
const PATH = `${join(ROOT, 'test/bin')}:${dirname(process.execPath)}`;
let tmp;
let env;

beforeEach(() => {
  tmp = realpathSync(mkdtempSync(join(tmpdir(), 'ask-test-')));
  env = { PATH, HOME: tmp, ASK_HOME: join(tmp, 'home'), CODEX_HOME: join(tmp, 'codex'), FAKE_LOG: join(tmp, 'calls.jsonl') };
});
afterEach(() => rmSync(tmp, { recursive: true, force: true }));

/* Starts ask; resolves with { code, stdout, stderr } and exposes the child as `child`. */
function start(args, { input = '', extra = {} } = {}) {
  const child = spawn(process.execPath, [join(ROOT, 'ask'), ...args], { cwd: tmp, env: { ...env, ...extra } });
  const done = new Promise((resolve) => {
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (d) => (stdout += d));
    child.stderr.on('data', (d) => (stderr += d));
    child.on('close', (code) => resolve({ code, stdout, stderr }));
  });
  child.stdin.end(input);
  return Object.assign(done, { child });
}
const ask = (args, options) => start(args, options);

const calls = () => {
  try {
    return readFileSync(env.FAKE_LOG, 'utf8').trim().split('\n').filter(Boolean).map((line) => JSON.parse(line));
  } catch {
    return [];
  }
};
const alive = (pid) => {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
};
const until = async (condition) => {
  for (let i = 0; i < 100 && !condition(); i++) await new Promise((resolve) => setTimeout(resolve, 50));
  assert.ok(condition(), 'timed out waiting');
};
const writeJson = (path, value) => {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, JSON.stringify(value));
};
const runDirs = () => readdirSync(join(env.ASK_HOME, 'runs'));
const runFile = (name) => JSON.parse(readFileSync(join(env.ASK_HOME, 'runs', runDirs()[0], name), 'utf8'));
const after = (argv, flag) => argv[argv.indexOf(flag) + 1];

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

test('an unknown harness is a usage error', async () => {
  const r = await ask(['-m', 'gemini:x', 'hello']);
  assert.equal(r.code, 2);
  assert.match(r.stderr, /expected harness:id/);
});

test('-r and -w together are refused', async () => {
  const r = await ask(['-m', 'claude:opus', '-r', '-w', 'hello']);
  assert.equal(r.code, 2);
  assert.match(r.stderr, /choose one of -r/);
  assert.equal(calls().length, 0);
});

test('models lists Claude aliases, the Codex cache and models.json, without review slugs', async () => {
  writeJson(join(env.CODEX_HOME, 'models_cache.json'), { models: [{ slug: 'gpt-x' }, { slug: 'gpt-x-review' }] });
  writeJson(join(env.ASK_HOME, 'models.json'), { opencode: ['p/m'] });
  const r = await ask(['models']);
  assert.equal(r.code, 0);
  assert.deepEqual(r.stdout.trim().split('\n'), ['claude:fable', 'claude:opus', 'claude:sonnet', 'claude:haiku', 'codex:gpt-x', 'opencode:p/m']);
});

test('models works without models.json or a Codex cache', async () => {
  const r = await ask(['models']);
  assert.equal(r.code, 0);
  assert.deepEqual(r.stdout.trim().split('\n'), ['claude:fable', 'claude:opus', 'claude:sonnet', 'claude:haiku']);
});

test('a models.json that does not parse is a usage error', async () => {
  mkdirSync(env.ASK_HOME, { recursive: true });
  writeFileSync(join(env.ASK_HOME, 'models.json'), '{');
  const r = await ask(['models']);
  assert.equal(r.code, 2);
  assert.match(r.stderr, /cannot parse/);
});

test('claude read runs get the read-only flags and the grounding preamble', async () => {
  const r = await ask(['-m', 'claude:opus', 'where is main?']);
  assert.equal(r.code, 0);
  const [call] = calls();
  assert.equal(after(call.argv, '--permission-mode'), 'default');
  assert.match(after(call.argv, '--allowedTools'), /Read,Grep,Glob,Bash\(git diff:\*\)/);
  assert.match(after(call.argv, '--disallowedTools'), /^Edit,Write,NotebookEdit,/);
  assert.match(call.stdin, /^Answer from the files in your working directory/);
  assert.match(call.stdin, /where is main\?$/);
});

test('claude write runs get the write flags and no preamble', async () => {
  await ask(['-m', 'claude:opus', '-w', 'fix it']);
  const [call] = calls();
  assert.equal(after(call.argv, '--permission-mode'), 'bypassPermissions');
  assert.ok(!call.argv.includes('--allowedTools'));
  assert.equal(call.stdin, 'fix it');
});

test('codex runs use the read-only sandbox, or workspace-write with -w', async () => {
  await ask(['-m', 'codex:gpt-x', 'look']);
  await ask(['-m', 'codex:gpt-x', '-w', 'change']);
  const [read, write] = calls();
  assert.equal(after(read.argv, '--sandbox'), 'read-only');
  assert.equal(after(write.argv, '--sandbox'), 'workspace-write');
  assert.ok(write.argv.includes('sandbox_workspace_write.network_access=true'));
  assert.ok(!read.argv.includes('sandbox_workspace_write.network_access=true'));
  assert.ok(read.argv.includes('approval_policy="never"'));
  assert.equal(after(read.argv, '-m'), 'gpt-x');
});

test('opencode read runs use the plan agent with injected read-only permissions', async () => {
  await ask(['-m', 'opencode:p/m', 'look']);
  const [call] = calls();
  assert.equal(after(call.argv, '--agent'), 'plan');
  const { steps, permissions } = call.config.agents.plan;
  assert.equal(steps, 25);
  assert.deepEqual(permissions[0], { action: '*', resource: '*', effect: 'deny' });
  assert.ok(permissions.some((p) => p.action === 'shell' && p.resource === 'git diff *' && p.effect === 'allow'));
  for (const token of ['>', '|', ';', '&', '`', '$']) {
    assert.ok(permissions.some((p) => p.resource === `*${token}*` && p.effect === 'deny'), token);
  }
});

test('read runs refuse writing options, quoting and escaping, and git grep', async () => {
  await ask(['-m', 'claude:opus', 'look']);
  await ask(['-m', 'opencode:p/m', 'look']);
  const [claude, opencode] = calls();
  const claudeDenied = after(claude.argv, '--disallowedTools').split(',');
  const opencodeDenied = opencode.config.agents.plan.permissions.filter((p) => p.effect === 'deny').map((p) => p.resource);
  for (const token of ['--output', '--ext-diff', '--textconv', '--pre', '--hostname-bin', "'", '"', '{']) {
    assert.ok(claudeDenied.includes(`Bash(*${token}*)`), token);
    assert.ok(opencodeDenied.includes(`*${token}*`), token);
  }
  // Claude Code only matches a backslash written as four in its rule.
  assert.ok(claudeDenied.includes('Bash(*\\\\\\\\*)'));
  assert.ok(opencodeDenied.includes('*\\*'));
  assert.ok(!after(claude.argv, '--allowedTools').includes('git grep'));
  assert.ok(!opencode.config.agents.plan.permissions.some((p) => p.resource.startsWith('git grep')));
});

test('opencode write runs use the build agent with the larger step cap', async () => {
  await ask(['-m', 'opencode:p/m', '-w', 'change']);
  const [call] = calls();
  assert.equal(after(call.argv, '--agent'), 'build');
  assert.deepEqual(call.config, { agents: { build: { steps: 100 } } });
});

test('an opencode run that hits the step cap says its answer may be partial', async () => {
  const r = await ask(['-m', 'opencode:p/m', 'look'], { extra: { FAKE_STEPS: '25' } });
  assert.equal(r.code, 0);
  assert.match(r.stderr, /hit step cap/);
});

test('the answer goes to stdout and the status line to stderr', async () => {
  const r = await ask(['-m', 'claude:opus', 'hi']);
  assert.equal(r.stdout, 'claude: hi\n');
  assert.match(r.stderr, /^ask: Opus 5\.5 [\d.]+s /);
});

for (const [model, usage] of [
  ['claude:opus', '35 in 7 out $0.0100'],
  ['codex:gpt-x', '100 in 10 out'],
  ['opencode:p/m', '50 in 7 out $0.0020'],
]) {
  test(`usage is reported for ${model.split(':')[0]}`, async () => {
    const r = await ask(['-m', model, 'hi']);
    assert.ok(r.stderr.includes(usage), r.stderr);
  });
}

test('a failing model exits 1 with its message', async () => {
  const r = await ask(['-m', 'codex:gpt-x', 'break'], { extra: { FAKE_FAIL: 'break' } });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /failed after .*codex boom/);
  assert.equal(r.stdout, '');
});

test('--schema answers are parsed and printed as JSON', async () => {
  writeJson(join(tmp, 's.json'), { type: 'object', properties: { a: { type: 'number' } } });
  const r = await ask(['-m', 'claude:opus', '--schema', join(tmp, 's.json'), 'go'], { extra: { FAKE_ANSWER: '{"a": 1}' } });
  assert.equal(r.code, 0);
  assert.deepEqual(JSON.parse(r.stdout), { a: 1 });
  assert.ok(calls()[0].argv.includes('--json-schema'));
});

test('an opencode answer that does not match the schema fails', async () => {
  writeJson(join(tmp, 's.json'), { type: 'object', properties: { a: { type: 'number' } }, required: ['a'] });
  const r = await ask(['-m', 'opencode:p/m', '--schema', join(tmp, 's.json'), 'go'], { extra: { FAKE_ANSWER: '{"a": "one"}' } });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /does not match the schema: \$\.a: expected number, got string/);
  assert.equal(r.stdout, '');
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

test('codex gets a strict schema: objects closed, every property required', async () => {
  writeJson(join(tmp, 's.json'), { type: 'object', properties: { a: { type: 'string' }, b: { type: 'object', properties: { c: { type: 'number' } } } } });
  await ask(['-m', 'codex:gpt-x', '--schema', join(tmp, 's.json'), 'go'], { extra: { FAKE_ANSWER: '{}' } });
  assert.deepEqual(calls()[0].schema, {
    type: 'object',
    properties: { a: { type: 'string' }, b: { type: 'object', properties: { c: { type: 'number' } }, required: ['c'], additionalProperties: false } },
    required: ['a', 'b'],
    additionalProperties: false,
  });
});

test('codex strict schemas leave a property named "properties" alone', async () => {
  writeJson(join(tmp, 's.json'), { type: 'object', properties: { properties: { type: 'string' } } });
  await ask(['-m', 'codex:gpt-x', '--schema', join(tmp, 's.json'), 'go'], { extra: { FAKE_ANSWER: '{"properties": "x"}' } });
  assert.deepEqual(calls()[0].schema.properties, { properties: { type: 'string' } });
});

test('a relative -C is resolved once, against where ask runs', async () => {
  mkdirSync(join(tmp, 'work'));
  const r = await ask(['-m', 'codex:gpt-x', '-C', 'work', 'hi']);
  assert.equal(r.code, 0);
  const [call] = calls();
  assert.equal(call.cwd, join(tmp, 'work'));
  assert.equal(after(call.argv, '-C'), join(tmp, 'work'));
});

for (const model of ['claude:opus', 'codex:gpt-x', 'opencode:p/m']) {
  test(`a ${model.split(':')[0]} answer from a run that exited nonzero fails`, async () => {
    const r = await ask(['-m', model, 'hi'], { extra: { FAKE_EXIT: '1' } });
    assert.equal(r.code, 1);
    assert.match(r.stderr, /failed after .*exit 1/);
  });
}

test('-C sets the directory the model works in', async () => {
  mkdirSync(join(tmp, 'work'));
  await ask(['-m', 'claude:opus', '-C', join(tmp, 'work'), 'hi']);
  assert.equal(calls()[0].cwd, join(tmp, 'work'));
});

test('an effort suffix reaches each harness that takes one', async () => {
  await ask(['-m', 'claude:opus#high', 'hi']);
  await ask(['-m', 'codex:gpt-x#low', 'hi']);
  const [claude, codex] = calls();
  assert.equal(after(claude.argv, '--effort'), 'high');
  assert.ok(codex.argv.includes('model_reasoning_effort="low"'));
});

test('the prompt is read from stdin when none is given', async () => {
  const r = await ask(['-m', 'claude:opus'], { input: 'from stdin\n' });
  assert.equal(r.stdout, 'claude: from stdin\n');
});

test('batch keeps task order and records tasks.json and results.json', async () => {
  const tasks = [
    { id: 'slow', prompt: 'one slow', model: 'claude:opus' },
    { id: 'fast', prompt: 'two', model: 'codex:gpt-x' },
  ];
  const r = await ask(['batch', '-j', '2', '-'], { input: JSON.stringify(tasks), extra: { FAKE_SLOW: 'slow' } });
  assert.equal(r.code, 0);
  const results = JSON.parse(r.stdout);
  assert.deepEqual(results.map((x) => [x.id, x.ok, x.answer]), [['slow', true, 'claude: one slow'], ['fast', true, 'codex: two']]);
  assert.equal(runDirs().length, 1);
  assert.deepEqual(runFile('tasks.json').map((t) => t.id), ['slow', 'fast']);
  assert.deepEqual(runFile('results.json').map((x) => x.id), ['slow', 'fast']);
});

test('batch accepts JSON lines, with -m as the default model and a task model winning', async () => {
  const lines = ['{"prompt": "a"}', '{"prompt": "b", "model": "codex:gpt-x"}'].join('\n');
  const r = await ask(['batch', '-m', 'claude:opus'], { input: lines });
  assert.deepEqual(JSON.parse(r.stdout).map((x) => [x.id, x.model]), [['1', 'claude:opus'], ['2', 'codex:gpt-x']]);
});

test('a batch task whose write is not a boolean is a usage error', async () => {
  const r = await ask(['batch', '-m', 'claude:opus'], { input: '[{"prompt": "a", "write": "false"}]' });
  assert.equal(r.code, 2);
  assert.match(r.stderr, /"write" must be true or false/);
  assert.equal(calls().length, 0);
});

test('batch -w is the default write access, and a task may override it', async () => {
  const tasks = [{ prompt: 'a' }, { prompt: 'b', write: false }];
  await ask(['batch', '-w', '-m', 'claude:opus'], { input: JSON.stringify(tasks) });
  const modes = calls().map((call) => after(call.argv, '--permission-mode')).sort();
  assert.deepEqual(modes, ['bypassPermissions', 'default']);
  assert.deepEqual(runFile('tasks.json').map((t) => t.write), [true, false]);
});

test('a batch task without a model is a usage error that lists the models', async () => {
  const r = await ask(['batch'], { input: '[{"prompt": "a"}]' });
  assert.equal(r.code, 2);
  assert.match(r.stderr, /task 1 needs a model/);
  assert.match(r.stderr, /claude:opus/);
});

test('an empty batch is a usage error', async () => {
  for (const input of ['', '[]']) {
    const r = await ask(['batch', '-m', 'claude:opus'], { input });
    assert.equal(r.code, 2, JSON.stringify(input));
    assert.match(r.stderr, /no tasks/);
  }
});

test('a failed batch task is reported and the batch exits 1', async () => {
  const tasks = [{ prompt: 'fine' }, { prompt: 'break' }];
  const r = await ask(['batch', '-m', 'claude:opus'], { input: JSON.stringify(tasks), extra: { FAKE_FAIL: 'break' } });
  assert.equal(r.code, 1);
  const [ok, failed] = JSON.parse(r.stdout);
  assert.equal(ok.ok, true);
  assert.equal(failed.ok, false);
  assert.equal(failed.error, 'claude boom');
});

test('batch --resume reruns only the tasks that did not finish', async () => {
  const tasks = [{ prompt: 'fine' }, { prompt: 'break' }];
  await ask(['batch', '-m', 'claude:opus'], { input: JSON.stringify(tasks), extra: { FAKE_FAIL: 'break' } });
  assert.equal(calls().length, 2);
  const r = await ask(['batch', '--resume', join(env.ASK_HOME, 'runs', runDirs()[0])]);
  assert.equal(r.code, 0);
  assert.equal(calls().length, 3);
  assert.match(calls()[2].stdin, /break$/);
  assert.deepEqual(JSON.parse(r.stdout).map((x) => x.ok), [true, true]);
});

test('batch --resume reruns a failed task even when a finished one shares its id', async () => {
  const tasks = [{ id: 'x', prompt: 'fine' }, { id: 'x', prompt: 'break' }];
  await ask(['batch', '-m', 'claude:opus'], { input: JSON.stringify(tasks), extra: { FAKE_FAIL: 'break' } });
  const r = await ask(['batch', '--resume', join(env.ASK_HOME, 'runs', runDirs()[0])]);
  assert.equal(calls().length, 3);
  assert.match(calls()[2].stdin, /break$/);
  assert.deepEqual(JSON.parse(r.stdout).map((x) => x.answer), ['claude: fine', 'claude: break']);
});

test('batch --resume of an unknown run is a usage error', async () => {
  const r = await ask(['batch', '--resume', join(tmp, 'nope')]);
  assert.equal(r.code, 2);
  assert.match(r.stderr, /no run to resume/);
});

test('runs lists recorded runs with their counts', async () => {
  const tasks = [{ prompt: 'fine' }, { prompt: 'break' }];
  await ask(['batch', '-m', 'claude:opus'], { input: JSON.stringify(tasks), extra: { FAKE_FAIL: 'break' } });
  const r = await ask(['runs']);
  assert.equal(r.stdout.trim().split('\t').slice(1).join(','), '1 ok,1 failed,0 pending');
  assert.ok(r.stdout.startsWith(join(env.ASK_HOME, 'runs')));
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

test('stopping ask kills a model that ignores SIGTERM before exiting', async () => {
  const run = start(['-m', 'codex:gpt-x', 'hang'], { extra: { FAKE_HANG: 'hang', FAKE_IGNORE_TERM: '1' } });
  await until(() => calls().length === 1);
  const { pid } = calls()[0];
  run.child.kill('SIGINT');
  const r = await run;
  assert.equal(r.code, 130);
  assert.equal(alive(pid), false);
});

test('results and status lines name the model that ran', async () => {
  writeJson(join(env.CODEX_HOME, 'models_cache.json'), { models: [{ slug: 'gpt-x', display_name: 'GPT-6.1-Sol' }] });
  const batch = JSON.stringify([
    { id: 'c', model: 'claude:opus', prompt: 'p' },
    { id: 'x', model: 'codex:gpt-x#high', prompt: 'p' },
    { id: 'o', model: 'opencode:p/deepseek-v4.1-flash', prompt: 'p' },
  ]);
  const r = await ask(['batch', '-'], { input: batch });
  const names = Object.fromEntries(JSON.parse(r.stdout).map((x) => [x.id, x.name]));
  assert.deepEqual(names, { c: 'Opus 5.5', x: 'GPT-6.1 Sol (high)', o: 'Deepseek V4.1 Flash' });
  assert.match(r.stderr, /\[c\] Opus 5\.5 ok/);
});
