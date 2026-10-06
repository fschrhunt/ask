/*
 * The Kilo agent: runs `kilo run --format json` (Kilo CLI, the Opencode fork from Kilo Code).
 * Kilo has no read-only flag and its sandbox keeps the workspace writable, so read runs inject,
 * through KILO_CONFIG_CONTENT, a primary agent with a fresh name each run (ask-read-UUID), so no
 * configured agent deep-merges into it, whose rules deny everything but the read, grep and glob
 * tools: no shell, no edits, no subagents. An agent's own rules come last and the last matching
 * rule wins; Kilo applies an agent's deny before any saved approval. It is also the default agent,
 * since Kilo runs the default when it cannot find --agent. Read runs also keep git from taking
 * optional locks or running fsmonitor (see readEnv).
 * Write runs use Kilo's code agent with --auto, which approves whatever your own rules don't deny.
 * Models are listed from `kilo models` and named like the other agents', family-version-variant in
 * lowercase, found in any provider; a provider/model id is used as it is. An effort is Kilo's
 * --variant. The session id of the first event is reported at once; a follow-up continues it with
 * --session. A new read run's prompt starts with GROUNDING. Kilo is $ASK_KILO_BIN, else on PATH,
 * else where its installers put it. Started by agents/kilo, which finds Node.js.
 */
