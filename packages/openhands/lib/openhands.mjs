/* Runs the dedicated OpenHands CLI headlessly. Read runs are refused because the CLI
 * exposes no read-only sandbox or exclusive read-tool policy. JSON events supply the
 * final answer; the CLI's conversation trailer supplies the resumable session id.
 */
import { spawn } from 'node:child_process';
import { accessSync, constants, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { delimiter, join } from 'node:path';

/* Find the CLI before claiming readiness, respecting an authoritative override. */
function findCli() {
  const executable = (path) => {
    try { accessSync(path, constants.X_OK); return statSync(path).isFile(); } catch { return false; }
  };
  if (process.env.ASK_OPENHANDS_BIN) return executable(process.env.ASK_OPENHANDS_BIN) ? process.env.ASK_OPENHANDS_BIN : undefined;
  return [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(homedir(), '.local', 'bin'), '/opt/homebrew/bin', '/usr/local/bin']
    .map((dir) => join(dir, 'openhands')).find(executable);
}
const CLI = findCli();

const reported = {};

/* Publishes known report fields atomically, including sessions on failed runs. */
function report(fields) {
  Object.assign(reported, fields);
  if (!process.env.ASK_REPORT) return;
  writeFileSync(`${process.env.ASK_REPORT}.tmp`, JSON.stringify(reported));
  renameSync(`${process.env.ASK_REPORT}.tmp`, process.env.ASK_REPORT);
}

/* Implements ask's run contract using CLI events, excluding tool output and UI summaries. */
async function main() {
  if (!CLI) throw new Error('OpenHands CLI not found: install openhands or set ASK_OPENHANDS_BIN');
  if (process.argv[2] === 'models') return;
  if (process.env.ASK_ACCESS !== 'write') throw new Error('OpenHands read runs are unsupported: the CLI has no enforced read-only mode; use write access');
  const model = process.env.ASK_MODEL?.trim();
  if (!model || /(^|[/:_-])(auto|default|latest)($|[/:_-])/i.test(model)) throw new Error('OpenHands requires a concrete, explicit ASK_MODEL');
  if (process.env.ASK_EFFORT) throw new Error('OpenHands CLI has no documented per-run effort flag');
  const prompt = readFileSync(0, 'utf8');
  if (!prompt.trim()) throw new Error('OpenHands requires a nonempty prompt');
  const args = ['--headless', '--json', '--always-approve', '--override-with-envs', '--task', prompt];
  if (process.env.ASK_SESSION) args.push('--resume', process.env.ASK_SESSION);
  report({ name: model, note: 'no usage or CLI title support' });
  let text = '';
  let failure = false;
  let pending = '';
  /* Decodes the upstream event schema and ignores Rich's human-readable output. */
  const line = (raw) => {
    const session = /^Conversation ID: ([a-f0-9]{32})\s*$/.exec(raw)?.[1];
    if (session) report({ session });
    let event;
    try { event = JSON.parse(raw); } catch { return; }
    if (event.kind === 'ConversationErrorEvent') failure = true;
    if (event.kind === 'MessageEvent' && event.source === 'agent' && event.llm_message?.role === 'assistant') {
      text = (event.llm_message.content || []).filter((c) => c.type === 'text' && typeof c.text === 'string').map((c) => c.text).join('\n');
    }
    if (event.kind === 'ActionEvent' && event.action?.kind === 'FinishAction' && typeof event.action.message === 'string') text = event.action.message;
  };
  const code = await new Promise((resolve) => {
    const child = spawn(CLI, args, {
      env: { ...process.env, LLM_MODEL: model, NO_COLOR: '1', TERM: 'dumb' },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    child.stdout.setEncoding('utf8');
    child.stdout.on('data', (data) => {
      pending += data;
      for (let end; (end = pending.indexOf('\n')) !== -1;) {
        line(pending.slice(0, end));
        pending = pending.slice(end + 1);
      }
    });
    child.stderr.resume();
    child.on('error', () => { failure = true; });
    child.on('close', (status) => { if (pending) line(pending); resolve(status); });
  });
  if (code !== 0 || failure) throw new Error(`OpenHands CLI failed (exit ${code}); check installation, LLM_API_KEY and model configuration directly`);
  if (!text.trim()) throw new Error('OpenHands returned no final answer in its JSON events');
  console.log(text);
}

try { await main(); }
catch (error) { console.error(error.message); process.exitCode = 1; }
