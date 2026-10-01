/*
 * Where ask keeps its local state: ~/.ask, or $ASK_HOME. It holds agents/ (one executable per coding
 * agent), hooks/ and commands/ (see find.js), packages/ (installed packages of those), models.json
 * (extra model ids per agent), runs/ (every run, for show, -c and --resume) and worktrees/ (the git
 * worktrees of --worktree runs).
 */
import { readFileSync, renameSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

export const HOME = process.env.ASK_HOME || join(homedir(), '.ask');
export const RUNS = join(HOME, 'runs');
export const WORKTREES = join(HOME, 'worktrees');
export const AGENTS = join(HOME, 'agents');
export const PACKAGES = join(HOME, 'packages');
const MODELS = join(HOME, 'models.json');

// The version of the contracts ask keeps with agents, hooks and commands; see docs/compatibility.md.
export const CONTRACT = '1';
// This ask's own executable, for hooks and commands that run ask themselves.
const BIN = join(dirname(fileURLToPath(import.meta.url)), '..', 'bin', 'ask');

/*
 * The environment for an agent, hook or command: ask's own, plus ASK_CONTRACT, ASK_BIN and ASK_HOME,
 * plus `vars`. Per-run variables inherited from an ask further up (an agent that itself runs ask)
 * are dropped first, so they never leak into this run.
 */
export function contractEnv(vars) {
  const env = { ...process.env };
  for (const key of ['ASK_MODEL', 'ASK_EFFORT', 'ASK_ACCESS', 'ASK_SCHEMA', 'ASK_SESSION', 'ASK_REPORT', 'ASK_RUN', 'ASK_EVENT']) delete env[key];
  return Object.assign(env, { ASK_CONTRACT: CONTRACT, ASK_BIN: BIN, ASK_HOME: HOME }, vars);
}

/* A mistake in how ask was called; main prints it with a pointer to --help and exits 2. */
export class UsageError extends Error {}

/* Writes JSON so a reader never sees half a file: a temporary file, then a rename over the old one. */
export function writeJson(path, value) {
  writeFileSync(`${path}.tmp`, JSON.stringify(value, null, 2) + '\n');
  renameSync(`${path}.tmp`, path);
}

/* Reads a file named on the command line (- is stdin); a missing or unreadable one is a usage error. */
export function readInput(path, what) {
  try {
    return readFileSync(path === '-' ? 0 : path, 'utf8');
  } catch (error) {
    throw new UsageError(`cannot read ${what} ${path}: ${error.code === 'ENOENT' ? 'no such file' : error.message}`);
  }
}

/* A path as people read it, with the home folder as ~. */
export const tilde = (path) => (path && path.startsWith(homedir()) ? `~${path.slice(homedir().length)}` : path);

/*
 * models.json is optional: { "<agent>": ["model-id", ...] } adds ids to what each agent lists
 * itself. A file that exists must parse and have that shape.
 */
export function readModels() {
  let text;
  try {
    text = readFileSync(MODELS, 'utf8');
  } catch (error) {
    if (error.code === 'ENOENT') return {};
    throw new UsageError(`cannot read ${MODELS}: ${error.message}`);
  }
  let models;
  try {
    models = JSON.parse(text);
  } catch (error) {
    throw new UsageError(`cannot parse ${MODELS}: ${error.message}`);
  }
  const lists = models && typeof models === 'object' && !Array.isArray(models) ? Object.values(models) : null;
  if (!lists?.every((ids) => Array.isArray(ids) && ids.every((id) => typeof id === 'string')))
    throw new UsageError(`${MODELS} must map agent names to lists of model ids, like {"mycli": ["atlas-2.1"]}`);
  return models;
}
