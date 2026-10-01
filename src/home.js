/*
 * Where ask keeps its local state: ~/.ask, or $ASK_HOME. It holds harnesses/ (the executables that
 * reach each agent), models.json (extra model ids per harness), runs/ (every run, for show, -c and
 * --resume) and worktrees/ (the git worktrees of --worktree runs).
 */
import { readFileSync, renameSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

export const HOME = process.env.ASK_HOME || join(homedir(), '.ask');
export const RUNS = join(HOME, 'runs');
export const WORKTREES = join(HOME, 'worktrees');
export const LOCAL_HARNESSES = join(HOME, 'harnesses');
const MODELS = join(HOME, 'models.json');

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
 * models.json is optional: { "<harness>": ["model-id", ...] } adds ids to what each harness lists
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
    throw new UsageError(`${MODELS} must map harness names to lists of model ids, like {"mycli": ["atlas-2.1"]}`);
  return models;
}
