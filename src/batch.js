/*
 * Batches: many tasks run in parallel and recorded under ~/.ask/runs/<time>-<pid>/ as tasks.json and
 * results.json, so a stopped or failed batch can be resumed and `ask runs` can list it.
 */
import { mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { modelName, parseModel } from './harness.js';
import { RUNS, UsageError } from './home.js';
import { runTask } from './task.js';
import { addUsage, formatUsage } from './usage.js';

/* A folder for a new run, named so that runs sort by time and end with the pid of the ask running them. */
export const newRunDir = () => join(RUNS, `${new Date().toISOString().replace(/[-:]/g, '').replace(/\..*/, '')}-${process.pid}`);

/*
 * Reads batch tasks from a JSON array or JSON lines. Every task needs a prompt and a model (its own
 * or the batch's -m); `check` raises the usage error for a missing one. A task's write must be a
 * boolean and defaults to the batch's -w; its dir is made absolute, so a recorded run means the same
 * when resumed.
 */
export function parseTasks(text, defaults, check) {
  const trimmed = text.trim();
  if (!trimmed) throw new UsageError('no tasks: pass a JSON array or JSON lines in FILE or on stdin');
  const items = trimmed.startsWith('[') ? JSON.parse(trimmed) : trimmed.split('\n').filter((l) => l.trim()).map((l) => JSON.parse(l));
  if (!items.length) throw new UsageError('no tasks: the batch is empty');
  return items.map((item, index) => {
    if (typeof item?.prompt !== 'string' || !item.prompt.trim()) throw new UsageError(`task ${index + 1} has no "prompt"`);
    check(item.model || defaults.model, `task ${index + 1}`);
    if (item.write !== undefined && typeof item.write !== 'boolean') throw new UsageError(`task ${index + 1}: "write" must be true or false`);
    return { ...defaults, ...item, id: item.id ?? String(index + 1), dir: resolve(item.dir ?? defaults.dir) };
  });
}

/* The tasks of a recorded run, for --resume. */
export function recordedTasks(dir) {
  try {
    return JSON.parse(readFileSync(join(dir, 'tasks.json'), 'utf8'));
  } catch {
    throw new UsageError(`no run to resume at ${dir}; see \`ask runs\``);
  }
}

/*
 * Runs tasks with at most `jobs` at once and returns results in task order. The run is recorded in
 * `dir` (tasks.json, then results.json after every task), and results already ok there are kept
 * for the task at the same position, which is how --resume skips finished work.
 */
export async function runBatch(tasks, jobs, dir) {
  mkdirSync(dir, { recursive: true });
  writeFileSync(join(dir, 'tasks.json'), JSON.stringify(tasks, null, 2));
  let saved = [];
  try {
    saved = JSON.parse(readFileSync(join(dir, 'results.json'), 'utf8'));
  } catch {}
  const results = tasks.map((_, index) => (saved[index]?.ok ? saved[index] : null));
  const todo = tasks.map((_, index) => index).filter((index) => !results[index]);
  if (todo.length < tasks.length) console.error(`ask: resuming ${dir}: ${tasks.length - todo.length} done, ${todo.length} to run`);
  else console.error(`ask: run ${dir}`);
  let next = 0;
  const worker = async () => {
    while (next < todo.length) {
      const index = todo[next++];
      console.error(`ask: [${tasks[index].id}] ${await modelName(parseModel(tasks[index].model))} started`);
      const r = (results[index] = await runTask(tasks[index]));
      writeFileSync(join(dir, 'results.json'), JSON.stringify(results, null, 2));
      console.error(`ask: [${r.id}] ${r.name} ${r.ok ? 'ok' : 'failed'} ${r.seconds}s${formatUsage(r.usage)}${r.ok ? '' : `: ${r.error}`}`);
    }
  };
  await Promise.all(Array.from({ length: Math.max(1, Math.min(jobs, todo.length)) }, worker));
  const total = results.reduce((sum, r) => (r?.usage ? addUsage(sum, r.usage) : sum), {});
  console.error(`ask: ${results.filter((r) => r?.ok).length}/${tasks.length} ok,${formatUsage(total)} (${dir})`);
  return results;
}

/*
 * Lists recent batch runs, newest first, with how many tasks finished. A run with unfinished tasks
 * whose ask process is gone (its pid ends the run folder's name) is shown as stopped, with the
 * command that resumes it.
 */
export function listRuns() {
  let names = [];
  try {
    names = readdirSync(RUNS).sort().reverse().slice(0, 20);
  } catch {}
  for (const name of names) {
    let tasks = [];
    let results = [];
    try {
      tasks = JSON.parse(readFileSync(join(RUNS, name, 'tasks.json'), 'utf8'));
      results = JSON.parse(readFileSync(join(RUNS, name, 'results.json'), 'utf8'));
    } catch {}
    const ok = results.filter((r) => r?.ok).length;
    const failed = results.filter((r) => r && !r.ok).length;
    const pending = tasks.length - ok - failed;
    const pid = Number(name.split('-').pop());
    let alive = false;
    try {
      alive = pid > 0 && process.kill(pid, 0);
    } catch {}
    const left = pending && !alive ? `${pending} stopped (ask batch --resume ${join(RUNS, name)})` : `${pending} pending`;
    console.log(`${join(RUNS, name)}\t${ok} ok\t${failed} failed\t${left}`);
  }
}
