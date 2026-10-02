/*
 * The Opencode agent: runs `opencode run --standalone`, so stopping the run also ends its session.
 * Opencode has no read-only flag, and a global allow-everything rule overrides its built-in plan
 * agent, so read runs inject rules for plan: file tools plus the read-only inspection commands.
 * Injected config is applied last and the last matching rule wins. Write runs use build with the
 * user's own permissions. It lists every model of the providers you have set up in Opencode;
 * turn off the ones you don't use with ask models or ask setup opencode. Models are named like the other agents',
 * family-version-variant in lowercase (deepseek-4.1-flash for opencode-go/deepseek-v4.1-flash),
 * found in any provider; a provider/model id is used as it is. The session id of the first event is
 * reported at once; a follow-up continues it with --session. A new read run's prompt starts with
 * GROUNDING. Opencode is $ASK_OPENCODE_BIN, else on PATH, else where its installers put it.
 * Started by agents/opencode, which finds Node.js.
 */
import { execFileSync, spawn } from 'node:child_process';
import { accessSync, constants, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { delimiter, join } from 'node:path';

// Put before a new read run's prompt, so the answer comes from the code rather than a guess.
const GROUNDING = 'Answer from the files in your working directory: search and read them before answering. Never guess file names, functions or facts; if you cannot find something, say so.\n\n';

/* The opencode executable: $ASK_OPENCODE_BIN if set, else the first on PATH or where Opencode's installers put it; undefined when there is none. */
function findCli() {
  if (process.env.ASK_OPENCODE_BIN) return executable(process.env.ASK_OPENCODE_BIN) ? process.env.ASK_OPENCODE_BIN : undefined;
  const home = homedir();
  const dirs = [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(home, '.opencode', 'bin'), join(home, '.local', 'bin'), '/opt/homebrew/bin', '/usr/local/bin', join(home, '.bun', 'bin')];
  return dirs.map((dir) => join(dir, 'opencode')).find(executable);
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

/* The clean name of an Opencode model id: qwen3.8-flash -> qwen-3.8-flash, deepseek-v4.1-flash -> deepseek-4.1-flash. */
const clean = (id) => id.toLowerCase().replace(/^([a-z]+)(\d)/, '$1-$2').replace(/-v(\d)/g, '-$1');

/*
 * Every provider/model id Opencode offers, which is every model of the providers you have set up
 * in Opencode. Returns { listed } or { error }; the list is asked for twice before giving up, since
 * Opencode can briefly list nothing while it refreshes its catalog.
 */
function listed() {
  let ids = [];
  let reason = 'it listed none';
  for (let attempt = 0; attempt < 2 && !ids.length; attempt++) {
    try {
      ids = execFileSync(CLI, ['models'], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).split('\n').filter((id) => id.includes('/'));
    } catch (error) {
      reason = String(error.stderr || error.message).trim().split('\n').pop();
    }
  }
  return ids.length ? { listed: ids } : { error: `could not list Opencode's models: ${reason}` };
}

/*
 * Opencode's provider/model id for a model name: a provider/model id as it is, else the model with
 * that clean name. When several providers have it, the first of $ASK_OPENCODE_PROVIDER,
 * opencode-go and opencode wins, then the first alphabetically. Returns { id } or { error }.
 */
function opencodeId(model) {
  if (model.includes('/')) return { id: model };
  const all = listed();
  if (all.error) return all;
  const provider = (id) => id.slice(0, id.indexOf('/'));
  const found = all.listed.filter((id) => clean(id.slice(id.indexOf('/') + 1)) === clean(model));
  if (!found.length) return { error: `no Opencode model named ${model}; see opencode models` };
  const rank = (id) => [process.env.ASK_OPENCODE_PROVIDER, 'opencode-go', 'opencode', provider(id)].indexOf(provider(id));
  return { id: found.sort((a, b) => rank(a) - rank(b) || a.localeCompare(b))[0] };
}

/*
 * The messages of a session, from `opencode session export --standalone`: one assistant message per
 * step, with that step's tokens and cost. Returns [] when Opencode cannot export it.
 */
function exported(session) {
  try {
    return JSON.parse(execFileSync(CLI, ['session', 'export', '--standalone', session], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'], maxBuffer: 1 << 28 })).messages || [];
  } catch {
    return [];
  }
}

// Agent iterations before it must answer in text: lookups measured 1-13, a runaway 107.
const STEPS = { read: 25, write: 100 };

/* The agent config injected through OPENCODE_CONFIG_CONTENT: plan with read-only rules, or build. */
function config(write) {
  const rule = (action, resource, effect) => ({ action, resource, effect });
  const readOnly = [
    rule('*', '*', 'deny'),
    ...['read', 'grep', 'glob'].map((action) => rule(action, '*', 'allow')),
    ...INSPECT.flatMap((command) => [rule('shell', command, 'allow'), rule('shell', `${command} *`, 'allow')]),
    ...[...UNSAFE, ...WRITERS].map((token) => rule('shell', `*${token}*`, 'deny')),
  ];
  return JSON.stringify({
    agents: write ? { build: { steps: STEPS.write } } : { plan: { steps: STEPS.read, permissions: readOnly } },
  });
}

/*
 * Read runs may run only INSPECT commands, written as plain words: a command containing an
 * UNSAFE character (one that redirects, pipes, chains, substitutes, expands, quotes or escapes, any
 * of which could also spell a refused option past a rule) or a WRITER option (one that writes files
 * or runs another program) is refused. Rules match text, so commands that take abbreviated or
 * bundled options, like git grep, are left out; the agent has its own grep tool. Tests and builds
 * write files, so they need -w.
 */
const INSPECT = ['git diff', 'git log', 'git show', 'git status', 'git blame', 'git ls-files', 'rg', 'grep', 'ls', 'wc', 'cat', 'head', 'tail'];
const UNSAFE = ['>', '|', ';', '&', '`', '$', '\\', "'", '"', '{'];
const WRITERS = ['--output', '--ext-diff', '--textconv', '--pre', '--hostname-bin'];

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
  const { ASK_MODEL, ASK_EFFORT, ASK_ACCESS, ASK_SCHEMA, ASK_SESSION, ASK_REPORT } = process.env;
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
  console.error('Opencode not found: install it from https://opencode.ai, or set ASK_OPENCODE_BIN to its path');
  process.exit(1);
}

await adapter({
  // Every model Opencode offers, by clean name, once each; turn off the ones you don't use with ask.
  models: () => {
    const all = listed();
    if (all.error) throw new Error(all.error);
    return [...new Set(all.listed.map((id) => clean(id.slice(id.indexOf('/') + 1))))].sort().map((name) => [name]);
  },

  async run({ prompt, model, effort, write, session, report }) {
    const { id, error } = opencodeId(model);
    if (error) return { ok: false, error };
    const args = ['run', '--standalone', '--agent', write ? 'build' : 'plan', '-m', effort ? `${id}#${effort}` : id, '--format', 'json'];
    if (session) args.push('--session', session);
    // Events stream on stdout: the session on the first, and tokens and cost after every step.
    let sessionId;
    let text = '';
    let steps = 0;
    let failure;
    const usage = {};
    const unfinished = new Set();
    const finished = new Set();
    const add = ({ tokens: t = {}, cost }) => {
      usage.input = (usage.input || 0) + (t.input || 0);
      usage.output = (usage.output || 0) + (t.output || 0) + (t.reasoning || 0);
      usage.cached = (usage.cached || 0) + (t.cache?.read || 0);
      if (typeof cost === 'number') usage.cost = (usage.cost || 0) + cost;
    };
    const onLine = (line) => {
      const id = !sessionId && /"sessionID":"([^"]+)"/.exec(line)?.[1];
      if (id) report({ session: (sessionId = id) });
      let event;
      try {
        event = JSON.parse(line);
      } catch {
        return;
      }
      if (event.type === 'error') failure ??= event.error?.message || 'error';
      if (event.type === 'step_start') steps++;
      // Every step's events carry its message id; Opencode 2.0 sometimes prints only an answer's text.
      const step = event.part?.messageID;
      if (event.type === 'step_finish') {
        finished.add(step);
        unfinished.delete(step);
        add(event.part || {});
        report({ ...usage });
      } else if (step && !finished.has(step)) unfinished.add(step);
      if (event.type === 'text' && typeof event.part?.text === 'string' && !event.part.synthetic) text = event.part.text;
    };
    const r = await exec(CLI, args, { input: prompt, env: { OPENCODE_CONFIG_CONTENT: config(write) }, onLine });
    // Opencode 2.0 prints no step_finish for the step that answers; its stored message has the
    // usage, saved a moment after Opencode exits, so the export is retried for up to 1.5 seconds.
    if (unfinished.size && sessionId) {
      const saved = (m) => m.type === 'assistant' && unfinished.has(m.id) && (m.tokens?.input || 0) + (m.tokens?.cache?.read || 0) > 0;
      for (let attempt = 1; ; attempt++) {
        const found = exported(sessionId).filter(saved);
        if (found.length === unfinished.size || attempt === 10) {
          found.forEach(add);
          break;
        }
        await new Promise((resolve) => setTimeout(resolve, 150));
      }
      report({ ...usage });
    }
    if (failure) return { ok: false, error: failure, usage };
    if (r.code !== 0 || !text) return { ok: false, error: text ? `exit ${r.code}` : `no answer (exit ${r.code})`, usage };
    const capped = steps >= STEPS[write ? 'write' : 'read'];
    return { ok: true, text, note: capped ? 'hit step cap; answer may be partial' : '', usage };
  },
});
