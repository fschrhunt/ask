/*
 * Tests for the opencode agent. Each runs agents/opencode as ask does (prompt on stdin, ASK_* env,
 * answer on stdout, report in $ASK_REPORT), driving the fake opencode in ./bin. No network, no real
 * models. Run: node --test
 */
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { after as cleanup, afterEach, beforeEach, test } from 'node:test';
import { fileURLToPath } from 'node:url';

const HERE = dirname(fileURLToPath(import.meta.url));
const AGENT = join(HERE, '..', 'agents', 'opencode');
const FAKE = join(HERE, 'bin', 'opencode');
// A PATH directory holding only node, so no real CLI next to it can be found.
const NODE_DIR = mkdtempSync(join(tmpdir(), 'agent-node-'));
symlinkSync(process.execPath, join(NODE_DIR, 'node'));
const PATH = `${join(HERE, 'bin')}:${NODE_DIR}`;
let tmp;

beforeEach(() => {
  tmp = realpathSync(mkdtempSync(join(tmpdir(), 'agent-test-')));
});
afterEach(() => rmSync(tmp, { recursive: true, force: true }));
cleanup(() => rmSync(NODE_DIR, { recursive: true, force: true }));

/* Runs the agent with args and env; resolves with { code, stdout, stderr }. */
function spawnAgent(args, env, { input = '', cwd = tmp } = {}) {
  return new Promise((resolve) => {
    const child = spawn(AGENT, args, { cwd, env });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (d) => (stdout += d));
    child.stderr.on('data', (d) => (stderr += d));
    child.on('close', (code) => resolve({ code, stdout, stderr }));
    child.stdin.end(input);
  });
}

/*
 * Runs the agent on one prompt. Resolves with { code, stdout, stderr, report, calls }, where calls
 * are the fake CLI's recorded invocations.
 */
async function run(prompt, { model = 'm', effort = '', access = 'read', schema, extra = {} } = {}) {
  const env = { PATH, HOME: tmp, FAKE_LOG: join(tmp, 'calls.jsonl'), ASK_MODEL: model, ASK_EFFORT: effort, ASK_ACCESS: access, ASK_REPORT: join(tmp, 'report.json'), ...extra };
  if (schema) writeFileSync((env.ASK_SCHEMA = join(tmp, 'schema.json')), JSON.stringify(schema));
  const r = await spawnAgent([], env, { input: prompt });
  let report = null;
  let calls = [];
  try {
    report = JSON.parse(readFileSync(env.ASK_REPORT, 'utf8'));
  } catch {}
  try {
    calls = readFileSync(env.FAKE_LOG, 'utf8').trim().split('\n').filter(Boolean).map((line) => JSON.parse(line));
  } catch {}
  return { ...r, report, calls };
}

/* Runs `opencode models` and resolves with its lines. */
async function models(extra = {}) {
  const r = await spawnAgent(['models'], { PATH, HOME: tmp, ...extra });
  return r.stdout.trim().split('\n').filter(Boolean);
}

const after = (argv, flag) => argv[argv.indexOf(flag) + 1];
const GROUNDING = 'Answer from the files in your working directory: search and read them before answering. Never guess file names, functions or facts; if you cannot find something, say so.\n\n';

test('lists no models', async () => {
  assert.deepEqual(await models(), []);
});

test('read runs use the plan agent with injected read-only permissions', async () => {
  const [call] = (await run('look')).calls;
  assert.equal(after(call.argv, '--agent'), 'plan');
  const { steps, permissions } = call.config.agents.plan;
  assert.equal(steps, 25);
  assert.deepEqual(permissions[0], { action: '*', resource: '*', effect: 'deny' });
  assert.ok(permissions.some((p) => p.action === 'shell' && p.resource === 'git diff *' && p.effect === 'allow'));
});

test('read runs refuse writing options, quoting and escaping, and git grep', async () => {
  const [call] = (await run('look')).calls;
  const denied = call.config.agents.plan.permissions.filter((p) => p.effect === 'deny').map((p) => p.resource);
  for (const token of ['>', '|', ';', '&', '`', '$', '\\', '--output', '--ext-diff', '--textconv', '--pre', '--hostname-bin', "'", '"', '{']) assert.ok(denied.includes(`*${token}*`), token);
  assert.ok(!call.config.agents.plan.permissions.some((p) => p.resource.startsWith('git grep')));
});

