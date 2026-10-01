/*
 * One task, start to finish: where it runs (its directory, or a worktree), the prompt ask sends,
 * the harness run, what it changed, and the checks on the answer. A task is { id, prompt, model,
 * write, json, schema, dir, timeout, worktree, session }; worktree is a worktree name and session the
 * agent session a follow-up continues (see runs.js).
 */
import { addWorktree, changes, removeWorktree, snapshot } from './git.js';
import { modelName, parseModel, runHarness } from './harness.js';
import { schemaMismatch } from './schema.js';

// Delegated read runs answer from memory unless told otherwise; measured with one at low effort,
// which named a nonexistent function in 3 s without reading anything.
const GROUNDING =
  'Answer from the files in your working directory: search and read them before answering. ' +
  'Never guess file names, functions or facts; if you cannot find something, say so.\n\n';

/* Removes a Markdown code fence around a whole answer, which some models add despite instructions. */
function unfence(text) {
  const match = /^\s*```[a-z]*\s*\n([\s\S]*?)\n\s*```\s*$/i.exec(text);
  return (match ? match[1] : text).trim();
}

/* The prompt ask sends: a fresh read run is told to ground its answer, a JSON run the answer format. */
function fullPrompt(task) {
  let prompt = task.write || task.session ? task.prompt : GROUNDING + task.prompt;
  if (task.schema) prompt += `\n\nAnswer ONLY with JSON matching this JSON Schema, no prose and no code fences:\n${JSON.stringify(task.schema)}`;
  else if (task.json) prompt += '\n\nAnswer ONLY with JSON, no prose and no code fences.';
  return prompt;
}

/* Parses and checks a JSON answer; returns { answer } or { problem }. */
function checkJson(text, schema) {
  let answer;
  try {
    answer = JSON.parse(unfence(text));
  } catch {
    return { problem: 'answer was not valid JSON' };
  }
  const mismatch = schemaMismatch(answer, schema);
  return mismatch ? { problem: `answer does not match the schema: ${mismatch}` } : { answer };
}

/*
 * Runs one task. `started({ dir, worktree })` is called just before the agent starts. Resolves with
 * the result { id, model, name, ok, answer | error, note, seconds, usage, session, dir, changes,
 * commits, worktree }; never rejects: anything that goes wrong becomes the task's error.
 */
export async function runTask(task, started = () => {}) {
  const begin = Date.now();
  let m = { spec: task.model, model: task.model };
  let dir = task.dir;
  let worktree = null;
  let diff = null;
  let r = {};
  try {
    m = parseModel(task.model);
    if (task.worktree) {
      worktree = addWorktree(task.dir, task.worktree);
      dir = worktree.dir;
    }
    started({ dir, worktree });
    const before = task.write ? snapshot(dir) : null;
    r = await runHarness(m, fullPrompt(task), { write: task.write, schema: task.schema, session: task.session, dir, timeoutMs: 1000 * (task.timeout || 900) });
    if (before) diff = changes(before);
    if (r.ok && (task.json || task.schema)) {
      const { answer, problem } = checkJson(r.text, task.schema);
      Object.assign(r, problem ? { ok: false, note: problem } : { answer });
    } else if (r.ok) r.answer = r.text.trimEnd();
    if (worktree && !diff?.files.length && !diff?.commits) removeWorktree(worktree);
  } catch (error) {
    r = { ...r, ok: false, note: error.message };
  }
  const changed = diff && (diff.files.length || diff.commits);
  return {
    id: task.id,
    model: m.spec,
    name: m.harness ? await modelName(m, r.name).catch(() => m.spec) : m.spec,
    ok: r.ok,
    ...(r.ok ? { answer: r.answer } : { error: r.note }),
    ...(r.ok && r.note ? { note: r.note } : {}),
    seconds: Number(((Date.now() - begin) / 1000).toFixed(1)),
    usage: r.usage || null,
    session: r.session || null,
    dir,
    ...(diff && { changes: diff.files, commits: diff.commits }),
    ...(worktree && changed && { worktree: { path: worktree.path, branch: worktree.branch } }),
  };
}
