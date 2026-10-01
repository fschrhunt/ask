/*
 * One task, start to finish: the prompt ask sends, the harness run, and the checks on the answer.
 * A task is { id, prompt, model, write, json, schema, dir, timeout }.
 */
import { modelName, parseModel, runHarness } from './harness.js';

// Delegated read runs answer from memory unless told otherwise; measured with Codex at #low,
// which named a nonexistent function in 3 s without reading anything.
const GROUNDING =
  'Answer from the files in your working directory: search and read them before answering. ' +
  'Never guess file names, functions or facts; if you cannot find something, say so.\n\n';

/* Removes a Markdown code fence around a whole answer, which some models add despite instructions. */
function unfence(text) {
  const match = /^\s*```[a-z]*\s*\n([\s\S]*?)\n\s*```\s*$/i.exec(text);
  return (match ? match[1] : text).trim();
}

/*
 * Checks a parsed answer against the JSON Schema keywords ask relies on: type, enum, properties,
 * required, additionalProperties false and items. Returns the first mismatch as "path: problem",
 * or '' when the answer matches. Other keywords are not checked.
 */
export function schemaMismatch(value, schema, path = '$') {
  if (!schema || typeof schema !== 'object') return '';
  const kind = value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value;
  const fits = (type) => type === kind || (type === 'integer' && Number.isInteger(value)) || (type === 'number' && kind === 'number');
  const types = [schema.type].flat().filter(Boolean);
  if (types.length && !types.some(fits)) return `${path}: expected ${types.join(' or ')}, got ${kind}`;
  if (schema.enum && !schema.enum.some((option) => JSON.stringify(option) === JSON.stringify(value)))
    return `${path}: not one of ${JSON.stringify(schema.enum)}`;
  if (kind === 'object') {
    for (const key of schema.required || []) if (!(key in value)) return `${path}: missing "${key}"`;
    for (const [key, item] of Object.entries(value)) {
      if (schema.properties && key in schema.properties) {
        const problem = schemaMismatch(item, schema.properties[key], `${path}.${key}`);
        if (problem) return problem;
      } else if (schema.additionalProperties === false) return `${path}: unexpected "${key}"`;
    }
  }
  if (kind === 'array' && schema.items)
    for (const [index, item] of value.entries()) {
      const problem = schemaMismatch(item, schema.items, `${path}[${index}]`);
      if (problem) return problem;
    }
  return '';
}

/*
 * Runs one task on its model. Read runs get the grounding preamble; --json and --schema runs are told
 * the answer format in the prompt (a harness may also enforce ASK_SCHEMA natively), and the answer
 * must parse and match. Resolves with { id, model, name, ok, answer | error, seconds, usage, note };
 * never rejects.
 */
export async function runTask(task) {
  const m = parseModel(task.model);
  const json = task.json || Boolean(task.schema);
  let prompt = task.write ? task.prompt : GROUNDING + task.prompt;
  if (json)
    prompt += task.schema
      ? `\n\nAnswer ONLY with JSON matching this JSON Schema, no prose and no code fences:\n${JSON.stringify(task.schema)}`
      : '\n\nAnswer ONLY with JSON, no prose and no code fences.';
  const started = Date.now();
  const r = await runHarness(m, prompt, { write: task.write, schema: task.schema, dir: task.dir, timeoutMs: 1000 * (task.timeout || 900) });
  let answer = r.ok ? r.text.trimEnd() : undefined;
  if (r.ok && json) {
    try {
      answer = JSON.parse(unfence(r.text));
    } catch {
      r.ok = false;
      r.note = 'answer was not valid JSON';
    }
    const problem = r.ok && schemaMismatch(answer, task.schema);
    if (problem) {
      r.ok = false;
      r.note = `answer does not match the schema: ${problem}`;
    }
  }
  const seconds = Number(((Date.now() - started) / 1000).toFixed(1));
  const name = await modelName(m, r.name);
  return r.ok
    ? { id: task.id, model: m.spec, name, ok: true, answer, seconds, usage: r.usage, note: r.note }
    : { id: task.id, model: m.spec, name, ok: false, error: r.note, seconds, usage: r.usage };
}
