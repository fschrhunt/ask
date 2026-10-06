/*
 * The Kimi Code agent: runs `kimi --prompt=TEXT --output-format stream-json` (print mode, which
 * approves tool calls on its own). Models are the aliases of `kimi provider list --json`, each an
 * exact provider and model id; the alias is passed to -m, and KIMI_MODEL_* variables, which could
 * define another model, are removed from its environment. Read runs start a session with the
 * bundled lib/read-agent.md, whose tool list is Read, Grep and Glob, which Kimi enforces before
 * running a tool, so there is no shell, no editor and no sub-agent. Kimi stores the agent's
 * rendered prompt and tool list in the session and cannot change them on resume, so a read
 * session's id is reported as ask-read:ID and only read runs may continue it (with no agent flag);
 * a write session cannot be continued read-only. Kimi prints the session id
 * when the run ends, so only a finished run can be continued. Started by agents/kimi;
 * ASK_KIMI_BIN overrides executable discovery.
 */
import { execFileSync, spawn } from 'node:child_process';
import { accessSync, constants, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { delimiter, join } from 'node:path';
import { fileURLToPath } from 'node:url';

// Put before a new read run's prompt, so the answer comes from the code rather than a guess.
const GROUNDING = 'Answer from the files in your working directory: search and read them before answering. Never guess file names, functions or facts; if you cannot find something, say so.\n\n';

// Kimi takes the prompt as one argument, which Linux caps at 128 KiB.
const MAX_PROMPT_BYTES = 100000;

// The read agent: Kimi's default system prompt and only the tools that read.
const READ_AGENT = fileURLToPath(new URL('read-agent.md', import.meta.url));

/* The kimi executable: $ASK_KIMI_BIN if set, else the first on PATH or in the usual places; undefined when there is none. */
function findCli() {
  if (process.env.ASK_KIMI_BIN) return executable(process.env.ASK_KIMI_BIN) ? process.env.ASK_KIMI_BIN : undefined;
  const dirs = [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(homedir(), '.local', 'bin'), '/opt/homebrew/bin', '/usr/local/bin'];
  return dirs.map((dir) => join(dir, 'kimi')).find(executable);
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

/* Kimi's environment without KIMI_MODEL_*, which can define a model that -m would not name. */
function cleanEnv() {
  return Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('KIMI_MODEL_')));
}

/*
 * The configured model aliases as [{ alias, id, name }]: id is the model's provider-side id in
 * lowercase, or the alias when several aliases share it (they may route to different providers).
 * Throws when kimi cannot list them or none is configured.
 */
function listed() {
  let models;
  try {
    models = JSON.parse(execFileSync(CLI, ['provider', 'list', '--json'], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], env: cleanEnv() })).models || {};
  } catch (error) {
    throw new Error(`could not list Kimi's models: ${String(error.stderr || error.message).trim().split('\n').pop()}`);
  }
  const all = Object.entries(models).map(([alias, m]) => ({ alias, model: String(m.model || alias), name: m.display_name }));
  if (!all.length) throw new Error('Kimi has no models configured; run kimi and /login first');
  return all.map(({ alias, model, name }) => ({ alias, model, id: (all.filter((o) => o.model.toLowerCase() === model.toLowerCase()).length > 1 ? alias : model).toLowerCase(), name: name || model }));
}

/* The model whose id or alias is `name`, without regard to case: { model } or { error }. A model id several aliases share is refused. */
function resolve(name) {
  const want = name.toLowerCase();
  const all = listed();
  const found = all.filter((m) => m.id === want || m.alias.toLowerCase() === want);
  if (found.length === 1) return { model: found[0] };
  const shared = all.filter((m) => m.model.toLowerCase() === want);
  if (shared.length > 1) return { error: `${name} fits several Kimi models; use one of their aliases: ${shared.map((m) => m.alias).join(', ')}` };
  return { error: `no Kimi model named ${name}; see ask models kimi` };
}

/* Runs kimi in the current directory, calling onLine with each line of stdout; resolves with { code, stderr }, never rejects. */
function exec(args, onLine) {
  return new Promise((resolveRun) => {
    const child = spawn(CLI, args, { env: cleanEnv(), stdio: ['ignore', 'pipe', 'pipe'] });
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
    child.on('close', (code) => {
      if (buffer) onLine(buffer);
      resolveRun({ code, stderr });
    });
  });
}

/* The agent contract (docs/agents.md): `kimi models` lists models; otherwise answer the prompt on stdin. */
async function main() {
  if (process.argv[2] === 'models') {
    for (const { id, name } of listed()) console.log(`${id}\t${name}`);
    return;
  }
  const { ASK_MODEL, ASK_EFFORT, ASK_ACCESS, ASK_SESSION, ASK_REPORT } = process.env;
  const fail = (message) => {
    process.stderr.write(`${String(message).replace(/\s+/g, ' ').trim()}\n`);
    process.exitCode = 1;
  };
  const report = (fields) => {
    if (!ASK_REPORT) return;
    writeFileSync(`${ASK_REPORT}.tmp`, JSON.stringify(fields));
    renameSync(`${ASK_REPORT}.tmp`, ASK_REPORT);
  };
  const read = ASK_ACCESS === 'read';
  if (ASK_EFFORT) return fail('Kimi Code has no reasoning-effort option for -p; run without #effort');
  const resumed = ASK_SESSION?.startsWith('ask-read:');
  if (ASK_SESSION && !ASK_SESSION.replace(/^ask-read:/, '')) return fail('empty session id');
  if (ASK_SESSION && read !== !!resumed) {
    return fail(read ? 'that session was started with write access, and Kimi cannot make it read-only; start a new read run' : 'that session was started read-only, and Kimi cannot make it writable; start a new write run with ask -w');
  }
  const { model, error } = resolve(ASK_MODEL);
  if (error) return fail(error);
  const prompt = (read && !ASK_SESSION ? GROUNDING : '') + readFileSync(0, 'utf8');
  if (Buffer.byteLength(prompt) > MAX_PROMPT_BYTES) return fail('prompt too large: kimi -p takes it as one argument, up to 100 KB');

  const args = ['-m', model.alias, `--prompt=${prompt}`, '--output-format', 'stream-json'];
  if (ASK_SESSION) args.push('--session', ASK_SESSION.replace(/^ask-read:/, ''));
  if (read && !ASK_SESSION) args.push('--agent-file', READ_AGENT);
  let answer;
  let session;
  const r = await exec(args, (line) => {
    let event;
    try {
      event = JSON.parse(line);
    } catch {
      return;
    }
    if (event.role === 'assistant' && event.content && !event.tool_calls?.length) answer = event.content;
    if (event.role === 'meta' && event.type === 'session.resume_hint') session = event.session_id;
  });
  if (session) report({ session: read ? `ask-read:${session}` : session, name: model.name });
  if (r.code === 0 && answer) process.stdout.write(`${answer}\n`);
  else fail(r.stderr.trim().split('\n').pop() || `no answer (exit ${r.code})`);
}

const CLI = findCli();
if (!CLI) {
  console.error('Kimi Code not found: install it from https://github.com/MoonshotAI/kimi-code, or set ASK_KIMI_BIN to its path');
  process.exit(1);
}
try {
  await main();
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}
