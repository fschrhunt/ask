/*
 * The scaffolding the shipped harnesses share, implementing the harness contract from
 * docs/harnesses.md so each harness only says how to reach its CLI. A harness calls
 * adapter({ models, run }):
 *   models() -> [[id, name?], ...]   what `<harness> models` prints
 *   run({ prompt, model, effort, write, schema, dir })
 *     -> { ok, text, error, name, note, usage: { input, output, cached, cost } }
 * Local harnesses need none of this; any executable that keeps the contract works.
 */
import { spawn } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';

/*
 * Runs an agent CLI in the current directory with `input` on stdin. Resolves with { code, stdout,
 * stderr }; never rejects. No timeout: ask stops the whole process group when a run is out of time.
 */
export function exec(command, args, { input = '', env } = {}) {
  return new Promise((resolve) => {
    const child = spawn(command, args, { env: { ...process.env, ...env }, stdio: ['pipe', 'pipe', 'pipe'] });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (d) => (stdout += d));
    child.stderr.on('data', (d) => (stderr += d));
    child.on('error', (error) => (stderr += `${error.message}\n`));
    child.stdin.on('error', () => {});
    child.on('close', (code) => resolve({ code, stdout, stderr }));
    child.stdin.end(input);
  });
}

/* Runs a harness: lists its models for `models`, otherwise answers the prompt on stdin. */
export async function adapter({ models, run }) {
  if (process.argv[2] === 'models') {
    for (const [id, name] of await models()) console.log(name ? `${id}\t${name}` : id);
    return;
  }
  const { ASK_MODEL, ASK_EFFORT, ASK_ACCESS, ASK_SCHEMA, ASK_REPORT } = process.env;
  const r = await run({
    prompt: readFileSync(0, 'utf8'),
    model: ASK_MODEL,
    effort: ASK_EFFORT || undefined,
    write: ASK_ACCESS === 'write',
    schema: ASK_SCHEMA ? JSON.parse(readFileSync(ASK_SCHEMA, 'utf8')) : undefined,
    dir: process.cwd(),
  });
  if (ASK_REPORT) writeFileSync(ASK_REPORT, JSON.stringify({ name: r.name, note: r.note || undefined, ...r.usage }));
  if (r.ok) process.stdout.write(`${r.text}\n`);
  else {
    process.stderr.write(`${String(r.error).replace(/\s+/g, ' ').trim()}\n`);
    process.exitCode = 1;
  }
}
