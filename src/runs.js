/*
 * Runs: every ask invocation that starts agents is a run, recorded under ~/.ask/runs/<time>-<id>/ so
 * it can be shown, continued (-c), resumed and stopped later. A run holds tasks.json (its tasks,
 * fixed once written), results.json (rewritten whole after each task) and lock (the pid of the ask
 * running it). A single prompt is a run of one task, referred to by its id alone; a batch task is
 * RUN/TASK.
 */
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { basename, join, resolve } from 'node:path';
import { parseModel } from './agent.js';
import { RUNS, UsageError, writeJson } from './home.js';
import { runTask } from './task.js';

// Seconds a task may run unless it says otherwise, and the most it may ask for (Node's timer limit).
const DEFAULT_TIMEOUT = 900;
const MAX_TIMEOUT = 2_000_000;

const alive = (pid) => {
  try {
    return pid > 0 && process.kill(pid, 0);
  } catch (error) {
    return error.code === 'EPERM';
  }
};
const readJson = (path, fallback) => {
  try {
    return JSON.parse(readFileSync(path, 'utf8'));
  } catch {
    return fallback;
  }
};

/* A new run id: six lowercase letters and digits, short enough to type. */
export const newRunId = () => Array.from({ length: 6 }, () => '0123456789abcdefghijklmnopqrstuvwxyz'[Math.floor(Math.random() * 36)]).join('');

/* How a task is named in output and for -c: the run id for a run of one task, else RUN/TASK. */
export const taskRef = (run, index) => (run.tasks.length === 1 ? run.id : `${run.id}/${run.tasks[index].id}`);

/* Loads a run from its folder; results.json that exists but cannot be read is an error, never "no results". */
function loadRun(dir) {
  const tasks = readJson(join(dir, 'tasks.json'), null);
  if (!Array.isArray(tasks)) throw new UsageError(`${dir} is not a readable run`);
  let results = tasks.map(() => null);
  if (existsSync(join(dir, 'results.json'))) {
    const saved = readJson(join(dir, 'results.json'), null);
    if (!Array.isArray(saved)) throw new UsageError(`${join(dir, 'results.json')} is damaged; move it away to rerun every task`);
    results = tasks.map((_, i) => saved[i] || null);
  }
  const name = basename(dir);
  return { id: name.slice(name.lastIndexOf('-') + 1), dir, created: name.slice(0, name.lastIndexOf('-')), tasks, results };
}

/* Starts recording a run of `tasks` under `id`. */
export function createRun(id, tasks) {
  const stamp = new Date().toISOString().replace(/[-:.Z]/g, '');
  const dir = join(RUNS, `${stamp}-${id}`);
  mkdirSync(dir, { recursive: true });
  writeJson(join(dir, 'tasks.json'), tasks);
  return loadRun(dir);
}

/*
 * Finds a run, and a task in it, from a reference: RUN or RUN/TASK, where RUN is a run id or its
 * folder, and TASK a task id or 1-based position. index is null for a bare reference to a run of
 * several tasks.
 */
export function openRun(ref) {
  if (/^[./~]/.test(ref)) {
    if (!existsSync(ref) || !statSync(ref).isDirectory()) throw new UsageError(`no run at ${ref}; see \`ask runs\``);
    return { run: loadRun(resolve(ref)), index: null };
  }
  const [id, task] = ref.split(/\/(.*)/s);
  let names = [];
  try {
    names = readdirSync(RUNS);
  } catch {}
  const name = names.find((n) => n === id || n.endsWith(`-${id}`));
  if (!id || !name) throw new UsageError(`no run ${id}; see \`ask runs\``);
  const run = loadRun(join(RUNS, name));
  if (task === undefined) return { run, index: run.tasks.length === 1 ? 0 : null };
  const matches = run.tasks.flatMap((t, i) => (String(t.id) === task ? [i] : []));
  if (matches.length > 1) throw new UsageError(`run ${id} has ${matches.length} tasks named ${task}; use their position, like ${id}/${matches[0] + 1}`);
  const index = matches[0] ?? (/^\d+$/.test(task) && run.tasks[task - 1] ? task - 1 : undefined);
  if (index === undefined) throw new UsageError(`run ${id} has no task ${task}`);
  return { run, index };
}

/* The pid of the ask running this run, or null when none is. */
export function runOwner(run) {
  const pid = Number(readJson(join(run.dir, 'lock'), 0));
  return alive(pid) ? pid : null;
}

/* Takes the run for this process; another live ask holding it is a usage error. A lock left by a dead ask is taken over. */
function lock(run) {
  const owner = runOwner(run);
  if (owner && owner !== process.pid) throw new UsageError(`run ${run.id} is running (pid ${owner}); stop it with \`ask stop ${run.id}\``);
  writeFileSync(join(run.dir, 'lock'), String(process.pid));
}