test('write runs use the build agent with the larger step cap', async () => {
  const [call] = (await run('change', { access: 'write' })).calls;
  assert.equal(after(call.argv, '--agent'), 'build');
  assert.deepEqual(call.config, { agents: { build: { steps: 100 } } });
});

test('a new read run is told to answer from the files; write runs and follow-ups get the prompt as is', async () => {
  const read = (await run('where is main?')).calls[0];
  const write = (await run('fix it', { access: 'write' })).calls[1];
  const followUp = (await run('and then?', { extra: { ASK_SESSION: 'ses_fake' } })).calls[2];
  assert.equal(read.stdin, `${GROUNDING}where is main?`);
  assert.equal(write.stdin, 'fix it');
  assert.equal(followUp.stdin, 'and then?');
});

test('passes an effort as the model variant', async () => {
  const [call] = (await run('hi', { model: 'p/m', effort: 'max' })).calls;
  assert.equal(after(call.argv, '-m'), 'p/m#max');
});

test('answers on stdout and reports usage', async () => {
  const r = await run('hi');
  assert.equal(r.code, 0);
  assert.equal(r.stdout, 'opencode: hi\n');
  for (const [key, value] of Object.entries({ input: 50, output: 7, cached: 10, cost: 0.002 })) assert.equal(r.report[key], value, key);
});

test('counts the usage of the answering step that Opencode prints no step_finish for', async () => {
  const r = await run('hi', { extra: { FAKE_STEPS: '2', FAKE_UNFINISHED: '1' } });
  assert.equal(r.code, 0);
  for (const [key, value] of Object.entries({ input: 100, output: 14, cached: 20, cost: 0.004 })) assert.equal(r.report[key], value, key);
});

test('counts the answering step when Opencode prints only its text', async () => {
  const r = await run('hi', { extra: { FAKE_UNFINISHED: 'text' } });
  assert.equal(r.code, 0);
  for (const [key, value] of Object.entries({ input: 50, output: 7, cached: 10, cost: 0.002 })) assert.equal(r.report[key], value, key);
});

test('waits for Opencode to save the answering step before counting it', async () => {
  const r = await run('hi', { extra: { FAKE_STEPS: '2', FAKE_UNFINISHED: '1', FAKE_SAVED_LATE: join(tmp, 'saved') } });
  assert.equal(r.code, 0);
  for (const [key, value] of Object.entries({ input: 100, output: 14, cached: 20, cost: 0.004 })) assert.equal(r.report[key], value, key);
});

test('reports usage while Opencode is still running', async () => {
  let done = false;
  const finished = run('hi', { extra: { FAKE_PAUSE: '1500' } }).then(() => (done = true));
  let live;
  while (!done && !live?.input) {
    await new Promise((resolve) => setTimeout(resolve, 50));
    try {
      live = JSON.parse(readFileSync(join(tmp, 'report.json'), 'utf8'));
    } catch {}
  }
  assert.equal(done, false);
  assert.ok(live.input > 0);
  await finished;
});

test('a run that reports no usage reports none', async () => {
  const r = await run('hi', { extra: { FAKE_NO_USAGE: '1' } });
  assert.equal(r.code, 0);
  assert.ok(!r.report.input && !r.report.output);
});

test('a run that hits the step cap notes its answer may be partial', async () => {
  const r = await run('look', { extra: { FAKE_STEPS: '25' } });
  assert.equal(r.code, 0);
  assert.match(r.report.note, /hit step cap/);
});

test('fails a run whose Opencode exited nonzero, with the reason last on stderr', async () => {
  const r = await run('hi', { extra: { FAKE_EXIT: '1' } });
  assert.equal(r.code, 1);
  assert.match(r.stderr.trim().split('\n').pop(), /exit 1/);
});