import { execFileSync, spawn } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { accessSync, constants, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { delimiter, join } from 'node:path';

// Put before a new read run's prompt, so the answer comes from the code rather than a guess.
const GROUNDING = 'Answer from the files in your working directory: search and read them before answering. Never guess file names, functions or facts; if you cannot find something, say so.\n\n';

/* The kilo executable: $ASK_KILO_BIN if set, else the first on PATH or where Kilo's installers put it; undefined when there is none. */
function findCli() {
  if (process.env.ASK_KILO_BIN) return executable(process.env.ASK_KILO_BIN) ? process.env.ASK_KILO_BIN : undefined;
  const home = homedir();
  const dirs = [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(home, '.kilo', 'bin'), join(home, '.local', 'bin'), '/opt/homebrew/bin', '/usr/local/bin', join(home, '.npm-global', 'bin'), join(home, '.bun', 'bin')];
  return dirs.map((dir) => join(dir, 'kilo')).find(executable);
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

/*
 * The clean name of a Kilo model id, from its last segment: qwen3.8-flash -> qwen-3.8-flash,
 * deepseek-v4.1-flash -> deepseek-4.1-flash, kilo/anthropic/claude-sonnet-5.5 -> claude-sonnet-5.5.
 */
const clean = (id) => id.slice(id.lastIndexOf('/') + 1).toLowerCase().replace(/^([a-z]+)(\d)/, '$1-$2').replace(/-v(\d)/g, '-$1');

/* Every provider/model id Kilo offers, from `kilo models`. Returns { listed } or { error }. */
function listed() {
  try {
    const ids = execFileSync(CLI, ['models'], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).split('\n').map((id) => id.trim()).filter((id) => id.includes('/'));
    return ids.length ? { listed: ids } : { error: "could not list Kilo's models: it listed none" };
  } catch (error) {
    return { error: `could not list Kilo's models: ${String(error.stderr || error.message).trim().split('\n').pop()}` };
  }
}

/*
 * Kilo's provider/model id for a model name: a provider/model id as it is, else the model with that
 * clean name. When several providers have it, the first of $ASK_KILO_PROVIDER and kilo wins, then
 * the first alphabetically. Returns { id } or { error }.
 */
function kiloId(model) {
  if (model.includes('/')) return { id: model };
  const all = listed();
  if (all.error) return all;
  const provider = (id) => id.slice(0, id.indexOf('/'));
  const found = all.listed.filter((id) => clean(id) === clean(model));
  if (!found.length) return { error: `no Kilo model named ${model}; see kilo models` };
  const rank = (id) => [process.env.ASK_KILO_PROVIDER, 'kilo', provider(id)].indexOf(provider(id));
  return { id: found.sort((a, b) => rank(a) - rank(b) || a.localeCompare(b))[0] };
}

// Agent iterations before it must answer in text, as in the Opencode agent.
const STEPS = { read: 25, write: 100 };

// What a read run may do: the read, grep and glob tools, and nothing else.
const READ_ONLY = { '*': 'deny', read: 'allow', grep: 'allow', glob: 'allow' };

/*
 * The config injected through KILO_CONFIG_CONTENT: for a read run, the read-only primary agent
 * `agent`, also made the default agent; for a write run, a step cap for the code agent. Kilo
 * deep-merges agent config, so `agent` must be a name no config has: a fresh one per run.
 */
function config(write, agent) {
  if (write) return JSON.stringify({ agent: { code: { steps: STEPS.write } } });
  const readOnly = { mode: 'primary', description: 'Read-only agent for ask: reads and searches files, changes nothing.', steps: STEPS.read, permission: READ_ONLY };
  return JSON.stringify({ default_agent: agent, agent: { [agent]: readOnly } });
}

/*
 * The environment of a read run's CLI, for the git it runs on its own: no optional locks, so
 * status never rewrites the index, and core.fsmonitor off, so the repository's config cannot start
 * a command. Added after any GIT_CONFIG_* the user set.
 */
function readEnv() {
  const n = Number(process.env.GIT_CONFIG_COUNT) || 0;
  return { GIT_OPTIONAL_LOCKS: '0', GIT_CONFIG_COUNT: String(n + 1), [`GIT_CONFIG_KEY_${n}`]: 'core.fsmonitor', [`GIT_CONFIG_VALUE_${n}`]: 'false' };
}

/*
 * Runs an agent CLI in the current directory with `input` on stdin, calling onLine with each line of
 * stdout as it arrives. Resolves with { code, stdout, stderr }; never rejects. No timeout: ask stops
 * the whole process group when a run is out of time.
 */
function exec(command, args, { input = '', env, onLine } = {}) {
  return new Promise((resolve) => {
    const child = spawn(command, args, { env: { ...process.env, ...env }, stdio: ['pipe', 'pipe', 'pipe'] });
    let stdout = '';
    let stderr = '';
    let seen = 0;
    child.stdout.setEncoding('utf8');
    child.stderr.setEncoding('utf8');
    child.stdout.on('data', (d) => {
      stdout += d;
      for (let end; onLine && (end = stdout.indexOf('\n', seen)) !== -1; seen = end + 1) onLine(stdout.slice(seen, end));
    });
    child.stderr.on('data', (d) => (stderr += d));
    child.on('error', (error) => (stderr += `${error.message}\n`));
    child.stdin.on('error', () => {});
    child.on('close', (code) => resolve({ code, stdout, stderr }));
    child.stdin.end(input);
  });
}

/* The message of a Kilo error event's error: a string, or { name, data: { message } }. */
const errorText = (error) => (typeof error === 'string' ? error : error?.data?.message || error?.message || error?.name || 'error');

/*
 * The agent contract from ask's docs/agents.md: `models` prints [id, name] pairs from models();
 * otherwise run() answers the prompt on stdin, GROUNDING first for a new read run, and returns
 * { ok, text, error, name, note, usage }. run gets session (the session to continue, from
 * ASK_SESSION) and report(fields), which writes fields such as { session } or usage so far to the
 * report at once: a run stopped midway can still be continued, and ask shows usage live.
 */
async function adapter({ models, run }) {
  if (process.argv[2] === 'models') {
    try {
      for (const [id, name] of await models()) console.log(name ? `${id}\t${name}` : id);
    } catch (error) {
      console.error(error.message);
      process.exitCode = 1;
    }
    return;
  }
  const { ASK_MODEL, ASK_EFFORT, ASK_ACCESS, ASK_SCHEMA, ASK_SESSION, ASK_REPORT, ASK_TITLE } = process.env;
  const reported = {};
  const report = (fields) => {
    Object.assign(reported, fields);
    if (!ASK_REPORT) return;
    // ask reads the report while the run goes, so replace it whole: never let it see half a file.
    writeFileSync(`${ASK_REPORT}.tmp`, JSON.stringify(reported));
    renameSync(`${ASK_REPORT}.tmp`, ASK_REPORT);
  };
  const r = await run({
    prompt: (ASK_ACCESS === 'read' && !ASK_SESSION ? GROUNDING : '') + readFileSync(0, 'utf8'),
    model: ASK_MODEL,
    effort: ASK_EFFORT || undefined,
    write: ASK_ACCESS === 'write',
    schema: ASK_SCHEMA ? JSON.parse(readFileSync(ASK_SCHEMA, 'utf8')) : undefined,
    session: ASK_SESSION || undefined,
    title: ASK_TITLE || undefined,
    dir: process.cwd(),
    report,
  });
  report({ name: r.name, note: r.note || undefined, ...r.usage });
  if (r.ok) process.stdout.write(`${r.text}\n`);
  else {
    process.stderr.write(`${String(r.error).replace(/\s+/g, ' ').trim()}\n`);
    process.exitCode = 1;
  }
}

const CLI = findCli();
if (!CLI) {
  console.error('Kilo not found: install it from https://kilo.ai/cli, or set ASK_KILO_BIN to its path');
  process.exit(1);
}

await adapter({
  // Every model Kilo offers, by clean name, once each; turn off the ones you don't use with ask.
  models: () => {
    const all = listed();
    if (all.error) throw new Error(all.error);
    return [...new Set(all.listed.map(clean))].sort().map((name) => [name]);
  },

  async run({ prompt, model, effort, write, session, title, report }) {
    const { id, error } = kiloId(model);
    if (error) return { ok: false, error };
    const agent = write ? 'code' : `ask-read-${randomUUID()}`;
    const args = ['run', '--agent', agent, '-m', id, '--format', 'json'];
    if (write) args.push('--auto');
    if (effort) args.push('--variant', effort);
    if (title) args.push('--title', title);
    if (session) args.push('--session', session);
    // Events stream on stdout, each with the session id; tokens and cost come after every step.
    let sessionId;
    let text = '';
    let steps = 0;
    let failure;
    const usage = {};
    const onLine = (line) => {
      let event;
      try {
        event = JSON.parse(line);
      } catch {
        return;
      }
      if (!sessionId && event.sessionID) report({ session: (sessionId = event.sessionID) });
      if (event.type === 'error') failure ??= errorText(event.error);
      if (event.type === 'step_start') steps++;
      if (event.type === 'step_finish') {
        const { tokens: t = {}, cost } = event.part || {};
        usage.input = (usage.input || 0) + (t.input || 0);
        usage.output = (usage.output || 0) + (t.output || 0) + (t.reasoning || 0);
        usage.cached = (usage.cached || 0) + (t.cache?.read || 0);
        if (typeof cost === 'number') usage.cost = (usage.cost || 0) + cost;
        report({ ...usage });
      }
      if (event.type === 'text' && typeof event.part?.text === 'string' && !event.part.synthetic) text = event.part.text;
    };
    const r = await exec(CLI, args, { input: prompt, env: { ...(write ? {} : readEnv()), KILO_CONFIG_CONTENT: config(write, agent) }, onLine });
    if (failure) return { ok: false, error: failure, usage };
    if (r.code !== 0 || !text) {
      const reason = r.stderr.trim().split('\n').pop();
      return { ok: false, error: reason || (text ? `exit ${r.code}` : `no answer (exit ${r.code})`), usage };
    }
    const capped = steps >= STEPS[write ? 'write' : 'read'];
    return { ok: true, text, note: capped ? 'hit step cap; answer may be partial' : '', usage };
  },
});
