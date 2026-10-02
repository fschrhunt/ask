# Usage

```sh
ask -m MODEL [options] PROMPT
```

`ask --help` is a compact reference with the models installed here. `ask batch --help`,
`ask help hooks` and `ask help agents` describe their contracts.

Every run names its model. ask never picks one for you, the same way you name a model when you
start a subagent.

```sh
ask -m claude:sonnet-5.5 "Where is the retry logic, and what are its limits?"
ask -m codex:gpt-6.1-sol#high "Review the last commit for bugs."
```

## The prompt

Give it as arguments, or on stdin with `-` or no prompt at all. Words after `--` are always the
prompt, even ones that look like options:

```sh
ask -m claude:sonnet-5.5 "Summarize src/"
git diff | ask -m codex:gpt-6.1-sol -
ask -m claude:sonnet-5.5 < task.md
```

## Read or write

| Option | Access |
| --- | --- |
| `-r`, `--read` | Read only. The default. |
| `-w`, `--write` | Read and write: the agent may edit files and run commands, as you. |

```sh
ask -m claude:sonnet-5.5 "Which functions have no tests?"           # read
ask -m claude:sonnet-5.5 -w "Add tests for parseFlags, then run them" # write
```

Read runs start with a short instruction to search and read the files before answering, so the
agent answers from the code rather than from memory. The agent enforces read-only in its CLI's
own way; see [Agents](agents.md#read-only).

Running tests or builds writes files, so it needs `-w`.

## What a write run changed

When a write run's directory is in a git repository, ask compares the repository before and after
and reports what the run changed: in the status line, and in full with `ask show RUN --json`.

```text
ask k3f9a2 · ok · Sonnet 5.5 · 41.2s · 3 files changed, 1 commit · 52.1k in · 2.0k out · $0.31
```

```json
"changes": [
  { "path": "src/parser.js", "change": "modified" },
  { "path": "test/parser.test.js", "change": "added" }
],
"commits": 1
```

File mode changes and symbolic links are included. Files that were already modified before the run are reported only if the run changed them again,
so your own uncommitted work never shows up as the agent's.

## Working in a worktree

`--worktree` (with `-w`) gives the run its own git worktree and branch, so it can't touch your
checkout and several write runs can go in parallel:

```sh
ask -m claude:sonnet-5.5 -w --worktree -C ~/code/app "Add rate limiting to the login route."
```

```text
ask k3f9a2 · started · Sonnet 5.5 · write · worktree ~/.ask/worktrees/k3f9a2
ask k3f9a2 · ok · Sonnet 5.5 · 3:05 · 4 files changed · branch ask/k3f9a2 · 120.4k in · 6.2k out
```

The worktree starts from the repository's `HEAD`, at `~/.ask/worktrees/RUN`, on branch `ask/RUN`.
If the run changed something, both are kept for you to review and merge:

```sh
git -C ~/code/app diff main...ask/k3f9a2
git -C ~/code/app merge ask/k3f9a2
git -C ~/code/app worktree remove ~/.ask/worktrees/k3f9a2 && git -C ~/code/app branch -d ask/k3f9a2
```

After all hook follow-ups, ask removes a worktree only when git reports it clean and its HEAD
still points to the starting commit. An idle follow-up keeps earlier work. A read-only follow-up
can use a kept worktree; recreating one requires write access. Uncommitted changes in your
checkout are not in the worktree; ask says so when it starts.

## Where the agent works

`-C DIR` sets the directory the agent works in (default: where you run ask):

```sh
ask -m claude:sonnet-5.5 -C ~/code/app "How does login work?"
```

## Time limit

`-t SECONDS` stops a run that takes too long (default 900). The run fails with `timed out`, and
everything the agent started is stopped.

```sh
ask -m codex:gpt-6.1-sol -w -t 3600 "Upgrade the project to Node 24 and fix what breaks."
```

## JSON answers

`--json` requires the answer to be JSON. `--schema FILE` requires JSON that matches a JSON Schema.
ask tells the agent the format, strips a code fence if the agent adds one, and checks the answer
itself, whatever the agent. An agent whose CLI supports schemas natively can also enforce it
there, from `ASK_SCHEMA`.

```sh
cat > findings.json <<'EOF'
{
  "type": "object",
  "properties": {
    "bugs": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "file": { "type": "string" },
          "line": { "type": "integer" },
          "problem": { "type": "string" }
        },
        "required": ["file", "problem"]
      }
    }
  },
  "required": ["bugs"]
}
EOF
ask -m claude:sonnet-5.5 --schema findings.json "Find bugs in src/parser.js" | jq '.bugs[].file'
```

ask honors boolean schemas (`false` rejects every answer) and checks `type`, `enum`, `const`, `properties`, `required`, `additionalProperties: false` and `items` (including `items: false`). An
answer that is not JSON, or does not match, fails the run with the reason:

```text
ask k3f9a2 · failed · Sonnet 5.5 · 12.3s · answer does not match the schema: $.bugs[0]: missing "file"
```

## Follow-ups

Every run gets an id. Continue the same agent conversation with `-c`; see [Runs](runs.md).

```sh
ask -m claude:sonnet-5.5 "Why does the login test fail?"
ask -c k3f9a2 -w "Fix it."
```

## Output

- **stdout** has only the answer: text, or JSON under `--json`/`--schema`. It is safe to pipe.
- **stderr** in a pipe has a status line when the run starts and one when it ends. Each begins
  with the run's id. The first names the task after hooks, the second the model that ran, with the outcome, time,
  what changed and usage when the agent reports it.

```text
ask k3f9a2 · started · Sonnet 5.5 · read · ~/code/app
ask k3f9a2 · ok · Sonnet 5.5 · 14.2s · 31.0k in · 812 out · $0.09
```

- **Exit code**: 0 on success, 1 when the run failed, 2 when ask was called wrong (the message says
  what to fix and points to `ask COMMAND --help`). Missing or malformed model specifications
  also show available models grouped by agent.

Stopping ask (Ctrl-C) stops the agent too.

On a terminal, stderr shows a spinner with the model name, read/write access, directory and
elapsed time, then a final marked row. Hook notes and follow-ups appear beneath their task.
Batches show a header and task rows with queued, running, ok or failed state and reasons.
Rows fit the terminal width; very large batches show a count of hidden lines.
The frame finishes before answers print, so stdout may share the terminal. Ctrl-C stops agents,
leaves the stopped state visible and restores the cursor. `TERM=dumb` uses plain lines;
`NO_COLOR` disables colors while retaining live updates. Piped stderr has no ANSI or spinner.
