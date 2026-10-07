/*
 * The Amp agent: runs `amp --execute --stream-json` with the prompt on stdin. Only write runs
 * (`ask -w`) are supported: Amp has no native read-only switch that a workspace setting, plugin or
 * user setting cannot override (amp.tools.enable is one more overridable setting, and the tool
 * list Amp prints comes only after it has started), so a read run is refused rather than trusted.
 * Amp has no model flag, only routing modes (low, medium, high, ultra) whose models Amp chooses,
 * so the model name is the mode. Write runs pass --dangerously-allow-all. The init event's
 * thread id and mode form the opaque session; a follow-up validates the mode then uses
 * `threads continue`. Usage is
 * summed from the assistant messages; Amp reports no cost. Amp is $ASK_AMP_BIN, else on PATH, else
 * where its installers put it. Started by agents/amp, which finds Node.js.
 */
import { spawn } from 'node:child_process';
import { accessSync, constants, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { delimiter, join } from 'node:path';

// Amp's built-in modes, from its docs; any other id is passed on for plugin-defined modes.
const MODES = [['low', 'Amp Low'], ['medium', 'Amp Medium'], ['high', 'Amp High'], ['ultra', 'Amp Ultra']];

/* The amp executable: $ASK_AMP_BIN if set, else the first on PATH or where Amp's installers put it; undefined when there is none. */
function findCli() {
  if (process.env.ASK_AMP_BIN) return executable(process.env.ASK_AMP_BIN) ? process.env.ASK_AMP_BIN : undefined;
  const home = homedir();
  const dirs = [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(process.env.AMP_HOME || join(home, '.amp'), 'bin'), join(home, '.local', 'bin'), '/opt/homebrew/bin', '/usr/local/bin', join(home, '.bun', 'bin')];
  return dirs.map((dir) => join(dir, 'amp')).find(executable);
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
 * Runs an agent CLI in the current directory with `input` on stdin, calling onLine(line, child)
 * with each line of stdout as it arrives. Resolves with { code, stdout, stderr }; never rejects. No
 * timeout: ask stops the whole process group when a run is out of time.
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
      for (let end; onLine && (end = stdout.indexOf('\n', seen)) !== -1; seen = end + 1) onLine(stdout.slice(seen, end), child);
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
 * otherwise run() answers the prompt on stdin and returns
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
  const { ASK_MODEL, ASK_EFFORT, ASK_ACCESS, ASK_SESSION, ASK_REPORT, ASK_TITLE } = process.env;
  const reported = {};
  const report = (fields) => {
    Object.assign(reported, fields);
    if (!ASK_REPORT) return;
    // ask reads the report while the run goes, so replace it whole: never let it see half a file.
    writeFileSync(`${ASK_REPORT}.tmp`, JSON.stringify(reported));
    renameSync(`${ASK_REPORT}.tmp`, ASK_REPORT);
  };
  const r = await run({
    prompt: readFileSync(0, 'utf8'),
    model: ASK_MODEL,
    effort: ASK_EFFORT || undefined,
    write: ASK_ACCESS === 'write',
    session: ASK_SESSION || undefined,
    title: ASK_TITLE || undefined,
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
  console.error('Amp not found: install it from https://ampcode.com, or set ASK_AMP_BIN to its path');
  process.exit(1);
}

await adapter({
  models: () => MODES,

  async run({ prompt, model, effort, write, session, title, report }) {
    if (!write) return { ok: false, error: 'Amp read runs are refused: Amp has no read-only mode its settings cannot override; use ask -w to give Amp write access' };
    let thread;
    if (session) {
      let saved;
      try { saved = JSON.parse(session); } catch {}
      if (!Array.isArray(saved) || saved.length !== 2 || saved.some((v) => typeof v !== 'string' || !v)) return { ok: false, error: 'invalid Amp continuation record; start a new run' };
      if (saved[0] !== model) return { ok: false, error: 'Amp cannot change the mode of an existing thread; start a new run' };
      thread = saved[1];
    }
    // The opaque session records its mode because Amp ignores --mode when continuing.
    const args = thread ? ['threads', 'continue', thread, '--execute', '--stream-json'] : ['--execute', '--stream-json', '--mode', model];
    if (effort) args.push('--effort', effort);
    if (title && !session) args.push('--title', title);
    args.push('--dangerously-allow-all');
    let result;
    const usage = {};
    const onLine = (line) => {
      let event;
      try {
        event = JSON.parse(line);
      } catch {
        return;
      }
      if (event.type === 'system' && event.subtype === 'init') {
        if (event.session_id) report({ session: JSON.stringify([model, event.session_id]) });
      } else if (event.type === 'assistant' && event.message?.usage) {
        const u = event.message.usage;
        const cached = u.cache_read_input_tokens || 0;
        usage.input = (usage.input || 0) + (u.input_tokens || 0) + cached + (u.cache_creation_input_tokens || 0);
        usage.output = (usage.output || 0) + (u.output_tokens || 0);
        usage.cached = (usage.cached || 0) + cached;
        report({ ...usage });
      } else if (event.type === 'result') result = event;
    };
    const r = await exec(CLI, args, { input: prompt, onLine });
    const name = MODES.find(([id]) => id === model)?.[1] || `Amp ${model}`;
    if (result?.is_error) return { ok: false, error: result.error || result.subtype, usage, name };
    if (r.code !== 0 || typeof result?.result !== 'string') return { ok: false, error: r.stderr.trim().split('\n').pop() || `no answer (exit ${r.code})`, usage, name };
    return { ok: true, text: result.result, usage, name };
  },
});
