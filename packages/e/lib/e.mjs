/* e's ask adapter: RPC protocol 2, with an execution allowlist for read runs and saved logs. */
import { spawn } from 'node:child_process';
import { accessSync, constants, readFileSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { delimiter, join } from 'node:path';
import { createInterface } from 'node:readline';

const GROUNDING = 'Answer from the files in your working directory: search and read them before answering. Never guess file names, functions or facts; if you cannot find something, say so.\n\n';

/* Find e on PATH or in common install directories; an override is authoritative. */
function findCli() {
  const executable = (path) => {
    try { accessSync(path, constants.X_OK); return statSync(path).isFile(); } catch { return false; }
  };
  if (process.env.ASK_E_BIN) return executable(process.env.ASK_E_BIN) ? process.env.ASK_E_BIN : undefined;
  return [...(process.env.PATH || '').split(delimiter).filter(Boolean), join(homedir(), '.local', 'bin'), '/opt/homebrew/bin', '/usr/local/bin']
    .map((dir) => join(dir, 'e')).find(executable);
}

/* Merge partial reports atomically so ask can read session and usage during execution. */
function reporter() {
  const fields = {};
  return (update) => {
    Object.assign(fields, update);
    if (!process.env.ASK_REPORT) return;
    writeFileSync(`${process.env.ASK_REPORT}.tmp`, JSON.stringify(fields));
    renameSync(`${process.env.ASK_REPORT}.tmp`, process.env.ASK_REPORT);
  };
}

/* Map disjoint e usage categories; prompt_tokens in the final result includes compaction. */
function usage(u) {
  return {
    input: u.prompt_tokens ?? ((u.input_tokens || 0) + (u.cache_read_tokens || 0) + (u.cache_write_5m_tokens || 0) + (u.cache_write_1h_tokens || 0)),
    output: u.output_tokens || 0,
    cached: u.cache_read_tokens || 0,
  };
}

/* A single child RPC server: correlate responses, forward events, and reject pending calls on exit. */
function rpc(cli, onEvent) {
  const child = spawn(cli, ['rpc', '--no-extensions'], { stdio: ['pipe', 'pipe', 'pipe'] });
  const pending = new Map();
  let sequence = 0;
  let stderr = '';
  let closed = false;
  child.stdin.on('error', () => {});
  child.stderr.setEncoding('utf8');
  child.stderr.on('data', (data) => { stderr += data; });
  const finished = new Promise((resolve) => {
    child.on('error', (error) => { stderr += error.message; });
    child.on('close', (code) => {
      closed = true;
      for (const { reject } of pending.values()) reject(new Error(stderr.trim().split('\n').pop() || `e RPC closed before response (exit ${code})`));
      pending.clear();
      resolve(code);
    });
  });
  const lines = createInterface({ input: child.stdout });
  lines.on('line', (line) => {
    let message;
    try { message = JSON.parse(line); } catch { return; }
    if (message.type) { onEvent(message); return; }
    const call = pending.get(message.id);
    if (!call) return;
    pending.delete(message.id);
    if (message.error) call.reject(new Error(message.error));
    else call.resolve(message.result);
  });
  return {
    call(method, params = {}) {
      if (closed) return Promise.reject(new Error('e RPC is closed'));
      const id = String(++sequence);
      return new Promise((resolve, reject) => {
        pending.set(id, { resolve, reject });
        child.stdin.write(`${JSON.stringify({ id, method, params })}\n`);
      });
    },
    async close() { child.stdin.end(); return finished; },
  };
}

/* List models or run one persisted turn, resuming a log path rather than a process-local session id. */
async function main() {
  const cli = findCli();
  if (!cli) throw new Error('e not found: build https://github.com/arocomputer/e, or set ASK_E_BIN to its path');
  const listing = process.argv[2] === 'models';
  const { ASK_MODEL: model, ASK_EFFORT: effort, ASK_ACCESS: access, ASK_SESSION: resume, ASK_TITLE: name } = process.env;
  if (!listing && (!model || !/^[^/\s*]+\/[^\s*]+$/.test(model))) throw new Error('e requires an explicit provider/model id');
  const prompt = listing ? '' : (access !== 'write' && !resume ? GROUNDING : '') + readFileSync(0, 'utf8');
  const report = reporter();
  const totals = { input: 0, output: 0, cached: 0 };
  let session;
  let pathRequest;
  let pathError;
  let pathPending = false;
  let pathReported = false;
  const client = rpc(cli, (event) => {
    if (event.session !== session) return;
    if (['turn_start', 'usage'].includes(event.type) && !pathPending && !pathReported) {
      // New log paths become available only after the first prompt creates the log.
      pathPending = true;
      pathRequest = client.call('session.info', { session }).then((info) => {
        if (info.path) { report({ session: info.path }); pathReported = true; }
      }).catch((error) => { pathError = error; }).finally(() => { pathPending = false; });
    }
    if (event.type === 'usage') {
      const current = usage(event);
      for (const key of Object.keys(totals)) totals[key] += current[key];
      report(totals);
    }
  });
  try {
    const hello = await client.call('hello');
    if (hello?.protocol !== 2) throw new Error('e requires RPC protocol 2 with session.create tools; update your e checkout');
    if (listing) {
      const catalog = await client.call('models.list');
      const code = await client.close();
      if (code !== 0) throw new Error(`e RPC exited ${code}`);
      for (const id of [...new Set(catalog.models.map((entry) => entry.model))].sort()) console.log(id);
    } else {
      if (access !== 'write') {
        // Unknown params are ignored by older servers: verify tools is recognized before any prompt.
        let supported = false;
        try {
          await client.call('session.create', { cwd: process.cwd(), model, tools: ['__ask_read_only_probe__'] });
        } catch (error) {
          if (error.message.includes('unknown built-in tool in allowlist: `__ask_read_only_probe__`')) supported = true;
          else throw error;
        }
        if (!supported) throw new Error('e RPC does not recognize the tool allowlist; refusing read access');
      }
      const created = await client.call('session.create', {
        cwd: process.cwd(), model, effort, save: true, resume, name,
        tools: access === 'write' ? undefined : ['read', 'grep', 'read_result'],
      });
      if (!created?.session) throw new Error('e returned no session');
      session = created.session;
      report({ name: created.model, ...(created.path ? { session: created.path } : {}) });
      const result = await client.call('session.prompt', { session, prompt });
      await pathRequest;
      if (result?.path) report({ session: result.path });
      if (result?.usage) report(usage(result.usage));
      if (typeof result?.cost_usd === 'number') report({ cost: result.cost_usd });
      if (result?.warnings?.length) report({ note: result.warnings.join('; ').replace(/\s+/g, ' ') });
      if (result?.error || result?.aborted || !result?.final_output) throw new Error(result?.error || (result?.aborted ? 'e turn aborted' : 'e returned no completed answer'));
      if (pathError && !result.path) throw pathError;
      const code = await client.close();
      if (code !== 0) throw new Error(`e RPC exited ${code}`);
      process.stdout.write(`${result.final_output}\n`);
    }
  } finally {
    await client.close();
  }
}

await main().catch((error) => {
  console.error(String(error.message).replace(/\s+/g, ' ').trim());
  process.exitCode = 1;
});
