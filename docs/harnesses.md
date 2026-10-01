# Harnesses

A harness is a small program that ask runs to reach one agent CLI. ask knows nothing about any
particular agent: everything specific to one, from its flags to how it reports usage, lives in its
harness. That is how ask supports any agent.

Harnesses are local. ask ships none; you keep yours in `~/.ask/harnesses/`, one executable per
agent, named for it. A harness named `mycli` gives you models `mycli:...`.

## The contract

A harness is any executable: a shell script, Node, Python, a binary.

### Listing models: `NAME models`

Print one model per line, as `id` or `id<TAB>name`. The name is what status lines show.

```text
fast	My Fast
smart	My Smart
```

Print nothing if the CLI has too many models to list; users add the ones they use to
`~/.ask/models.json` (see [Models](models.md)).

### Running: `NAME`

ask runs the harness with no arguments, in the directory the agent should work in (`-C`).

| Input | |
| --- | --- |
| stdin | The prompt, complete. ask has already added any read-run or JSON instructions. |
| `ASK_MODEL` | The model id, without the harness or effort: `smart`. |
| `ASK_EFFORT` | The effort from `#effort`, or empty. |
| `ASK_ACCESS` | `read` or `write`. |
| `ASK_SCHEMA` | Set only with `--schema`: a file holding the JSON Schema, for CLIs that enforce one. |
| `ASK_SESSION` | Set only for a follow-up: the session to continue (see [Sessions](#sessions)). |
| `ASK_REPORT` | A file path for the optional report. |

| Output | |
| --- | --- |
| stdout | The answer, and nothing else. |
| exit code | 0 when the answer is good, anything else when the run failed. |
| stderr | On failure, the reason as the last line. ask shows that line. |
| `$ASK_REPORT` | Optional JSON: `{"session", "name", "input", "output", "cached", "cost", "note"}`. |

In the report, `session` is the agent session the run used, `name` is the model that actually ran
(`My Smart 2`), the counts are tokens, `cost` is in USD, and `note` is a short remark ask adds to
the status line (`hit step cap; answer may be partial`). Every field is optional. ask reads the
report even when the run fails or times out, so write it as early as you know something, above all
the session, and rewrite it whole as you learn more.

ask handles everything else: timeouts, stopping, batches, recording runs, follow-ups, worktrees,
reporting what changed and checking JSON answers.
It runs each harness in its own process group, so stopping a run also stops the CLI your harness
started. Don't detach the CLI from that group.

## Sessions

Follow-ups (`ask -c RUN`) continue the agent's own conversation, so they need the CLI's session
id. A harness that supports them:

1. reports the session in `$ASK_REPORT` as `"session"`, as soon as the CLI says what it is;
2. when `ASK_SESSION` is set, resumes that session with the prompt instead of starting a new one.

ask runs a follow-up in the same directory as the run it continues, which most CLIs need to find
the session. A harness that reports no session simply can't be continued; ask says so.

## Read-only

ask passes `ASK_ACCESS=read` for read runs and trusts the harness to keep it. How depends on the
CLI:

- **An OS sandbox** is the strongest: the agent may run any command, but nothing can write.
- **Tool and command rules**: allow only reading tools and a short list of inspection commands,
  and refuse anything with redirects, pipes, chaining, quoting or substitution. Rules read the
  command text, so they are weaker than a sandbox.
- **Neither**: refuse read runs. Exit 1 with a reason rather than run with write access.

## Example: a harness in shell

A harness for a made-up CLI, `mycli`, that takes a prompt with `-p`, a model with `--model`, and
has a `--readonly` flag:

```sh
#!/bin/sh
# ~/.ask/harnesses/mycli: runs mycli for ask.
if [ "$1" = models ]; then
  printf 'fast\tMy Fast\nsmart\tMy Smart\n'
  exit 0
fi

set -- --model "$ASK_MODEL"
[ -n "$ASK_EFFORT" ] && set -- "$@" --effort "$ASK_EFFORT"
[ "$ASK_ACCESS" = read ] && set -- "$@" --readonly

exec mycli "$@" -p "$(cat)"
```

```sh
chmod +x ~/.ask/harnesses/mycli
ask models
ask -m mycli:smart "What does this project do?"
```

## Example: sessions and usage

If the CLI prints JSON with its answer, session and token counts, a harness in any language can
split them. In Node:

```js
#!/usr/bin/env node
// ~/.ask/harnesses/othercli: runs othercli for ask, with sessions and usage.
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';

if (process.argv[2] === 'models') {
  console.log('pro\tOther Pro');
  process.exit(0);
}
const args = ['run', '--json', '--model', process.env.ASK_MODEL];
if (process.env.ASK_ACCESS === 'read') args.push('--sandbox', 'read-only');
if (process.env.ASK_SESSION) args.push('--resume', process.env.ASK_SESSION);
try {
  const out = JSON.parse(execFileSync('othercli', args, { input: readFileSync(0) }));
  writeFileSync(process.env.ASK_REPORT, JSON.stringify({ session: out.session, input: out.tokens.in, output: out.tokens.out }));
  console.log(out.answer);
} catch (error) {
  console.error(error.stderr?.toString().trim().split('\n').pop() || error.message);
  process.exit(1);
}
```

## Keeping harnesses in sync

`~/.ask/harnesses/` is plain files, so keep it wherever you keep your dotfiles and it follows you
to every machine. Test a harness by running it the way ask does:

```sh
echo "say hi" | ASK_MODEL=fast ASK_ACCESS=read ASK_REPORT=/tmp/r.json ~/.ask/harnesses/mycli
cat /tmp/r.json
```
