/*
 * Agents are executables that each run one coding agent's CLI for ask; see docs/agents.md for the
 * contract. An agent named NAME is ~/.ask/agents/NAME, or a package's (see find.js). They are local
 * only: ask ships none and knows nothing about any coding agent; everything specific to one lives
 * in its agent.
 */
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { find } from './find.js';
import { AGENTS, contractEnv, UsageError } from './home.js';
import { reason, run } from './process.js';

// Milliseconds an agent gets to list its models.
const LIST_TIMEOUT_MS = 10_000;

/* Every agent installed here, sorted by name. */
export const agentNames = () => [...find('agents').keys()].sort();

/* The executable for an agent, or null when it is not installed. */
export const agentPath = (name) => find('agents').get(name) || null;

/* Splits "agent:id#effort". A malformed spec or an agent that is not installed is a usage error. */
export function parseModel(spec) {
  const match = /^([a-z0-9][a-z0-9_-]*):([^#\s]+)(?:#(\S+))?$/.exec(spec || '');
  if (!match) throw new UsageError(`bad model "${spec}": expected agent:id[#effort]; see \`ask models\``);
  if (!agentPath(match[1]))
    throw new UsageError(`no agent "${match[1]}" in ${AGENTS}; installed: ${agentNames().join(', ') || 'none'}; see docs/agents.md`);
  return { spec, agent: match[1], model: match[2], effort: match[3] };
}

const listed = new Map();

/*
 * The models an agent offers, from `<agent> models` (one "id" or "id<TAB>name" per line) plus
 * its ids in models.json, as { models: [{ id, name }], error }. The agent's own list is cached;
 * an agent that fails to list offers only its models.json ids, and error says why.
 */
export async function agentModels(agent, config = {}) {
  if (!listed.has(agent)) {
    const r = await run(agentPath(agent), ['models'], { env: contractEnv({}), timeoutMs: LIST_TIMEOUT_MS });
    const lines = r.code === 0 ? r.stdout.split('\n').filter((line) => line.trim()) : [];
    const error = r.code === 0 ? '' : reason(r.stderr) || (r.timedOut ? 'timed out' : `exit ${r.code}`);
    listed.set(agent, { own: lines.map((line) => line.split('\t')).map(([id, name]) => ({ id: id.trim(), name: name?.trim() || undefined })), error });
  }
  const { own, error } = listed.get(agent);
  return { models: [...own, ...(config[agent] || []).map((id) => ({ id }))], error };
}

/* Every "agent:id" offered here, in agent order, and the listing failures as "agent: reason". */
export async function listModels(config) {
  const all = await Promise.all(agentNames().map(async (agent) => ({ agent, ...(await agentModels(agent, config)) })));
  return {
    ids: all.flatMap(({ agent, models }) => models.map((m) => `${agent}:${m.id}`)),
    errors: all.filter((x) => x.error).map((x) => `${x.agent}: ${x.error}`),
  };
}

/* An id in title case (provider/atlas-2.1-mini -> Atlas 2.1 Mini), for an agent that names no model. */
const titleCase = (id) => id.split('/').filter(Boolean).pop()?.split('-').filter(Boolean).map((w) => w[0].toUpperCase() + w.slice(1)).join(' ') || id;

/*
 * The model's own name for people reading status lines: what the agent reported it ran, else the
 * name it lists for the id, else the id in title case. Effort is appended, e.g. "GPT-6.1 Sol (high)".
 */
export async function modelName(m, reported) {
  const name = reported || (await agentModels(m.agent)).models.find((x) => x.id === m.model)?.name || titleCase(m.model);
  return m.effort ? `${name} (${m.effort})` : name;
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
 * Runs one prompt through an agent, continuing `session` when given. Resolves with { ok, text,
 * note, usage, name, session }: text is the agent's stdout; the rest comes from its report, which
 * is read even after a failure or timeout so the session can still be continued. Never rejects.
 */
export async function runAgent(m, prompt, { write, schema, session, dir, timeoutMs }) {
  const work = mkdtempSync(join(tmpdir(), 'ask-'));
  try {
    const vars = { ASK_MODEL: m.model, ASK_EFFORT: m.effort || '', ASK_ACCESS: write ? 'write' : 'read', ASK_REPORT: join(work, 'report.json') };
    if (schema) writeFileSync((vars.ASK_SCHEMA = join(work, 'schema.json')), JSON.stringify(schema));
    if (session) vars.ASK_SESSION = session;
    const r = await run(agentPath(m.agent), [], { input: prompt, cwd: dir, env: contractEnv(vars), timeoutMs });
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
