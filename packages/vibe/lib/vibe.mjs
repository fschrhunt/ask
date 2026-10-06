/* Runs Vibe's programmatic CLI. An explicit model definition in VIBE_MODELS pins the
 * backend id, avoiding alias/default selection. Read runs are refused because higher
 * priority profiles can override CLI tool filters. Public history entries provide the
 * session and final assistant text, but no usage.
 */
import { spawn } from 'node:child_process';
import { accessSync, constants, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { delimiter, join } from 'node:path';

/* Find Vibe before claiming readiness; an explicit override never falls back. */
function findCli() {
  const executable = (path) => {
    try { accessSync(path, constants.X_OK); return statSync(path).isFile(); } catch { return false; }
  };
  if (process.env.ASK_VIBE_BIN) return executable(process.env.ASK_VIBE_BIN) ? process.env.ASK_VIBE_BIN : undefined;
  return [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(homedir(), '.local', 'bin'), '/opt/homebrew/bin', '/usr/local/bin']
    .map((dir) => join(dir, 'vibe')).find(executable);
}
const CLI = findCli();

const reported = {};

/* Publishes the session immediately using ask's atomic report contract. */
function report(fields) {
  Object.assign(reported, fields);
  if (!process.env.ASK_REPORT) return;
  writeFileSync(`${process.env.ASK_REPORT}.tmp`, JSON.stringify(reported));
  renameSync(`${process.env.ASK_REPORT}.tmp`, process.env.ASK_REPORT);
}

/* Reads explicit backend model definitions through Vibe's documented JSON environment layer. */
function models() {
  let values;
  try { values = JSON.parse(process.env.VIBE_MODELS || '[]'); }
  catch { throw new Error('VIBE_MODELS must be a JSON array of explicit Vibe model definitions'); }
  if (!Array.isArray(values)) throw new Error('VIBE_MODELS must be a JSON array of explicit Vibe model definitions');
  return values.filter((m) => typeof m?.name === 'string' && m.name.trim() &&
    typeof m.provider === 'string' && m.provider.trim() &&
    !/(^|[/:_-])(auto|default|latest)($|[/:_-])/i.test(m.name));
}

/* Implements ask's write-run contract with exact session resumption. */
async function main() {
  if (!CLI) throw new Error('Vibe CLI not found: install vibe or set ASK_VIBE_BIN');
  const available = models();
  if (process.argv[2] === 'models') {
    for (const id of [...new Set(available.map((m) => m.name.toLowerCase()))].sort()) console.log(id);
    return;
  }
  if (process.env.ASK_ACCESS !== 'write') throw new Error('Vibe read runs are unsupported: agent profiles can override CLI tool filters; use write access');
  const model = process.env.ASK_MODEL?.trim();
  if (!model) throw new Error('Vibe requires an explicit ASK_MODEL');
  const matches = available.filter((m) => m.name.toLowerCase() === model.toLowerCase());
  if (matches.length !== 1) throw new Error('ASK_MODEL must match exactly one concrete model in VIBE_MODELS; include its name, provider and alias');
  if (process.env.ASK_EFFORT) throw new Error('Vibe CLI has no documented per-run effort flag');
  const prompt = readFileSync(0, 'utf8');
  if (!prompt.trim()) throw new Error('Vibe requires a nonempty prompt');
  const selected = matches[0];
  const args = ['-p', '--output', 'streaming', '--agent', 'auto-approve', '--auto-approve'];
  if (process.env.ASK_MAX_COST) {
    const limit = Number(process.env.ASK_MAX_COST);
    if (!Number.isFinite(limit) || limit <= 0) throw new Error('Vibe requires a positive ASK_MAX_COST');
    args.push('--max-price', String(limit));
  }
  if (process.env.ASK_SESSION) args.push('--resume', process.env.ASK_SESSION);
  report({ note: 'no usage or CLI title support' });
  let text = '';
  let pending = '';
  let failed = false;
  /* Parses completed public history entries; reasoning and tool responses are excluded. */
  const line = (raw) => {
    let entry;
    try { entry = JSON.parse(raw); } catch { return; }
    if (typeof entry.sessionId === 'string' && entry.sessionId) report({ session: entry.sessionId });
    if (entry.type === 'message' && entry.role === 'user') text = '';
    if (entry.type === 'message' && entry.role === 'assistant' && entry.generationStatus === 'completed') {
      text = (entry.content || []).filter((c) => c.type === 'text' && typeof c.text === 'string').map((c) => c.text).join('\n\n');
    }
  };
  const code = await new Promise((resolve) => {
    const child = spawn(CLI, args, {
      env: {
        ...process.env,
        VIBE_MODELS: JSON.stringify([{ ...selected, alias: selected.name }]),
        VIBE_ACTIVE_MODEL: selected.name,
        VIBE_ALLOWED_MODELS: JSON.stringify([selected.name]),
      },
      stdio: ['pipe', 'pipe', 'pipe'],
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
    child.on('error', () => { failed = true; });
    child.stdin.on('error', () => {});
    child.on('close', (status) => { if (pending) line(pending); resolve(status); });
    child.stdin.end(prompt);
  });
  if (failed || code !== 0) throw new Error(`Vibe CLI failed (exit ${code}); check installation, credentials and configuration directly`);
  if (!text.trim()) throw new Error('Vibe returned no completed assistant answer; this adapter requires the public-history streaming format');
  console.log(text);
}

try { await main(); }
catch (error) { console.error(error.message); process.exitCode = 1; }
