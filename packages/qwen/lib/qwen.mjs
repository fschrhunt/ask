/*
 * The Qwen Code agent: runs `qwen` headless (--output-format stream-json, prompt on stdin).
 * Models are exactly the ids in the user's modelProviders setting; the id is passed to --model as
 * written, and a run fails if Qwen reports any other model answering (a settings fallback).
 * Write runs only (yolo): Qwen Code has no read-only restriction that settings, workspace
 * settings or extensions cannot widen, so read runs are refused before qwen starts.
 * The session comes from the first event for follow-ups (--resume); usage is the result's.
 * Started by agents/qwen; ASK_QWEN_BIN overrides executable discovery.
 */
import { spawn } from 'node:child_process';
import { accessSync, constants, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { delimiter, join } from 'node:path';

/* The qwen executable: $ASK_QWEN_BIN if set, else the first on PATH or in the usual places; undefined when there is none. */
function findCli() {
  if (process.env.ASK_QWEN_BIN) return executable(process.env.ASK_QWEN_BIN) ? process.env.ASK_QWEN_BIN : undefined;
  const dirs = [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(homedir(), '.local', 'bin'), '/opt/homebrew/bin', '/usr/local/bin'];
  return dirs.map((dir) => join(dir, 'qwen')).find(executable);
}

/* Whether path is an executable file. */
function executable(path) {
  try {
    accessSync(path, constants.X_OK);
    return statSync(path).isFile();
  } catch {
    return false;
  }
}

/* The clean name of a model id: qwen3.8-flash -> qwen-3.8-flash. */
const clean = (id) => id.toLowerCase().replace(/^([a-z]+)(\d)/, '$1-$2').replace(/-v(\d)/g, '-$1');

/*
 * The models in $QWEN_HOME/settings.json (default ~/.qwen) modelProviders, as [{ id, name }] with
 * each id once. Qwen reads settings as JSON with comments, so those are dropped first. Throws when
 * the file is unreadable or lists nothing: Qwen has no command that lists models.
 */
function listed() {
  const path = join(process.env.QWEN_HOME || join(homedir(), '.qwen'), 'settings.json');
  let providers;
  try {
    const text = readFileSync(path, 'utf8').replace(/"(?:\\.|[^"\\])*"|\/\/[^\n]*|\/\*[\s\S]*?\*\//g, (m) => (m[0] === '"' ? m : ''));
    providers = JSON.parse(text).modelProviders;
  } catch {
    throw new Error(`could not read modelProviders from ${path}; check the Qwen Code settings file`);
  }
  const found = Object.values(providers || {}).flat().filter((m) => typeof m?.id === 'string');
  if (!found.length) throw new Error(`no models in modelProviders of ${path}; add them to Qwen Code first`);
  const byId = new Map();
  for (const m of found) if (!byId.has(m.id)) byId.set(m.id, { id: m.id, name: m.name });
  return [...byId.values()];
}

/*
 * The exact modelProviders entry for a model name: its id as written, or its clean name, without
 * regard to case. Returns { model } or { error }; a name matching several ids is refused.
 */
function resolve(name) {
  const want = name.toLowerCase();
  const found = listed().filter((m) => m.id.toLowerCase() === want || clean(m.id) === clean(name));
  if (found.length === 1) return { model: found[0] };
  return { error: found.length ? `${name} matches ${found.map((m) => m.id).join(', ')}; use one of those ids` : `no Qwen model named ${name} in modelProviders; see ask models qwen` };
}

/*
 * Runs `qwen` in the current directory with `input` on stdin, calling onLine with each line of
 * stdout as it arrives. Resolves with { code, stderr }; never rejects. No timeout: ask stops the
 * whole process group when a run is out of time.
 */
