# Hooks

A hook is a small executable in `~/.ask/hooks/` that runs around every task: before it, to change
or refuse the task, and after it, to check the result. With hooks you can:

- add project context to every prompt, or route reads to a cheaper model;
- refuse write runs outside some folders;
- run the tests after a write run and, if they fail, have the same agent fix them;
- fail a result that doesn't meet your bar, or post it somewhere.

Hooks are yours: ask runs every hook in `~/.ask/hooks/` (and in your [packages](packages.md)) for
every run. `--no-hooks` turns them off for one run.

## The contract

A hook is any executable. ask calls it with one argument, the event:

| Call | stdin | What it may print (one JSON object, or nothing) |
| --- | --- | --- |
| `NAME events` | none | The events it handles, one per line: `task`, `result`, `title` |
| `NAME task` | `{"task": {...}}` | `{"task": {changes}}`, `{"refuse": "why"}`, `{"note": "text"}` |
| `NAME title` | `{"title": "...", "command": ["ask", "..."], "description": "..."}` | `{"title": "replacement"}` |
| `NAME result` | `{"task": {...}, "result": {...}}` | `{"followup": "prompt"}`, `{"fail": "why"}`, `{"note": "text"}` |

- **`task`** is the task about to run: `id`, `prompt`, `model`, `write`, `json`, `schema`, `dir`,
  `timeout`, and for a follow-up `continues`. A hook may change `prompt`, `model`, `write`, `json`,
  `timeout` and `schema`; ask ignores other changes, and says so.
- **`result`** is the task's result, as `ask show RUN --json` prints it.
- **`followup`** asks the same agent, in the same conversation and place, to keep working. Its new
  result goes through the result hooks again, up to three follow-ups per task. The task's final
  result combines every round: the last answer, with the time, usage and changes of all of them.
- A hook runs in the task's directory, with `ASK_RUN` (the run, like `k3f9a2/api`), `ASK_EVENT`,
  `ASK_BIN` (ask itself), `ASK_HOME` and `ASK_CONTRACT` (see [Compatibility](compatibility.md)).

Hooks run in name order; each sees the task as the previous one left it. For results, the first
hook to ask for a follow-up or to fail the result decides, and the rest wait for the next round.

**Hooks fail open.** A hook that cannot start, crashes, takes more than 10 minutes, or prints something that
isn't a JSON object changes nothing; ask notes it and carries on. Only an explicit `refuse` or
`fail` stops a task. Every note and follow-up shows as a status line:

```text
ask k3f9a2 · started · Sonnet 5.5 · write · ~/code/app
ask k3f9a2 · note · hook verify · 2 tests fail
ask k3f9a2 · follow-up · hook verify · These tests fail after your change: ...
ask k3f9a2 · ok · Sonnet 5.5 · 3:12 · 3 files changed · 140.2k in · 6.1k out · $0.62
```

## Example: verify write runs with the tests

```js
#!/usr/bin/env node
// ~/.ask/hooks/verify: after a write run, run the tests and hand failures back to the agent.
import { execSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

if (process.argv[2] === 'events') {
  console.log('result');
  process.exit(0);
}
const { task, result } = JSON.parse(readFileSync(0, 'utf8'));
if (!task.write || !result.ok) process.exit(0);
try {
  execSync('npm test', { stdio: 'pipe' });
} catch (error) {
  const output = String(error.stdout).slice(-4000);
  console.log(JSON.stringify({ note: 'tests fail', followup: `These tests fail after your change; fix them:\n${output}` }));
}
```

## Example: a guard in shell

```sh
#!/bin/sh
# ~/.ask/hooks/guard: refuse write runs outside ~/code.
[ "$1" = events ] && { echo task; exit 0; }
case "$PWD" in
  "$HOME"/code/*) ;;
  *) grep -q '"write":true' && echo '{"refuse": "write runs only under ~/code"}' ;;
esac
```

## Example: add context to every prompt

```sh
#!/bin/sh
# ~/.ask/hooks/context: put the project's CONTEXT.md before every prompt, when there is one.
[ "$1" = events ] && { echo task; exit 0; }
[ -f CONTEXT.md ] || exit 0
node -e '
  const { task } = JSON.parse(require("fs").readFileSync(0, "utf8"));
  const context = require("fs").readFileSync("CONTEXT.md", "utf8");
  console.log(JSON.stringify({ task: { prompt: context + "\n\n" + task.prompt } }));
'
```

ask never runs hooks from the project it works in, only yours and your packages'.

The `title` event runs only for [host titles](hosts.md). `command` is the literal ask argv,
without shell assignments or redirects; `description` is the host's original text. Hooks run
in the invocation's directory without a run id. An empty replacement keeps the current title.
