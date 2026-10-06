/* Runs Continue's headless CLI with a single explicit model. Read runs are refused:
 * upstream plan mode allows Bash and MCP, and overrides CLI permission exclusions.
 * ASK_CONTINUE_CONFIG is a JSON-encoded Continue YAML config; no YAML parser is bundled.
 */
import { spawn } from 'node:child_process';
import { accessSync, constants, mkdtempSync, readFileSync, renameSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { homedir, tmpdir } from 'node:os';
import { delimiter, join } from 'node:path';

/* Find cn before claiming readiness; an explicit override never falls back. */
function findCli() {
  const executable = (path) => {
    try { accessSync(path, constants.X_OK); return statSync(path).isFile(); } catch { return false; }
  };
  if (process.env.ASK_CONTINUE_BIN) return executable(process.env.ASK_CONTINUE_BIN) ? process.env.ASK_CONTINUE_BIN : undefined;
  return [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(homedir(), '.local', 'bin'), '/opt/homebrew/bin', '/usr/local/bin']
    .map((dir) => join(dir, 'cn')).find(executable);
}
const CLI = findCli();

/* Loads concrete chat models, preserving provider settings without printing credentials. */
function config() {
  if (!process.env.ASK_CONTINUE_CONFIG) throw new Error('set ASK_CONTINUE_CONFIG to a JSON-encoded Continue config with explicit models');
  let value;
  try { value = JSON.parse(readFileSync(process.env.ASK_CONTINUE_CONFIG, 'utf8')); }
  catch { throw new Error('ASK_CONTINUE_CONFIG must be a readable JSON-encoded Continue config'); }
  if (!Array.isArray(value.models)) throw new Error('Continue config must contain models');
  return value;
}

/* Whether a config entry identifies a concrete model usable for chat. */
function concrete(model) {
  return typeof model?.model === 'string' && model.model.trim() &&
    !/(^|[/:_-])(auto|autodetect|default|latest)($|[/:_-])/i.test(model.model) &&
    typeof model.provider === 'string' && (!model.roles || model.roles.includes('chat')) && !model.uses;
}

/* Writes the optional ask report atomically. */
function report(fields) {
  if (!process.env.ASK_REPORT) return;
  writeFileSync(`${process.env.ASK_REPORT}.tmp`, JSON.stringify(fields));
  renameSync(`${process.env.ASK_REPORT}.tmp`, process.env.ASK_REPORT);
}

/* Runs cn in ask's process group, collecting only its final headless answer. */
function run(args, prompt) {
  return new Promise((resolve) => {
    const child = spawn(CLI, args, { stdio: ['pipe', 'pipe', 'pipe'] });
    let stdout = '';
    let failed = false;
    child.stdout.setEncoding('utf8');
    child.stdout.on('data', (data) => { stdout += data; });
    // CLI diagnostics can include provider credentials; expose only a generic failure.
    child.stderr.resume();
    child.on('error', () => { failed = true; });
    child.stdin.on('error', () => {});
    child.on('close', (code) => resolve({ code, stdout, failed }));
    child.stdin.end(prompt);
  });
}

/* Implements the ask models/run contract, without falling back to a configured default. */
async function main() {
  if (!CLI) throw new Error('Continue CLI not found: install cn or set ASK_CONTINUE_BIN');
  const value = config();
  if (process.argv[2] === 'models') {
    for (const id of [...new Set(value.models.filter(concrete).map((m) => m.model.toLowerCase()))].sort()) console.log(id);
    return;
  }
  if (process.env.ASK_ACCESS !== 'write') throw new Error('Continue read runs are unsupported: plan mode allows shell and MCP tools; use write access');
  if (process.env.ASK_SESSION) throw new Error('Continue cannot resume a specific session in headless mode; --resume selects the latest and --fork creates a new session');
  if (process.env.ASK_EFFORT) throw new Error('Continue has no documented per-run effort flag');
  const model = process.env.ASK_MODEL?.trim();
  if (!model) throw new Error('Continue requires an explicit ASK_MODEL');
  const matches = value.models.filter((m) => concrete(m) && m.model.toLowerCase() === model.toLowerCase());
  if (matches.length !== 1) throw new Error('ASK_MODEL must match exactly one concrete chat model in ASK_CONTINUE_CONFIG');
  const selected = matches[0];
  const prompt = readFileSync(0, 'utf8');
  if (!prompt.trim()) throw new Error('Continue requires a nonempty prompt');
  const dir = mkdtempSync(join(tmpdir(), 'ask-continue-'));
  try {
    const path = join(dir, 'config.yaml');
    writeFileSync(path, JSON.stringify({ ...value, models: [selected] }), { mode: 0o600 });
    report({ name: selected.model, note: 'no session, usage or CLI title support' });
    const result = await run(['-p', '--silent', '--config', path, '--allow', '*'], prompt);
    if (result.failed) throw new Error('Continue CLI not found or could not start; install cn or set ASK_CONTINUE_BIN');
    if (result.code !== 0) throw new Error(`Continue CLI failed (exit ${result.code}); inspect cn directly for diagnostics`);
    if (!result.stdout.trim()) throw new Error('Continue returned no answer');
    process.stdout.write(result.stdout.endsWith('\n') ? result.stdout : `${result.stdout}\n`);
  } finally { rmSync(dir, { recursive: true, force: true }); }
}

try { await main(); }
catch (error) { console.error(error.message); process.exitCode = 1; }
