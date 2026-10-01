/*
 * Harnesses are executables that ask runs to reach an agent CLI; see docs/harnesses.md for the
 * contract. A harness named NAME is ~/.ask/harnesses/NAME. They are local only: ask ships none and
 * knows nothing about any particular agent; everything specific to one lives in its harness.
 */
import { accessSync, constants, mkdtempSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { LOCAL_HARNESSES, UsageError } from './home.js';
import { run } from './process.js';

const NAME = /^[a-z0-9][a-z0-9_-]*$/;
// Seconds a harness gets to list its models.
const LIST_TIMEOUT_MS = 10_000;

const executable = (path) => {
  try {
    accessSync(path, constants.X_OK);
    return statSync(path).isFile();
  } catch {
    return false;
  }
};
/* Every harness installed here, sorted by name. */
export function harnessNames() {
  try {
    return readdirSync(LOCAL_HARNESSES).filter((name) => NAME.test(name) && executable(join(LOCAL_HARNESSES, name))).sort();
  } catch {
    return [];
  }
}

/* The executable for a harness, or null when it is not installed. */
export function harnessPath(name) {
  return NAME.test(name) && executable(join(LOCAL_HARNESSES, name)) ? join(LOCAL_HARNESSES, name) : null;
}

/* Splits "harness:id#effort". A malformed spec or a harness that is not installed is a usage error. */
export function parseModel(spec) {
  const match = /^([a-z0-9][a-z0-9_-]*):([^#\s]+)(?:#(\S+))?$/.exec(spec || '');
  if (!match) throw new UsageError(`bad model "${spec}": expected harness:id[#effort]; see \`ask models\``);
  if (!harnessPath(match[1]))
    throw new UsageError(`no harness "${match[1]}" in ${LOCAL_HARNESSES}; installed: ${harnessNames().join(', ') || 'none'}; see docs/harnesses.md`);
  return { spec, harness: match[1], model: match[2], effort: match[3] };
}

const listed = new Map();

/*
 * The models a harness offers, from `<harness> models` (one "id" or "id<TAB>name" per line) plus
 * its ids in models.json, as [{ id, name }]. Cached per harness; a harness that fails to list offers
 * only its models.json ids.
 */
export async function harnessModels(harness, config = {}) {
  if (!listed.has(harness)) {
    const r = await run(harnessPath(harness), ['models'], { timeoutMs: LIST_TIMEOUT_MS });
    const own = r.code === 0 ? r.stdout.split('\n').filter((line) => line.trim()).map((line) => line.split('\t')) : [];
    listed.set(harness, own.map(([id, name]) => ({ id: id.trim(), name: name?.trim() || undefined })));
  }
  const extra = (config[harness] || []).map((id) => ({ id }));
  return [...listed.get(harness), ...extra];
}

/* Every "harness:id" offered here, in harness order. */
export async function listModels(config) {
  const all = await Promise.all(harnessNames().map(async (harness) => (await harnessModels(harness, config)).map((m) => `${harness}:${m.id}`)));
  return all.flat();
}

/* An id in title case, for a harness that names neither the model it ran nor its models. */
const titleCase = (id) => id.split('/').pop().split('-').map((w) => w[0].toUpperCase() + w.slice(1)).join(' ');

/*
 * The model's own name for people reading status lines: what the harness reported it ran, else the
 * name it lists for the id, else the id in title case. Effort is appended, e.g. "GPT-6.1 Sol (high)".
 */
export async function modelName(m, reported) {
  const name = reported || (await harnessModels(m.harness)).find((x) => x.id === m.model)?.name || titleCase(m.model);
  return m.effort ? `${name} (${m.effort})` : name;
}

/* The last line a failed harness wrote to stderr, which the contract makes its reason. */
const reason = (stderr) => stderr.trim().split('\n').filter((line) => line.trim()).pop()?.trim().slice(0, 300);

/*
 * Runs one prompt through a harness. Resolves with { ok, text, note, usage, name }: text is the
 * harness's stdout, and note, usage and name come from the report it may write. Never rejects.
 */
export async function runHarness(m, prompt, { write, schema, dir, timeoutMs }) {
  const work = mkdtempSync(join(tmpdir(), 'ask-'));
  try {
    const env = { ASK_MODEL: m.model, ASK_EFFORT: m.effort || '', ASK_ACCESS: write ? 'write' : 'read', ASK_REPORT: join(work, 'report.json') };
    if (schema) writeFileSync((env.ASK_SCHEMA = join(work, 'schema.json')), JSON.stringify(schema));
    const r = await run(harnessPath(m.harness), [], { input: prompt, cwd: dir, env, timeoutMs });
    let report = {};
    try {
      report = JSON.parse(readFileSync(env.ASK_REPORT, 'utf8'));
    } catch {}
    const { name, note = '' } = report;
    const counts = ['input', 'output', 'cached', 'cost'].filter((key) => typeof report[key] === 'number');
    const usage = counts.length ? Object.fromEntries(counts.map((key) => [key, report[key]])) : null;
    if (r.timedOut) return { ok: false, note: 'timed out', usage, name };
    if (r.code !== 0) return { ok: false, note: reason(r.stderr) || `exit ${r.code}`, usage, name };
    if (!r.stdout.trim()) return { ok: false, note: 'no answer', usage, name };
    return { ok: true, text: r.stdout, note, usage, name };
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
}
