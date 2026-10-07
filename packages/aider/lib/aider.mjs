/*
 * The Aider agent: runs `aider --message-file` once per task, with no session to continue. Only
 * write runs (`ask -w`) are supported: Aider's ask mode applies no edits, but Aider still writes
 * its repo-map cache (.aider.tags.cache.v4) into the project, and its config files and
 * environment can switch on linting, testing, committing and loading by themselves, so a read run
 * is refused rather than trusted. Aider runs a message that starts with / or ! as a command, so
 * such a prompt gets a leading space. Aider's own files (chat, input and LLM history, analytics
 * log) go to a temporary directory, never the repository, and .gitignore is left alone. Aider
 * does not commit, so ask sees the changes. The answer is the last response in the LLM history
 * file, and usage is summed from the analytics log, which has an event for every successful
 * response: none means the run failed, since Aider exits 0 either way. The model is passed to
 * --model as it is. Aider is $ASK_AIDER_BIN, else on PATH, else where its installers put it.
 * Started by agents/aider, which finds Node.js.
 */
import { spawn } from 'node:child_process';
import { accessSync, constants, existsSync, mkdtempSync, readFileSync, renameSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { homedir, tmpdir } from 'node:os';
import { delimiter, join } from 'node:path';

/* The aider executable: $ASK_AIDER_BIN if set, else the first on PATH or where Aider's installers put it; undefined when there is none. */
function findCli() {
  if (process.env.ASK_AIDER_BIN) return executable(process.env.ASK_AIDER_BIN) ? process.env.ASK_AIDER_BIN : undefined;
  const dirs = [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(homedir(), '.local', 'bin'), '/opt/homebrew/bin', '/usr/local/bin'];
  return dirs.map((dir) => join(dir, 'aider')).find(executable);
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
 * otherwise run() answers the prompt on stdin, and returns
 * { ok, text, error, name, note, usage }. Aider has no sessions, so none is reported or continued.
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
  const { ASK_MODEL, ASK_EFFORT, ASK_ACCESS, ASK_REPORT } = process.env;
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
  });
  report({ name: r.name, note: r.note || undefined, ...r.usage });
  if (r.ok) process.stdout.write(`${r.text}\n`);
  else {
    process.stderr.write(`${String(r.error).replace(/\s+/g, ' ').trim()}\n`);
    process.exitCode = 1;
  }
}

/* The last non-empty line of text, or ''. */
const lastLine = (text) => text.trim().split('\n').pop().trim();

/*
 * The text of the last LLM response in Aider's LLM history file, whose lines start with the role:
 * "LLM RESPONSE <time>", then "ASSISTANT <line>" for each line, up to the next "TO LLM <time>".
 */
function lastResponse(history) {
  const header = /^(?:TO LLM|LLM RESPONSE) \d{4}-\d\d-\d\dT[\d:]+$/;
  let text = '';
  let inside = false;
  for (const line of history.split('\n')) {
    if (header.test(line)) {
      inside = line.startsWith('LLM RESPONSE');
      if (inside) text = '';
    } else if (inside) text += `${line.replace(/^ASSISTANT ?/, '')}\n`;
  }
  return text.trim();
}

/* The tokens and cost of every response in Aider's analytics log, which has one JSON event per line. */
function usageOf(log) {
  const usage = {};
  for (const line of log.split('\n')) {
    let event;
    try {
      event = JSON.parse(line);
    } catch {
      continue;
    }
    if (event.event !== 'message_send') continue;
    const p = event.properties;
    usage.input = (usage.input || 0) + (Number(p.prompt_tokens) || 0);
    usage.output = (usage.output || 0) + (Number(p.completion_tokens) || 0);
    usage.cost = (usage.cost || 0) + (Number(p.cost) || 0);
  }
  return usage;
}

const CLI = findCli();
if (!CLI) {
  console.error('Aider not found: install it from https://aider.chat, or set ASK_AIDER_BIN to its path');
  process.exit(1);
}

await adapter({
  // Aider takes any model litellm knows, and its list needs a search term: add the ids you use with ask models.
  models: () => [],

  async run({ prompt, model, effort, write }) {
    if (!write) return { ok: false, error: 'Aider read runs are refused: Aider writes its repo-map cache into the project and its config can run commands; use ask -w to give Aider write access' };
    const dir = mkdtempSync(join(tmpdir(), 'ask-aider-'));
    const path = (name) => join(dir, name);
    // A message starting with / or ! would run as a command.
    writeFileSync(path('message'), /^[/!]/.test(prompt) ? ` ${prompt}` : prompt);
    const args = ['--model', model, '--message-file', path('message'), '--yes-always', '--no-pretty', '--no-stream', '--no-fancy-input', '--no-check-update', '--no-show-release-notes', '--no-analytics', '--no-gitignore', '--no-detect-urls', '--no-auto-commits', '--no-dirty-commits', '--no-suggest-shell-commands', '--no-auto-lint', '--chat-history-file', path('chat.md'), '--input-history-file', path('input'), '--llm-history-file', path('llm'), '--analytics-log', path('analytics')];
    if (effort) args.push('--reasoning-effort', effort);
    try {
      const r = await exec(CLI, args);
      const text = existsSync(path('llm')) ? lastResponse(readFileSync(path('llm'), 'utf8')) : '';
      const usage = existsSync(path('analytics')) ? usageOf(readFileSync(path('analytics'), 'utf8')) : {};
      if (r.code !== 0 || usage.input === undefined || !text) return { ok: false, error: lastLine(r.stderr) || lastLine(r.stdout) || `no answer (exit ${r.code})`, usage, name: model };
      return { ok: true, text, usage, name: model };
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  },
});
