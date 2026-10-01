/*
 * What people read: the status lines ask prints on stderr while runs go, and the `ask runs` table.
 * A status line is "ask REF · part · part", where REF is what `ask show` and `ask -c` take.
 */
import { tilde } from './home.js';
import { runOwner, taskRef } from './runs.js';

/* "41.2s", "4m 12s" or "1h 03m". */
export function duration(seconds) {
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const m = Math.floor(seconds / 60);
  return m < 60 ? `${m}m ${String(Math.round(seconds % 60)).padStart(2, '0')}s` : `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, '0')}m`;
}

/* "52.1k in · 2.0k out · $0.3100", or '' when nothing was reported. */
export function formatUsage(usage) {
  if (!usage || (!usage.input && !usage.output && typeof usage.cost !== 'number')) return '';
  const k = (n) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n || 0));
  const cost = typeof usage.cost === 'number' ? ` · $${usage.cost.toFixed(usage.cost >= 1 ? 2 : 4)}` : '';
  return `${k(usage.input)} in · ${k(usage.output)} out${cost}`;
}

/* Sums usage; cost stays absent until some part reports one. */
export function addUsage(total, part) {
  for (const key of ['input', 'output', 'cached']) total[key] = (total[key] || 0) + (part[key] || 0);
  if (typeof part.cost === 'number') total.cost = (total.cost || 0) + part.cost;
  return total;
}

const line = (ref, parts) => `ask ${[ref, ...parts.filter(Boolean)].join(' · ')}`;
const plural = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;

/* The line a task prints as it starts: what was asked for, how, and where. */
export function startLine(run, i, { dir, worktree }) {
  const task = run.tasks[i];
  const where = worktree ? `worktree ${tilde(worktree.path)}` : tilde(dir);
  const lines = [line(taskRef(run, i), [task.model, task.write ? 'write' : 'read', where, task.continues && `continues ${task.continues}`, 'started'])];
  if (worktree?.dirty) lines.push(line(taskRef(run, i), [`the worktree starts from HEAD; uncommitted changes in ${tilde(task.dir)} are not in it`]));
  return lines.join('\n');
}

/* "3 files changed, 1 commit", or '' for a run that changed nothing or was not in a repository. */
function changeSummary(result) {
  if (!result.changes) return '';
  const parts = [result.changes.length && `${plural(result.changes.length, 'file')} changed`, result.commits && plural(result.commits, 'commit')];
  return parts.filter(Boolean).join(', ') || 'no changes';
}

/* The line a task prints when it ends: the model that ran, the outcome, and what it cost and changed. */
export function doneLine(result) {
  return line(result.run, [
    result.name,
    result.ok ? 'ok' : 'failed',
    duration(result.seconds),
    changeSummary(result),
    result.worktree && `branch ${result.worktree.branch}`,
    formatUsage(result.usage),
    result.ok ? result.note : result.error,
  ]);
}

/* The lines that open and close a batch. */
export const batchStart = (run, jobs, todo) =>
  line(run.id, [`batch of ${run.tasks.length}`, todo < run.tasks.length && `resuming ${todo} unfinished`, `${Math.min(jobs, todo)} at a time`]);
export function batchEnd(run, seconds) {
  const total = run.results.reduce((sum, r) => (r?.usage ? addUsage(sum, r.usage) : sum), {});
  return line(run.id, [`${run.results.filter((r) => r?.ok).length}/${run.tasks.length} ok`, duration(seconds), formatUsage(total)]);
}

/* "14:02" today, else "Sep 30 14:02", from a run's yyyymmddThhmmss[mmm] UTC stamp. */
function when(stamp) {
  const m = /^(\d{4})(\d\d)(\d\d)T(\d\d)(\d\d)(\d\d)\d*$/.exec(stamp);
  if (!m) return '';
  const date = new Date(Date.UTC(m[1], m[2] - 1, m[3], m[4], m[5], m[6]));
  const time = date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false });
  return date.toDateString() === new Date().toDateString() ? time : `${date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })} ${time}`;
}

/* The `ask runs` table: one row per run, aligned, the last column cut to the terminal's width. */
export function runsTable(runs) {
  const rows = runs.map((run) => {
    const done = run.results.filter(Boolean);
    const ok = done.filter((r) => r.ok).length;
    const running = runOwner(run);
    const single = run.tasks.length === 1;
    const status = running
      ? 'running'
      : done.length < run.tasks.length
        ? 'stopped'
        : single
          ? done[0].ok ? 'ok' : 'failed'
          : `${ok}/${run.tasks.length} ok`;
    const first = run.tasks[0];
    const model = single ? done[0]?.name || first.model : `${run.tasks.length} tasks`;
    const time = single && done[0] ? duration(done[0].seconds) : '';
    const about =
      status === 'stopped'
        ? `resume: ask batch --resume ${run.id}`
        : `${first.continues ? `↪ ${first.continues} ` : ''}${first.prompt.split('\n')[0]}`;
    return [run.id, when(run.created), status, model, time, about];
  });
  const head = ['RUN', 'STARTED', 'STATUS', 'MODEL', 'TIME', 'TASK'];
  const widths = head.map((h, c) => Math.max(h.length, ...rows.map((r) => r[c].length)));
  const room = Math.max(20, (process.stdout.columns || 120) - widths.slice(0, 5).reduce((a, b) => a + b + 2, 0));
  const fit = (text) => (text.length > room ? `${text.slice(0, room - 1)}…` : text);
  return [head, ...rows].map((r) => [...r.slice(0, 5).map((cell, c) => cell.padEnd(widths[c])), fit(r[5])].join('  ').trimEnd()).join('\n');
}
