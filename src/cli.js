/*
 * The ask command line: parses arguments and runs a command.
 *
 * Contract: stdout carries only answers (JSON with --json/--schema, and always for `batch`);
 * stderr carries one status line per run. Exit 0 on success, 1 when a run failed, 2 on usage
 * errors. Runs are read-only (-r) unless -w allows writing. Every run names its model as
 * harness:id[#effort]; like a native subagent, ask does not choose one for the caller.
 */
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { listRuns, newRunDir, parseTasks, recordedTasks, runBatch } from './batch.js';
import { listModels, parseModel } from './harness.js';
import { readModels, UsageError } from './home.js';
import { runTask } from './task.js';
import { formatUsage } from './usage.js';

const HELP = `ask: hand tasks to coding agents (Claude Code, Codex, Opencode, or any harness you add)
and get their answers back.

  ask -m MODEL [options] PROMPT   one prompt (use - or omit PROMPT to read stdin)
  ask models                      list the model ids available here
  ask batch [-j N] [-m MODEL] FILE|-
                                  run tasks in parallel (default 4 at once); prints a JSON
                                  array of results and records the run
  ask batch --resume DIR          rerun only the tasks of a recorded run that did not finish
  ask runs                        list recent recorded runs

options
  -m, --model ID     harness:id[#effort] from \`ask models\`, e.g. claude:sonnet#high
  -r, --read         read only (the default)
  -w, --write        read and write: may edit files and run commands
  --json             answer must be JSON
  --schema FILE      answer must match this JSON Schema (implies --json)
  -C, --dir DIR      directory the model works in (default: current)
  -t, --timeout S    seconds per run (default: 900)

batch tasks: a JSON array or one JSON object per line, each
  {"id": "...", "prompt": "...", "model": "...", "write": false, "schema": {...},
   "json": false, "dir": "...", "timeout": 900}
"prompt" is required, and "model" unless batch -m gives one. Results keep task order:
  [{"id", "model", "name", "ok", "answer", "error", "seconds", "usage"}]
name is the model's own name (Opus 5.5, GPT-6.1 Sol), for status lines and people.
usage is {input, output, cached} tokens, plus cost in USD where the harness reports it.
Status lines go to stderr; stopping ask stops every agent it started.

ask keeps its state in ~/.ask, or $ASK_HOME: harnesses/ (your own harnesses, which win over the
shipped ones), models.json (extra model ids per harness) and runs/ (recorded batches).
Docs: https://github.com/fschrhunt/ask/tree/main/docs`;

/* Parses the flags every command shares; leftovers are the prompt or the batch file. */
function parseFlags(argv) {
  const task = { dir: process.cwd(), write: false, json: false };
  const access = new Set();
  const words = [];
  const value = (i) => {
    if (argv[i + 1] === undefined) throw new UsageError(`missing value for ${argv[i]}`);
    return argv[i + 1];
  };
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === '-m' || arg === '--model') task.model = value(i++);
    else if (arg === '-r' || arg === '--read') access.add('read');
    else if (arg === '-w' || arg === '--write') access.add('write');
    else if (arg === '--json') task.json = true;
    else if (arg === '--schema') task.schema = JSON.parse(readFileSync(value(i++), 'utf8'));
    else if (arg === '-C' || arg === '--dir') task.dir = resolve(value(i++));
    else if (arg === '-t' || arg === '--timeout') task.timeout = Number(value(i++));
    else if (arg === '-j') task.jobs = Number(value(i++));
    else if (arg === '--resume') task.resume = value(i++);
    else if (arg === '-h' || arg === '--help') task.help = true;
    else if (arg.startsWith('-') && arg !== '-') throw new UsageError(`unknown option ${arg}`);
    else words.push(arg);
  }
  if (access.size > 1) throw new UsageError('choose one of -r (read) or -w (read and write)');
  task.write = access.has('write');
  return { task, words };
}

/* Checks a model spec; a missing one is a usage error that lists the choices, so the caller can pick one and retry. */
async function modelChecker(config) {
  const available = await listModels(config);
  return (model, where) => {
    if (!model) throw new UsageError(`${where} needs a model (-m); available:\n  ${available.join('\n  ')}`);
    parseModel(model);
  };
}

export async function main(argv) {
  const sub = ['models', 'batch', 'runs'].includes(argv[0]) ? argv[0] : null;
  const { task, words } = parseFlags(sub ? argv.slice(1) : argv);
  if (task.help || argv.length === 0) return console.log(HELP);
  const config = readModels();

  if (sub === 'models') {
    for (const id of await listModels(config)) console.log(id);
    return;
  }

  if (sub === 'runs') return listRuns();

  if (sub === 'batch') {
    const { jobs = 4, resume, ...defaults } = task;
    let tasks;
    let dir;
    if (resume) {
      dir = resume;
      tasks = recordedTasks(dir);
    } else {
      const source = words[0] && words[0] !== '-' ? readFileSync(words[0], 'utf8') : readFileSync(0, 'utf8');
      const check = await modelChecker(config);
      tasks = parseTasks(source, defaults, check);
      dir = newRunDir();
    }
    const results = await runBatch(tasks, jobs, dir);
    process.stdout.write(JSON.stringify(results, null, 2) + '\n');
    if (results.some((r) => !r.ok)) process.exitCode = 1;
    return;
  }

  let prompt = words.join(' ');
  if (!prompt || prompt === '-') prompt = readFileSync(0, 'utf8');
  if (!prompt.trim()) throw new UsageError('empty prompt');
  if (!task.model) (await modelChecker(config))(task.model, 'ask');
  parseModel(task.model);
  const r = await runTask({ ...task, prompt });
  if (!r.ok) {
    console.error(`ask: ${r.name} failed after ${r.seconds}s${formatUsage(r.usage)}: ${r.error}`);
    process.exitCode = 1;
    return;
  }
  console.error(`ask: ${r.name} ${r.seconds}s${formatUsage(r.usage)}${r.note ? `; ${r.note}` : ''}`);
  process.stdout.write((task.json || task.schema ? JSON.stringify(r.answer) : r.answer) + '\n');
}

/* Runs main, printing an error as one `ask:` line; usage errors exit 2 and point to --help. */
export function cli(argv) {
  main(argv).catch((error) => {
    console.error(`ask: ${error.message}${error instanceof UsageError ? '\nrun `ask --help`' : ''}`);
    process.exitCode = 2;
  });
}
