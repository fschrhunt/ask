/*
 * Hooks: executables in ~/.ask/hooks (or a package's) that change a task before it runs and check
 * its result after, for every run. See docs/hooks.md for the contract. In short:
 *   NAME events          prints the events it handles, one per line: task, result
 *   NAME task            stdin {task}; may print {"task": {changes}}, {"refuse": why}, {"note": text}
 *   NAME result          stdin {task, result}; may print {"followup": prompt}, {"fail": why}, {"note": text}
 * Hooks run in name order, each seeing the previous one's changes. They fail open: a hook that
 * crashes, times out or prints something unreadable changes nothing and leaves a note; only an
 * explicit refuse or fail stops a task.
 */
import { find } from './find.js';
import { contractEnv } from './home.js';
import { reason, run } from './process.js';

// Milliseconds a hook gets to declare its events, and to handle one: long enough to run a test suite.
const EVENTS_TIMEOUT_MS = 10_000;
const HOOK_TIMEOUT_MS = 600_000;
// The task fields a task hook may change, with the type each must keep.
const CHANGEABLE = { prompt: 'string', model: 'string', write: 'boolean', json: 'boolean', timeout: 'number', schema: 'object' };

const declared = new Map();

/* The hooks that handle an event, as [name, path], in name order. */
async function hooksFor(event) {
  const hooks = [...find('hooks')].sort(([a], [b]) => a.localeCompare(b));
  for (const [name, path] of hooks)
    if (!declared.has(path)) {
      const r = await run(path, ['events'], { env: contractEnv({}), timeoutMs: EVENTS_TIMEOUT_MS });
      declared.set(path, r.code === 0 ? r.stdout.split(/\s+/).filter(Boolean) : []);
    }
  return hooks.filter(([, path]) => declared.get(path).includes(event));
}

/* Runs one hook on one event. Resolves with its answer object, or { error } when it gave none it could read. */
async function call(path, event, input, { dir, ref }) {
  const r = await run(path, [event], { input: JSON.stringify(input), cwd: dir, env: contractEnv({ ASK_RUN: ref, ASK_EVENT: event }), timeoutMs: HOOK_TIMEOUT_MS });
  if (r.timedOut) return { error: 'timed out' };
  if (r.code !== 0) return { error: reason(r.stderr) || `exit ${r.code}` };
  if (!r.stdout.trim()) return {};
  try {
    const answer = JSON.parse(r.stdout);
    return answer && typeof answer === 'object' && !Array.isArray(answer) ? answer : { error: 'printed something other than a JSON object' };
  } catch {
    return { error: 'printed something other than JSON' };
  }
}

/*
 * Runs the task hooks on a task about to start. Resolves with { task, refused, notes }: the task
 * with every hook's changes applied, the refusal ("name: why") if one refused, and notes as
 * [name, text] for status lines.
 */
export async function beforeTask(task, ctx) {
  const notes = [];
  for (const [name, path] of await hooksFor('task')) {
    const answer = await call(path, 'task', { task }, { ...ctx, dir: task.dir });
    if (answer.error) notes.push([name, `failed: ${answer.error}`]);
    if (typeof answer.note === 'string' && answer.note.trim()) notes.push([name, answer.note.trim()]);
    if (typeof answer.refuse === 'string') return { task, refused: `${name}: ${answer.refuse || 'refused'}`, notes };
    for (const [key, value] of Object.entries(answer.task && typeof answer.task === 'object' ? answer.task : {})) {
      const kind = CHANGEABLE[key];
      if (kind && typeof value === kind && value !== null && (kind !== 'number' || value > 0)) task = { ...task, [key]: value };
      else notes.push([name, `ignored a change to "${key}"`]);
    }
  }
  return { task, refused: null, notes };
}

/*
 * Runs the result hooks on a finished task. Resolves with { followup, fail, notes }: the first hook
 * to ask for a follow-up or to fail the result ends the chain, as [name, prompt] or [name, why].
 */
export async function afterTask(task, result, ctx) {
  const notes = [];
  for (const [name, path] of await hooksFor('result')) {
    const answer = await call(path, 'result', { task, result }, { ...ctx, dir: result.dir || task.dir });
    if (answer.error) notes.push([name, `failed: ${answer.error}`]);
    if (typeof answer.note === 'string' && answer.note.trim()) notes.push([name, answer.note.trim()]);
    if (typeof answer.fail === 'string') return { fail: [name, answer.fail || 'failed'], followup: null, notes };
    if (typeof answer.followup === 'string' && answer.followup.trim()) return { followup: [name, answer.followup.trim()], fail: null, notes };
  }
  return { followup: null, fail: null, notes };
}
