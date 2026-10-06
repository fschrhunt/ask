/*
 * The Cline agent: drives `cline --acp`, the Agent Client Protocol server of Cline CLI, as its
 * client, over newline-delimited JSON-RPC on stdio, in act mode, approving every tool call. Read
 * runs are refused: plan mode keeps the shell behind a check of the command's text, the CLI takes no
 * tool rules, and while ACP asks the client before tool calls, a Cline plugin's beforeTool hook can
 * auto-approve a call past that question, with no option to turn plugins off for a run.
 * CLINE_SESSION_BACKEND_MODE=local keeps the run in this process group rather than Cline's
 * background hub. Models are those Cline offers for the provider you signed in with
 * ($ASK_CLINE_PROVIDER picks another), set explicitly on every run. The session id from
 * session/new is reported at once; a follow-up loads it with session/load. Cline is $ASK_CLINE_BIN,
 * else on PATH, else where npm puts it. Started by agents/cline, which finds Node.js.
 */
import { spawn } from 'node:child_process';
import { accessSync, constants, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { delimiter, join } from 'node:path';

const READ_REFUSED = "Cline cannot run read-only: its plugins can auto-approve tool calls past ask's approvals, and nothing turns them off for a run; use -w";

/* The cline executable: $ASK_CLINE_BIN if set, else the first on PATH or where npm puts it; undefined when there is none. */
function findCli() {
  if (process.env.ASK_CLINE_BIN) return executable(process.env.ASK_CLINE_BIN) ? process.env.ASK_CLINE_BIN : undefined;
  const home = homedir();
  const dirs = [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(home, '.npm-global', 'bin'), join(home, '.local', 'bin'), '/opt/homebrew/bin', '/usr/local/bin', join(home, '.bun', 'bin')];
  return dirs.map((dir) => join(dir, 'cline')).find(executable);
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
 * Starts `cline --acp` and returns { request(method, params), close() }. request resolves with the
 * result or rejects with the error's message, or with Cline's last stderr line if it exits first.
 * onPermission(params) answers Cline's session/request_permission with an option id; onUpdate gets
 * every session/update. Other requests from Cline are refused as unsupported.
 */
function connect({ onPermission, onUpdate }) {
  const env = { ...process.env, CLINE_SESSION_BACKEND_MODE: 'local' };
  if (process.env.ASK_CLINE_PROVIDER) env.CLINE_PROVIDER = process.env.ASK_CLINE_PROVIDER;
  const child = spawn(CLI, ['--acp'], { env, stdio: ['pipe', 'pipe', 'pipe'] });
  const pending = new Map();
  let next = 0;
  let stderr = '';
  let buffer = '';
  const send = (message) => child.stdin.write(`${JSON.stringify({ jsonrpc: '2.0', ...message })}\n`);
  const fail = (reason) => {
    for (const { reject } of pending.values()) reject(new Error(reason));
    pending.clear();
  };
  child.stdin.on('error', () => {});
  child.stderr.setEncoding('utf8');
  child.stderr.on('data', (d) => (stderr += d));
  child.on('error', (error) => fail(error.message));
  child.on('close', (code) => fail(stderr.trim().split('\n').pop() || `Cline exited (${code})`));
  child.stdout.setEncoding('utf8');
  child.stdout.on('data', (d) => {
    buffer += d;
    for (let end; (end = buffer.indexOf('\n')) !== -1; ) {
      const line = buffer.slice(0, end);
      buffer = buffer.slice(end + 1);
      let message;
      try {
        message = JSON.parse(line);
      } catch {
        continue;
      }
      if (message.method === 'session/update') onUpdate(message.params?.update || {});
      else if (message.method === 'session/request_permission') send({ id: message.id, result: { outcome: { outcome: 'selected', optionId: onPermission(message.params || {}) } } });
      else if (message.method && message.id !== undefined) send({ id: message.id, error: { code: -32601, message: `${message.method} is not supported` } });
      else if (pending.has(message.id)) {
        const { resolve, reject } = pending.get(message.id);
        pending.delete(message.id);
        if (message.error) reject(new Error(message.error.data?.message || message.error.message || 'error'));
        else resolve(message.result);
      }
    }
  });
  return {
    request: (method, params) =>
      new Promise((resolve, reject) => {
        const id = ++next;
        pending.set(id, { resolve, reject });
        send({ id, method, params });
      }),
    close: () => {
      child.stdin.end();
      child.kill();
    },
  };
}

/* Starts a session, or loads `session`, in the current directory; resolves with { sessionId, models }. */
async function open(cline, session) {
  await cline.request('initialize', { protocolVersion: 1, clientCapabilities: { fs: { readTextFile: false, writeTextFile: false }, terminal: false } });
  const params = { cwd: process.cwd(), mcpServers: [] };
  const opened = session ? await cline.request('session/load', { sessionId: session, ...params }) : await cline.request('session/new', params);
  return { sessionId: session || opened.sessionId, models: opened.models?.availableModels || [] };
}

/* The model Cline offers that `model` names: its id without regard to case, or the part after its last slash. */
function find(models, model) {
  const last = (id) => id.slice(id.lastIndexOf('/') + 1);
  const name = model.toLowerCase();
  return models.find((m) => m.modelId.toLowerCase() === name) || models.find((m) => last(m.modelId).toLowerCase() === name);
}

/*
 * The agent contract from ask's docs/agents.md: `models` prints [id, name] pairs from models();
 * otherwise run() answers the prompt on stdin and returns
 * { ok, text, error, name }. run gets session (the session to continue, from ASK_SESSION) and
 * report(fields), which writes fields such as { session } to the report at once, so a run stopped
 * midway can still be continued.
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
  if (r.name) report({ name: r.name });
  if (r.ok) process.stdout.write(`${r.text}\n`);
  else {
    process.stderr.write(`${String(r.error).replace(/\s+/g, ' ').trim()}\n`);
    process.exitCode = 1;
  }
}

const CLI = findCli();
if (!CLI) {
  console.error('Cline not found: install it with npm install -g cline, or set ASK_CLINE_BIN to its path');
  process.exit(1);
}

await adapter({
  // The models Cline offers for its provider, as Cline names them.
  async models() {
    const cline = connect({ onPermission: () => 'reject_once', onUpdate: () => {} });
    try {
      return (await open(cline)).models.map((m) => [m.modelId.toLowerCase(), m.name && m.name !== m.modelId ? m.name : '']);
    } finally {
      cline.close();
    }
  },

  async run({ prompt, model, effort, write, session, report }) {
    if (!write) return { ok: false, error: READ_REFUSED };
    if (effort) return { ok: false, error: `Cline's ACP mode takes no effort; run cline:${model} without #${effort}` };
    // The answer is the text after the last tool call; text Cline replays when loading a session is skipped.
    let collecting = false;
    let text = '';
    const onUpdate = (update) => {
      if (!collecting) return;
      if (update.sessionUpdate === 'tool_call') text = '';
      if (update.sessionUpdate === 'agent_message_chunk' && update.content?.type === 'text') text += update.content.text;
    };
    const cline = connect({ onPermission: () => 'allow_once', onUpdate });
    try {
      const { sessionId, models } = await open(cline, session);
      report({ session: sessionId });
      const chosen = find(models, model);
      if (!chosen) return { ok: false, error: `no Cline model named ${model}; see ask models cline` };
      await cline.request('session/set_config_option', { sessionId, configId: 'model', value: chosen.modelId });
      await cline.request('session/set_mode', { sessionId, modeId: 'act' });
      collecting = true;
      const { stopReason } = await cline.request('session/prompt', { sessionId, prompt: [{ type: 'text', text: prompt }] });
      text = text.trim();
      const name = chosen.name || chosen.modelId;
      if (stopReason !== 'end_turn') return { ok: false, error: `Cline stopped: ${stopReason}`, name };
      if (!text) return { ok: false, error: 'no answer', name };
      return { ok: true, text, name };
    } catch (error) {
      return { ok: false, error: error.message };
    } finally {
      cline.close();
    }
  },
});
