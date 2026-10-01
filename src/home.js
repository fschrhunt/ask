/*
 * Where ask keeps its local state: ~/.ask, or $ASK_HOME. It holds harnesses/ (local adapters, which
 * win over the shipped ones), models.json (extra model ids per harness) and runs/ (recorded batches).
 */
import { readFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

export const HOME = process.env.ASK_HOME || join(homedir(), '.ask');
export const RUNS = join(HOME, 'runs');
export const LOCAL_HARNESSES = join(HOME, 'harnesses');
const MODELS = join(HOME, 'models.json');

/* A mistake in how ask was called; main prints it with a pointer to --help and exits 2. */
export class UsageError extends Error {}

/*
 * models.json is optional: { "<harness>": ["model-id", ...] } adds ids to what each harness lists
 * itself. A file that exists must parse.
 */
export function readModels() {
  let text;
  try {
    text = readFileSync(MODELS, 'utf8');
  } catch (error) {
    if (error.code === 'ENOENT') return {};
    throw new UsageError(`cannot read ${MODELS}: ${error.message}`);
  }
  try {
    return JSON.parse(text);
  } catch (error) {
    throw new UsageError(`cannot parse ${MODELS}: ${error.message}`);
  }
}