function exec(args, input, onLine) {
  return new Promise((resolveRun) => {
    const child = spawn(CLI, args, { stdio: ['pipe', 'pipe', 'pipe'] });
    let buffer = '';
    let stderr = '';
    child.stdout.setEncoding('utf8');
    child.stderr.setEncoding('utf8');
    child.stdout.on('data', (d) => {
      buffer += d;
      for (let end; (end = buffer.indexOf('\n')) !== -1; buffer = buffer.slice(end + 1)) onLine(buffer.slice(0, end));
    });
    child.stderr.on('data', (d) => (stderr += d));
    child.on('error', (error) => (stderr += `${error.message}\n`));
    child.stdin.on('error', () => {});
    child.on('close', (code) => {
      if (buffer) onLine(buffer);
      resolveRun({ code, stderr });
    });
    child.stdin.end(input);
  });
}

/* Writes the report so far; ask reads it while the run goes, so replace it whole. */
function reporter() {
  const fields = {};
  return (update) => {
    Object.assign(fields, update);
    if (!process.env.ASK_REPORT) return;
    writeFileSync(`${process.env.ASK_REPORT}.tmp`, JSON.stringify(fields));
    renameSync(`${process.env.ASK_REPORT}.tmp`, process.env.ASK_REPORT);
  };
}

/* Qwen's usage object as ask's report fields. */
const tokens = (u = {}) => ({ input: u.input_tokens || 0, output: u.output_tokens || 0, cached: u.cache_read_input_tokens || 0 });

/* The agent contract (docs/agents.md): `qwen models` lists models; otherwise answer the prompt on stdin. */
async function main() {
  if (process.argv[2] === 'models') {
    // A clean name that fits several ids would be refused by resolve, so those are listed by id.
    const all = listed();
    for (const { id, name } of all) console.log(`${all.filter((m) => clean(m.id) === clean(id)).length > 1 ? id.toLowerCase() : clean(id)}\t${name || id}`);
    return;
  }
  const { ASK_MODEL, ASK_EFFORT, ASK_ACCESS, ASK_SESSION } = process.env;
  const report = reporter();
  const fail = (message) => {
    process.stderr.write(`${String(message).replace(/\s+/g, ' ').trim()}\n`);
    process.exitCode = 1;
  };
  if (ASK_ACCESS !== 'write') return fail('Qwen Code cannot be restricted to reading: its tool list is merged with settings and extensions. Use ask -w, or another agent for read-only runs');
  if (ASK_EFFORT) return fail('Qwen Code has no reasoning-effort option; run without #effort');
  const { model, error } = resolve(ASK_MODEL);
  if (error) return fail(error);

  const args = ['--output-format', 'stream-json', '--model', model.id, '--approval-mode', 'yolo'];
  if (ASK_SESSION) args.push('--resume', ASK_SESSION);
  const prompt = readFileSync(0, 'utf8');

  let session;
  let result;
  let wrong;
  let live = { input: 0, output: 0, cached: 0 };
  const r = await exec(args, prompt, (line) => {
    let event;
    try {
      event = JSON.parse(line);
    } catch {
      return;
    }
    if (!session && event.session_id) report({ session: (session = event.session_id) });
    if (event.type === 'assistant') {
      const used = event.message?.model;
      if (used && used.toLowerCase() !== model.id.toLowerCase()) wrong ??= used;
      const u = tokens(event.message?.usage);
      live = { input: live.input + u.input, output: live.output + u.output, cached: live.cached + u.cached };
      report({ name: model.name || model.id, ...live });
    }
    if (event.type === 'result') result = event;
  });
  if (result?.usage) report(tokens(result.usage));
  if (wrong) return fail(`Qwen Code answered with ${wrong}, not ${model.id} (a fallback model in its settings?)`);
  if (result && !result.is_error && typeof result.result === 'string' && r.code === 0) {
    report({ name: model.name || model.id });
    process.stdout.write(`${result.result}\n`);
    return;
  }
  fail(result?.error?.message || r.stderr.trim().split('\n').pop() || `no answer (exit ${r.code})`);
}

const CLI = findCli();
if (!CLI) {
  console.error('Qwen Code not found: install it from https://github.com/QwenLM/qwen-code, or set ASK_QWEN_BIN to its path');
  process.exit(1);
}
try {
  await main();
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}
