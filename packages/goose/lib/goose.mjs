/*
 * The Goose agent: runs `goose run --output-format stream-json` with the prompt on stdin. Goose
 * reads files only through its shell tool and has no OS sandbox, so a read run cannot be kept
 * read-only and is refused; write runs set GOOSE_MODE=auto, since approve modes stop a headless
 * run. Goose cannot list models: a model is `provider/model` (--provider and --model) or a model of
 * the provider set up in Goose. An effort is GOOSE_THINKING_EFFORT. A new run names its session
 * from ASK_TITLE plus a unique suffix and reports that name at once; a follow-up resumes it with
 * --resume --name. Usage and cost come from the final complete event, which totals the whole
 * session and its subagents, so a follow-up reports none rather than count earlier turns as its
 * own. Goose is $ASK_GOOSE_BIN, else on PATH, else where its installers put it. Started by
 * agents/goose, which finds Node.js.
 */
import { spawn } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { accessSync, constants, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { delimiter, join } from 'node:path';

const READ_REFUSED = 'Goose cannot run read-only: it reads files only through its shell, which can also write, and has no sandbox; use -w';

// The efforts GOOSE_THINKING_EFFORT accepts; Goose ignores any other value without a word.
const EFFORTS = ['off', 'low', 'medium', 'high', 'max'];

/* The goose executable: $ASK_GOOSE_BIN if set, else the first on PATH or where Goose's installers put it; undefined when there is none. */
function findCli() {
  if (process.env.ASK_GOOSE_BIN) return executable(process.env.ASK_GOOSE_BIN) ? process.env.ASK_GOOSE_BIN : undefined;
  const home = homedir();
  const dirs = [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(home, '.local', 'bin'), '/opt/homebrew/bin', '/usr/local/bin'];
  return dirs.map((dir) => join(dir, 'goose')).find(executable);
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
 * The agent contract from ask's docs/agents.md: `models` prints nothing, since Goose cannot list
 * them; otherwise run() answers the prompt on stdin and returns { ok, text, error, usage }. run gets
 * session (the session to continue, from ASK_SESSION) and report(fields), which writes fields such
 * as { session } to the report at once, so a run stopped midway can still be continued.
 */
async function adapter({ run }) {
  if (process.argv[2] === 'models') return;
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
  if (r.usage) report(r.usage);
  if (r.ok) process.stdout.write(`${r.text}\n`);
  else {
    process.stderr.write(`${String(r.error).replace(/\s+/g, ' ').trim()}\n`);
    process.exitCode = 1;
  }
}

const CLI = findCli();
if (!CLI) {
  console.error('Goose not found: install it from https://github.com/aaif-goose/goose, or set ASK_GOOSE_BIN to its path');
  process.exit(1);
}

await adapter({
  async run({ prompt, model, effort, write, session, title, report }) {
    if (!write) return { ok: false, error: READ_REFUSED };
    if (effort && !EFFORTS.includes(effort)) return { ok: false, error: `Goose has no effort ${effort}; use one of ${EFFORTS.join(', ')}` };
    const slash = model.indexOf('/');
    const name = session || `${title || 'ask'} · ${randomBytes(4).toString('hex')}`;
    report({ session: name });
    const args = ['run', '-i', '-', '--output-format', 'stream-json', '--name', name];
    if (session) args.push('--resume');
    if (slash > 0) args.push('--provider', model.slice(0, slash), '--model', model.slice(slash + 1));
    else args.push('--model', model);
    // Each message streams as it grows; the answer is the assistant's text after its last tool call.
    let text = '';
    let failure;
    let usage;
    const onLine = (line) => {
      let event;
      try {
        event = JSON.parse(line);
      } catch {
        return;
      }
      if (event.type === 'error') failure ??= event.error || 'error';
      if (event.type === 'message' && event.message?.role === 'assistant') {
        for (const part of event.message.content || []) {
          if (part.type === 'toolRequest') text = '';
          if (part.type === 'text' && typeof part.text === 'string') text += part.text;
        }
      }
      if (event.type === 'complete' && !session) {
        usage = { input: event.input_tokens, output: event.output_tokens, cached: event.cache_read_input_tokens, cost: event.cost_usd };
      }
    };
    const env = { GOOSE_MODE: 'auto', ...(effort ? { GOOSE_THINKING_EFFORT: effort } : {}) };
    const r = await exec(CLI, args, { input: prompt, env, onLine });
    text = text.trim();
    if (failure) return { ok: false, error: failure, usage };
    if (r.code !== 0 || !text) {
      const reason = r.stderr.trim().split('\n').pop();
      return { ok: false, error: reason || (text ? `exit ${r.code}` : `no answer (exit ${r.code})`), usage };
    }
    return { ok: true, text, usage };
  },
});
