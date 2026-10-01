/*
 * Shared setup for the tests: each test runs the real `bin/ask` with ASK_HOME and HOME inside a
 * fresh temp dir, and test/fake installed as the harness `fake`. No network, no real models. The
 * fake records each run in $FAKE_LOG; see it for the env vars that make it fail or hang. `tmp` and
 * `env` are the current test's.
 */
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, realpathSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { afterEach, beforeEach } from 'node:test';
import { fileURLToPath } from 'node:url';

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..');
const PATH = dirname(process.execPath);
export let tmp;
export let env;

beforeEach(() => {
  tmp = realpathSync(mkdtempSync(join(tmpdir(), 'ask-test-')));
  env = { PATH, HOME: tmp, ASK_HOME: join(tmp, 'home'), FAKE_LOG: join(tmp, 'calls.jsonl') };
  mkdirSync(join(env.ASK_HOME, 'harnesses'), { recursive: true });
  symlinkSync(join(ROOT, 'test/fake'), join(env.ASK_HOME, 'harnesses', 'fake'));
});
afterEach(() => rmSync(tmp, { recursive: true, force: true }));

/* Starts ask; resolves with { code, stdout, stderr } and exposes the child as `child`. */
export function start(args, { input = '', extra = {} } = {}) {
  const child = spawn(process.execPath, [join(ROOT, 'bin/ask'), ...args], { cwd: tmp, env: { ...env, ...extra } });
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
export const ask = (args, options) => start(args, options);

export const calls = () => {
  try {
    return readFileSync(env.FAKE_LOG, 'utf8').trim().split('\n').filter(Boolean).map((line) => JSON.parse(line));
  } catch {
    return [];
  }
};
export const alive = (pid) => {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
};
export const until = async (condition) => {
  for (let i = 0; i < 100 && !condition(); i++) await new Promise((resolve) => setTimeout(resolve, 50));
  assert.ok(condition(), 'timed out waiting');
};
export const writeJson = (path, value) => {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, JSON.stringify(value));
};
export const runDirs = () => readdirSync(join(env.ASK_HOME, 'runs'));
export const runFile = (name) => JSON.parse(readFileSync(join(env.ASK_HOME, 'runs', runDirs()[0], name), 'utf8'));
export const after = (argv, flag) => argv[argv.indexOf(flag) + 1];
