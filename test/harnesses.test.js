/* The shipped harnesses: how each drives its CLI, read-only rules, schemas, usage and model names. */
import assert from 'node:assert/strict';
import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';
import { tmp, env, ask, calls, writeJson, after } from './helpers.js';

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

test('an effort suffix reaches each harness that takes one', async () => {
  await ask(['-m', 'claude:opus#high', 'hi']);
  await ask(['-m', 'codex:gpt-x#low', 'hi']);
  const [claude, codex] = calls();
  assert.equal(after(claude.argv, '--effort'), 'high');
  assert.ok(codex.argv.includes('model_reasoning_effort="low"'));
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

test('a run that reports no usage prints no usage', async () => {
  const r = await ask(['-m', 'opencode:p/m', 'hi'], { extra: { FAKE_NO_USAGE: '1' } });
  assert.equal(r.code, 0);
  assert.doesNotMatch(r.stderr, / in .* out/);
});

for (const model of ['claude:opus', 'codex:gpt-x', 'opencode:p/m']) {
  test(`a ${model.split(':')[0]} answer from a run that exited nonzero fails`, async () => {
    const r = await ask(['-m', model, 'hi'], { extra: { FAKE_EXIT: '1' } });
    assert.equal(r.code, 1);
    assert.match(r.stderr, /failed after .*exit 1/);
  });
}
