/*
 * The Cursor agent: runs `agent -p --force --output-format stream-json` with the prompt as
 * its argument. Only write runs (`ask -w`) are supported: Cursor's docs call `--mode ask` read-only
 * but say in the same breath that print mode has full write access, and Cursor is closed source,
 * so nothing here can show that the mode is enforced; a read run is refused rather than trusted
 * to a mode or a prompt. The model is passed to --model as it is (see `agent models`); an
 * effort belongs in the model id, so #effort is refused. The session id of the init event is
 * reported at once; a follow-up continues it with --resume. The answer is the last assistant
 * message. Cursor reports no usage in this output and has no title flag. Cursor is
 * $ASK_CURSOR_BIN, else `agent` or `cursor-agent` (the installer's two symlinks) on PATH or in
 * ~/.local/bin. Started by agents/cursor, which finds Node.js.
 */
import { spawn } from 'node:child_process';
import { accessSync, constants, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { delimiter, join } from 'node:path';

/* The Cursor executable: $ASK_CURSOR_BIN if set, else the first `agent` or `cursor-agent` (both are symlinks the installer makes in ~/.local/bin) on PATH or there; undefined when there is none. */
function findCli() {
  if (process.env.ASK_CURSOR_BIN) return executable(process.env.ASK_CURSOR_BIN) ? process.env.ASK_CURSOR_BIN : undefined;
  const dirs = [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(homedir(), '.local', 'bin')];
  return dirs.flatMap((dir) => ['agent', 'cursor-agent'].map((name) => join(dir, name))).find(executable);
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
  const { ASK_MODEL, ASK_EFFORT, ASK_ACCESS, ASK_SESSION, ASK_REPORT } = process.env;
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
  console.error('Cursor not found: install it from https://cursor.com/cli, or set ASK_CURSOR_BIN to its path');
  process.exit(1);
}

await adapter({
  // Cursor's model list is account-specific and its output format is undocumented: add the ids of `agent models` with ask models.
  models: () => [],

  async run({ prompt, model, effort, write, session, report }) {
    if (!write) return { ok: false, error: 'Cursor read runs are refused: its read-only mode is not verifiably enforced; use ask -w to give Cursor write access' };
    if (effort) return { ok: false, error: `Cursor takes effort in the model id (see agent models), not as #${effort}` };
    const args = ['-p', '--force', '--trust', '--output-format', 'stream-json', '--model', model];
    if (session) args.push('--resume', session);
    // A prompt that starts with a dash would be read as an option.
    args.push(prompt.startsWith('-') ? ` ${prompt}` : prompt);
    let result;
    let answer;
    let name;
    const onLine = (line) => {
      let event;
      try {
        event = JSON.parse(line);
      } catch {
        return;
      }
      if (event.type === 'system' && event.subtype === 'init') {
        name = event.model;
        report({ session: event.session_id, name });
      } else if (event.type === 'assistant') {
        const text = (event.message?.content || []).filter((part) => part.type === 'text').map((part) => part.text).join('');
        if (text) answer = text;
      } else if (event.type === 'result') result = event;
    };
    const r = await exec(CLI, args, { onLine });
    if (result?.is_error) return { ok: false, error: result.result || result.subtype, name };
    if (r.code !== 0 || !(answer ?? result?.result)) return { ok: false, error: r.stderr.trim().split('\n').pop() || `no answer (exit ${r.code})`, name };
    return { ok: true, text: answer ?? result.result, name };
  },
});