/*
 * Turns task input (from flags or a batch file) into recorded tasks. Each needs a prompt, and a model
 * unless it continues a run. A task with "continue": REF resumes that task's agent session: it runs
 * where that task ran, in its worktree if it had one, and keeps its model and access unless the task
 * sets them; a different agent is refused. `single` is true for a one-prompt run, false for a batch.
 * `missingModel(where)` raises the error for a task without a model.
 */
export function prepareTasks(items, defaults, runId, single, missingModel) {
  const one = single;
  return items.map((item, index) => {
    const where = one ? 'ask' : `task ${index + 1}`;
    if (!item || typeof item !== 'object' || Array.isArray(item)) throw new UsageError(`${where} must be a JSON object`);
    const t = { ...defaults, ...Object.fromEntries(Object.entries(item).filter(([, v]) => v !== null && v !== undefined)) };
    if (typeof t.prompt !== 'string' || !t.prompt.trim()) throw new UsageError(`${where} has no "prompt"`);
    for (const key of ['write', 'json', 'worktree'])
      if (t[key] !== undefined && typeof t[key] !== 'boolean') throw new UsageError(`${where}: "${key}" must be true or false`);
    for (const key of ['model', 'dir', 'continue'])
      if (t[key] !== undefined && typeof t[key] !== 'string') throw new UsageError(`${where}: "${key}" must be a string`);
    if (t.schema !== undefined && (typeof t.schema !== 'object' || Array.isArray(t.schema)) && typeof t.schema !== 'boolean')
      throw new UsageError(`${where}: "schema" must be a JSON Schema object`);
    const timeout = t.timeout ?? DEFAULT_TIMEOUT;
    if (!(typeof timeout === 'number' && timeout > 0 && timeout <= MAX_TIMEOUT)) throw new UsageError(`${where}: timeout must be a number of seconds above 0`);
    const task = { id: String(item.id ?? index + 1), prompt: t.prompt, model: t.model, write: t.write, json: Boolean(t.json), schema: t.schema, dir: resolve(t.dir ?? '.'), timeout };
    if (t.continue) {
      const { run, index: at } = openRun(t.continue);
      if (at === null) throw new UsageError(`run ${run.id} has ${run.tasks.length} tasks; continue one of them, like ${run.id}/${run.tasks[0].id}`);
      const [prev, result] = [run.tasks[at], run.results[at]];
      if (!result?.session) throw new UsageError(`${t.continue} cannot be continued: its agent reported no session`);
      task.model ??= prev.model;
      if (parseModel(task.model).agent !== parseModel(prev.model).agent) throw new UsageError(`${t.continue} ran on ${prev.model}; a follow-up must use the same agent`);
      Object.assign(task, { write: task.write ?? prev.write, dir: prev.dir, worktree: prev.worktree, session: result.session, continues: taskRef(run, at) });
    } else if (t.worktree) task.worktree = one ? runId : `${runId}-${task.id.replace(/[^\w.-]/g, '_')}`;
    if (!task.model) missingModel(where);
    parseModel(task.model);
    task.write = Boolean(task.write);
    if (task.worktree && !task.write) throw new UsageError(`${where}: a worktree is for write runs; add -w`);
    if (!existsSync(task.dir)) throw new UsageError(`${where}: no directory ${task.dir}`);
    return task;
  });
}

/*
 * Runs a run's unfinished tasks, at most `jobs` at once, and returns all results in task order.
 * Results already ok are kept, which is how --resume skips finished work. results.json is saved
 * after every task. `report` gets ('start', i, info) and ('done', i, result) as tasks go.
 */
export async function executeRun(run, jobs, report) {
  lock(run);
  try {
    const todo = run.tasks.map((_, i) => i).filter((i) => !run.results[i]?.ok);
    let next = 0;
    const worker = async () => {
      while (next < todo.length) {
        const i = todo[next++];
        const result = await runTask(run.tasks[i], (info) => report('start', i, info));
        run.results[i] = { run: taskRef(run, i), ...result };
        writeJson(join(run.dir, 'results.json'), run.results);
        report('done', i, run.results[i]);
      }
    };
    await Promise.all(Array.from({ length: Math.min(jobs, todo.length) }, worker));
    return run.results;
  } finally {
    rmSync(join(run.dir, 'lock'), { force: true });
  }
}

/* The most recent runs, newest first. */
export function recentRuns(limit) {
  let names = [];
  try {
    names = readdirSync(RUNS).filter((name) => name.includes('-'));
  } catch {}
  return names
    .sort()
    .reverse()
    .slice(0, limit)
    .flatMap((name) => {
      try {
        return [loadRun(join(RUNS, name))];
      } catch {
        return [];
      }
    });
}

/* Stops the ask running a run, waiting up to 10 seconds for it to stop its agents and exit. Returns false if none was running. */
export async function stopRun(run) {
  const pid = runOwner(run);
  if (!pid) return false;
  process.kill(pid, 'SIGTERM');
  for (let i = 0; i < 200 && alive(pid); i++) await new Promise((done) => setTimeout(done, 50));
  return true;
}
