/*
 * The ask command line: parses arguments and runs a command.
 *
 * Contract: stdout carries only answers (JSON with --json/--schema, and always for `batch`);
 * stderr carries status lines, each starting "ask REF ·" where REF names the run for show, -c and
 * stop. Exit 0 on success, 1 when a run failed, 2 on usage errors. Runs are read-only (-r) unless -w
 * allows writing. Every run names its model as agent:id[#effort]; like a native subagent, ask
 * does not choose one for the caller.
 */
import { spawn } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { constants } from 'node:os';
import { listModels } from './agent.js';
import { find } from './find.js';
import { AGENTS, contractEnv, readInput, readModels, UsageError } from './home.js';
import { contents, install, installed, remove, update } from './packages.js';
import { createRun, executeRun, newRunId, openRun, prepareTasks, recentRuns, stopRun } from './runs.js';
import { batchEnd, batchStart, doneLine, hookLine, runsTable, startLine } from './status.js';

const HELP = `ask: hand tasks to coding agents and get their answers back.

  ask -m MODEL [options] PROMPT    run one task (PROMPT may be - or piped on stdin)
  ask -c RUN [options] PROMPT      continue that run's agent with a follow-up
  ask batch [options] FILE|-       run a batch of tasks in parallel; prints a JSON array
  ask batch --resume RUN [-j N]    rerun the tasks of a batch that did not finish ok
  ask show RUN [--json]            print a run's answer again (--json: its whole result)
  ask runs [-n N]                  list recent runs
  ask stop RUN                     stop a run that is going
  ask models                       list the model ids available here
  ask install [SOURCE]             install a package from git (OWNER/REPO, a URL or a path),
                                   or update every package
  ask packages                     list installed packages and what they offer
  ask remove PACKAGE               remove a package
  ask COMMAND [ARGS]               run one of your commands (~/.ask/commands)

options
  -m, --model ID      agent:id[#effort] from \`ask models\`, e.g. mycli:atlas-2.1#high
  -r, --read          read only (the default)
  -w, --write         read and write: may edit files and run commands
  --worktree          with -w: work in a new git worktree and branch, kept only if changed
  -c, --continue RUN  continue RUN's agent session, where it ran (RUN/TASK for a batch task)
  --json              the answer must be JSON
  --schema FILE       the answer must match this JSON Schema (implies --json)
  -C, --dir DIR       directory the agent works in (default: current)
  -t, --timeout S     seconds per task (default: 900)
  -j N                tasks at once in a batch (default: 4)
  --no-hooks          run without your hooks (~/.ask/hooks)

Every run gets an id, shown in its status lines as "ask RUN · ...". A batch task is RUN/TASK.
Batch tasks: a JSON array or one JSON object per line, each
  {"id", "prompt", "model", "write", "worktree", "continue", "json", "schema", "dir", "timeout"}
with "prompt" required, and "model" unless batch -m gives one or the task continues a run.
Results: {"run", "id", "model", "name", "ok", "answer" | "error", "seconds", "usage", "session",
"dir", "changes", "commits", "worktree"}; changes are the files a write run changed.

Make ask yours with executables in ~/.ask ($ASK_HOME), or from packages: agents/ run each coding
agent's CLI, hooks/ change tasks and check results, commands/ add commands. Runs are kept in
~/.ask/runs. Docs: https://github.com/fschrhunt/ask/tree/main/docs`;

// The options each command takes; anything else is a usage error.
const OPTIONS = {
  run: ['-m', '-r', '-w', '--worktree', '-c', '--json', '--schema', '-C', '-t', '--no-hooks'],
  batch: ['-m', '-r', '-w', '--worktree', '--json', '--schema', '-C', '-t', '-j', '--resume', '--no-hooks'],
  show: ['--json'],
  runs: ['-n'],
  stop: [],
  models: [],
  install: [],
  packages: [],
  remove: [],
};
const LONG = { '--model': '-m', '--read': '-r', '--write': '-w', '--continue': '-c', '--dir': '-C', '--timeout': '-t' };
const VALUE = new Set(['-m', '-c', '--schema', '-C', '-t', '-j', '-n', '--resume']);

/* A positive number (or whole number) from an option, or a usage error naming it. */
function number(flag, text, whole) {
  const n = Number(text);
  if (!(n > 0) || (whole && !Number.isInteger(n))) throw new UsageError(`${flag} needs a ${whole ? 'whole number' : 'number'} above 0, not "${text}"`);
  return n;
}