test("a failing Opencode's own message is the reason", async () => {
  const r = await run('break', { extra: { FAKE_FAIL: 'break' } });
  assert.equal(r.code, 1);
  assert.equal(r.stderr.trim(), 'opencode boom');
});

test('reports its session, and a follow-up continues it', async () => {
  const first = await run('hi');
  assert.equal(first.report.session, 'ses_fake');
  const second = await run('more', { extra: { ASK_SESSION: 'ses_fake' } });
  assert.equal(after(second.calls[1].argv, '--session'), 'ses_fake');
});

test('runs a clean model name as its listed id, in any case, and a provider/model id as it is', async () => {
  const qwen = (await run('hi', { model: 'Qwen-3.8-Flash' })).calls[0];
  const other = (await run('hi', { model: 'other/deepseek-v4.1-flash' })).calls[1];
  assert.equal(after(qwen.argv, '-m'), 'opencode-go/qwen3.8-flash');
  assert.equal(after(other.argv, '-m'), 'other/deepseek-v4.1-flash');
});

test('prefers ASK_OPENCODE_PROVIDER, then opencode-go, then opencode, then the first alphabetically', async () => {
  const chosen = (await run('hi', { model: 'deepseek-4.1-flash', extra: { ASK_OPENCODE_PROVIDER: 'other' } })).calls[0];
  const go = (await run('hi', { model: 'deepseek-4.1-flash' })).calls[1];
  const zen = (await run('hi', { model: 'glm-5' })).calls[2];
  const first = (await run('hi', { model: 'kimi-k3' })).calls[3];
  assert.equal(after(chosen.argv, '-m'), 'other/deepseek-v4.1-flash');
  assert.equal(after(go.argv, '-m'), 'opencode-go/deepseek-v4.1-flash');
  assert.equal(after(zen.argv, '-m'), 'opencode/glm-5');
  assert.equal(after(first.argv, '-m'), 'beta/kimi-k3');
});

test('refuses a model name it cannot find', async () => {
  const r = await run('hi', { model: 'nope-1.0' });
  assert.equal(r.code, 1);
  assert.equal(r.calls.length, 0);
  assert.equal(r.stderr, 'no Opencode model named nope-1.0; see opencode models\n');
});

test('finds Opencode off PATH through ASK_OPENCODE_BIN', async () => {
  const r = await run('hi', { extra: { PATH: NODE_DIR, ASK_OPENCODE_BIN: FAKE } });
  assert.equal(r.code, 0);
  assert.equal(r.calls.length, 1);
});

test('finds Opencode off PATH where its installer puts it', async () => {
  mkdirSync(join(tmp, '.opencode', 'bin'), { recursive: true });
  symlinkSync(FAKE, join(tmp, '.opencode', 'bin', 'opencode'));
  const r = await run('hi', { extra: { PATH: NODE_DIR } });
  assert.equal(r.code, 0);
  assert.equal(r.calls.length, 1);
});

test('without Opencode, models and runs fail saying how to install it', async () => {
  const extra = { ASK_OPENCODE_BIN: join(tmp, 'missing') };
  const hint = /^Opencode not found: install it from https:\/\/opencode\.ai, or set ASK_OPENCODE_BIN to its path\n$/;
  const listed = await spawnAgent(['models'], { PATH, HOME: tmp, ...extra });
  const ran = await run('hi', { extra });
  assert.equal(listed.code, 1);
  assert.match(listed.stderr, hint);
  assert.equal(ran.code, 1);
  assert.match(ran.stderr, hint);
});

// Node at a fixed location, as on CI runners, would be found whatever PATH says.
const fixedNode = ['/opt/homebrew/bin/node', '/usr/local/bin/node', '/home/linuxbrew/.linuxbrew/bin/node'].some(existsSync);
test('the launcher says how to get Node.js when it finds none', { skip: fixedNode && 'Node.js is installed at a fixed location' }, async () => {
  const empty = join(tmp, 'empty');
  mkdirSync(empty);
  const r = await spawnAgent(['models'], { PATH: empty, HOME: empty });
  assert.equal(r.code, 1);
  assert.equal(r.stderr, 'opencode needs Node.js 18 or newer: install it from https://nodejs.org (or set ASK_NODE)\n');
});
