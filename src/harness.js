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
 * its ids in models.json, as { models: [{ id, name }], error }. The harness's own list is cached;
 * a harness that fails to list offers only its models.json ids, and error says why.
 */
export async function harnessModels(harness, config = {}) {
  if (!listed.has(harness)) {
    const r = await run(harnessPath(harness), ['models'], { env: contractEnv({}), timeoutMs: LIST_TIMEOUT_MS });
    const lines = r.code === 0 ? r.stdout.split('\n').filter((line) => line.trim()) : [];
    const error = r.code === 0 ? '' : reason(r.stderr) || (r.timedOut ? 'timed out' : `exit ${r.code}`);
    listed.set(harness, { own: lines.map((line) => line.split('\t')).map(([id, name]) => ({ id: id.trim(), name: name?.trim() || undefined })), error });
  }
  const { own, error } = listed.get(harness);
  return { models: [...own, ...(config[harness] || []).map((id) => ({ id }))], error };
}

/* Every "harness:id" offered here, in harness order, and the listing failures as "harness: reason". */
export async function listModels(config) {
  const all = await Promise.all(harnessNames().map(async (harness) => ({ harness, ...(await harnessModels(harness, config)) })));
  return {
    ids: all.flatMap(({ harness, models }) => models.map((m) => `${harness}:${m.id}`)),
    errors: all.filter((x) => x.error).map((x) => `${x.harness}: ${x.error}`),
  };
}

/* An id in title case (provider/fast-one -> Fast One), for a harness that names no model. */
const titleCase = (id) => id.split('/').filter(Boolean).pop()?.split('-').filter(Boolean).map((w) => w[0].toUpperCase() + w.slice(1)).join(' ') || id;

/*
 * The model's own name for people reading status lines: what the harness reported it ran, else the
 * name it lists for the id, else the id in title case. Effort is appended, e.g. "GPT-6.1 Sol (high)".
 */
export async function modelName(m, reported) {
  const name = reported || (await harnessModels(m.harness)).models.find((x) => x.id === m.model)?.name || titleCase(m.model);
  return m.effort ? `${name} (${m.effort})` : name;
}

/* The last line a failed harness wrote to stderr, which the contract makes its reason. */
const reason = (stderr) => stderr.trim().split('\n').filter((line) => line.trim()).pop()?.trim().slice(0, 300);

/*
 * ask's environment for a harness, with the contract's variables exactly as given: one inherited
 * from an ask further up (an agent that itself runs ask) must not leak into this run.
 */
function contractEnv(vars) {
  const env = { ...process.env };
  for (const key of ['ASK_MODEL', 'ASK_EFFORT', 'ASK_ACCESS', 'ASK_SCHEMA', 'ASK_SESSION', 'ASK_REPORT']) delete env[key];
  return Object.assign(env, vars);
}

/* The report's fields ask uses, keeping only well-formed ones: strings for name, note and session, counts as numbers. */
function readReport(path) {
  let report;
  try {
    report = JSON.parse(readFileSync(path, 'utf8'));
  } catch {
    return {};
  }
  if (!report || typeof report !== 'object') return {};
  const text = (key) => (typeof report[key] === 'string' && report[key].trim() ? report[key].trim() : undefined);
  const counts = ['input', 'output', 'cached', 'cost'].filter((key) => Number.isFinite(report[key]) && report[key] >= 0);
  return {
    name: text('name'),
    note: text('note') || '',
    session: text('session'),
    usage: counts.length ? Object.fromEntries(counts.map((key) => [key, report[key]])) : null,
  };
}

/*
 * Runs one prompt through a harness, continuing `session` when given. Resolves with { ok, text,
 * note, usage, name, session }: text is the harness's stdout; the rest comes from its report, which
 * is read even after a failure or timeout so the session can still be continued. Never rejects.
 */
export async function runHarness(m, prompt, { write, schema, session, dir, timeoutMs }) {
  const work = mkdtempSync(join(tmpdir(), 'ask-'));
  try {
    const vars = { ASK_MODEL: m.model, ASK_EFFORT: m.effort || '', ASK_ACCESS: write ? 'write' : 'read', ASK_REPORT: join(work, 'report.json') };
    if (schema) writeFileSync((vars.ASK_SCHEMA = join(work, 'schema.json')), JSON.stringify(schema));
    if (session) vars.ASK_SESSION = session;
    const r = await run(harnessPath(m.harness), [], { input: prompt, cwd: dir, env: contractEnv(vars), timeoutMs });
    const report = readReport(vars.ASK_REPORT);
    const meta = { usage: report.usage, name: report.name, session: report.session || session };
    if (r.timedOut) return { ok: false, note: 'timed out', ...meta };
    if (r.code !== 0) return { ok: false, note: reason(r.stderr) || `exit ${r.code}`, ...meta };
    if (!r.stdout.trim()) return { ok: false, note: 'no answer', ...meta };
    return { ok: true, text: r.stdout, note: report.note, ...meta };
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
}