/* Parses a command's options; returns { opts, words }. Words after -- are taken as they are. */
function parse(command, argv) {
  const opts = {};
  const words = [];
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === '--') {
      words.push(...argv.slice(i + 1));
      break;
    }
    if (arg === '-h' || arg === '--help') opts.help = true;
    else if (arg.startsWith('-') && arg !== '-') {
      const flag = LONG[arg] || arg;
      if (!OPTIONS[command].includes(flag)) throw new UsageError(`${command === 'run' ? '' : `ask ${command}: `}unknown option ${arg}`);
      if (!VALUE.has(flag)) opts[flag] = true;
      else if (argv[i + 1] === undefined) throw new UsageError(`missing value for ${arg}`);
      else opts[flag] = argv[++i];
    } else words.push(arg);
  }
  if (opts['-r'] && opts['-w']) throw new UsageError('choose one of -r (read) or -w (read and write)');
  return { opts, words };
}

/* Task fields from the options a run and a batch share. */
function taskOptions(opts) {
  const task = {};
  if (opts['-m']) task.model = opts['-m'];
  if (opts['-r'] || opts['-w']) task.write = Boolean(opts['-w']);
  if (opts['--worktree']) task.worktree = true;
  if (opts['--json']) task.json = true;
  if (opts['--schema']) {
    const text = readInput(opts['--schema'], 'schema');
    try {
      task.schema = JSON.parse(text);
    } catch (error) {
      throw new UsageError(`cannot parse schema ${opts['--schema']}: ${error.message}`);
    }
  }
  if (opts['-C']) task.dir = opts['-C'];
  if (opts['-t']) task.timeout = number('-t', opts['-t']);
  return task;
}

/* Reads batch tasks: a JSON array, or one JSON object per line. */
function readTasks(text) {
  const trimmed = text.trim();
  if (!trimmed) throw new UsageError('no tasks: pass a JSON array or JSON lines in FILE or on stdin');
  const json = (source, where) => {
    try {
      return JSON.parse(source);
    } catch (error) {
      throw new UsageError(`cannot parse ${where}: ${error.message}`);
    }
  };
  const items = trimmed.startsWith('[') ? json(trimmed, 'the batch') : trimmed.split('\n').flatMap((l, i) => (l.trim() ? [json(l, `line ${i + 1}`)] : []));
  if (!Array.isArray(items) || !items.length) throw new UsageError('no tasks: the batch is empty');
  return items;
}

/* Prepares tasks, listing the available models in the error when one has no model. */
async function prepare(items, defaults, id, single) {
  const lacking = items.some((item) => !(item?.model || defaults.model || item?.continue));
  const { ids } = lacking ? await listModels(readModels()) : { ids: [] };
  return prepareTasks(items, defaults, id, single, (where) => {
    throw new UsageError(`${where} needs a model (-m); available:\n  ${ids.join('\n  ') || `none; add an agent to ${AGENTS}`}`);
  });
}

/* Prints each task's status lines on stderr: its start, hook notes and follow-ups, and its end. */
const reporter = (run) => (kind, i, x) =>
  console.error(kind === 'start' ? startLine(run, i, x) : kind === 'done' ? doneLine(x) : hookLine(run, i, kind, x));

/* What a finished task prints on stdout: its answer as text, or as JSON for a JSON task. */
const answerText = (task, result) => (task.json || task.schema ? JSON.stringify(result.answer) : result.answer);

async function runOne({ opts, words }) {
  let prompt = words.join(' ');
  if (!prompt || prompt === '-') {
    if (process.stdin.isTTY) throw new UsageError('no prompt: give it as an argument, or pipe it in');
    prompt = readInput('-', 'prompt');
  }
  const id = newRunId();
  const tasks = await prepare([{ ...taskOptions(opts), prompt, continue: opts['-c'] }], {}, id, true);
  const run = createRun(id, tasks);
  const [result] = await executeRun(run, 1, reporter(run), { hooks: !opts['--no-hooks'] });
  if (!result.ok) return (process.exitCode = 1);
  process.stdout.write(answerText(tasks[0], result) + '\n');
}

async function batch({ opts, words }) {
  const jobs = opts['-j'] ? number('-j', opts['-j'], true) : 4;
  let run;
  if (opts['--resume']) {
    const extra = [...Object.keys(opts).filter((flag) => !['--resume', '-j', '--no-hooks'].includes(flag)), ...words];
    if (extra.length) throw new UsageError(`--resume reruns a batch as it was recorded; it takes only -j and --no-hooks, not ${extra.join(' ')}`);
    run = openRun(opts['--resume']).run;
  } else {
    if (words.length > 1) throw new UsageError(`ask batch takes one file of tasks, not ${words.length}`);
    const id = newRunId();
    run = createRun(id, await prepare(readTasks(readInput(words[0] || '-', 'tasks')), taskOptions(opts), id, false));
  }
  const todo = run.tasks.filter((_, i) => !run.results[i]?.ok).length;
  const begin = Date.now();
  console.error(batchStart(run, jobs, todo));
  const results = await executeRun(run, jobs, reporter(run), { hooks: !opts['--no-hooks'] });
  console.error(batchEnd(run, (Date.now() - begin) / 1000));
  process.stdout.write(JSON.stringify(results, null, 2) + '\n');
  if (results.some((r) => !r?.ok)) process.exitCode = 1;
}

function show({ opts, words }) {
  if (words.length !== 1) throw new UsageError('ask show takes one run, like ask show k3f9a2');
  const { run, index } = openRun(words[0]);
  if (index === null) return process.stdout.write(JSON.stringify(run.results, null, 2) + '\n');
  const result = run.results[index];
  if (!result) throw new UsageError(`${words[0]} has not finished; see \`ask runs\``);
  console.error(doneLine(result));
  if (opts['--json']) process.stdout.write(JSON.stringify(result, null, 2) + '\n');
  else if (result.ok) process.stdout.write(answerText(run.tasks[index], result) + '\n');
  if (!result.ok) process.exitCode = 1;
}

async function stop({ words }) {
  if (words.length !== 1) throw new UsageError('ask stop takes one run, like ask stop k3f9a2');
  const { run } = openRun(words[0]);
  const stopped = await stopRun(run);
  console.error(`ask ${run.id} · ${stopped ? 'stopped' : 'not running'}`);
  if (!stopped) process.exitCode = 1;
}

function runs({ opts }) {
  const list = recentRuns(opts['-n'] ? number('-n', opts['-n'], true) : 20);
  if (!list.length) return console.error('ask: no runs yet');
  console.log(runsTable(list));
}

async function models() {
  const { ids, errors } = await listModels(readModels());
  for (const error of errors) console.error(`ask: could not list the models of ${error}`);
  if (!ids.length) console.error(`ask: no models; add an agent to ${AGENTS} (see docs/agents.md)`);
  for (const id of ids) console.log(id);
}

function installCommand({ words }) {
  if (words.length > 1) throw new UsageError('ask install takes one source, like ask install owner/repo');
  if (words.length) return console.error(`ask: ${install(words[0])}`);
  const all = installed();
  if (!all.length) return console.error('ask: no packages installed; ask install OWNER/REPO adds one');
  for (const { name, dir } of all) {
    try {
      console.error(`ask: ${update(dir)}`);
    } catch (error) {
      console.error(`ask: ${name}: could not update: ${error.message}`);
      process.exitCode = 1;
    }
  }
}

function packages() {
  const all = installed();
  if (!all.length) return console.error('ask: no packages installed; ask install OWNER/REPO adds one');
  for (const { name, dir } of all) console.log(`${name}\t${contents(dir)}`);
}

function removeCommand({ words }) {
  if (words.length !== 1) throw new UsageError('ask remove takes one package, like ask remove owner/repo');
  console.error(`ask: ${remove(words[0])}`);
}

/* The one-line description a command gives itself on a line containing "ask-command: TEXT" near its top. */
function describe(path) {
  try {
    return /ask-command:\s*(.+)/.exec(readFileSync(path, 'utf8').slice(0, 4096))?.[1].trim() || '';
  } catch {
    return '';
  }
}

/* The help text, with your commands listed after ask's own. */
function help() {
  const commands = [...find('commands')].sort(([a], [b]) => a.localeCompare(b));
  if (!commands.length) return HELP;
  const width = Math.max(...commands.map(([name]) => name.length));
  return `${HELP}\n\nyour commands\n${commands.map(([name, path]) => `  ask ${name.padEnd(width)}  ${describe(path)}`.trimEnd()).join('\n')}`;
}

/* Runs one of your commands with the rest of the arguments; ask exits with its exit code. */
function runCommand(path, args) {
  return new Promise((resolve) => {
    const child = spawn(path, args, { stdio: 'inherit', env: contractEnv({}) });
    child.on('error', (error) => {
      console.error(`ask: cannot run ${path}: ${error.message}`);
      resolve((process.exitCode = 1));
    });
    child.on('exit', (code, signal) => resolve((process.exitCode = code ?? 128 + (constants.signals[signal] || 0))));
  });
}

const COMMANDS = { batch, show, stop, runs, models, install: installCommand, packages, remove: removeCommand };

export async function main(argv) {
  const command = Object.hasOwn(COMMANDS, argv[0]) ? argv[0] : 'run';
  const yours = command === 'run' && argv[0] && !argv[0].startsWith('-') && find('commands').get(argv[0]);
  if (yours) return runCommand(yours, argv.slice(1));
  const parsed = parse(command, command === 'run' ? argv : argv.slice(1));
  if (parsed.opts.help || argv.length === 0) return console.log(help());
  await (COMMANDS[command] || runOne)(parsed);
}

/* Runs main, printing an error as one `ask:` line: usage errors exit 2 and point to --help, others exit 1. */
export function cli(argv) {
  main(argv).catch((error) => {
    const usage = error instanceof UsageError;
    console.error(`ask: ${error.message}${usage ? '\nrun `ask --help`' : ''}`);
    process.exitCode = usage ? 2 : 1;
  });
}
