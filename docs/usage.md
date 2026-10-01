# Usage

```sh
ask -m MODEL [options] PROMPT
```

Every run names its model. ask never picks one for you, the same way you name a model when you
start a subagent.

```sh
ask -m mycli:atlas-2.1 "Where is the retry logic, and what are its limits?"
ask -m othercli:nova-4#high "Review the last commit for bugs."
```

## The prompt

Give it as arguments, or on stdin with `-` or no prompt at all. Words after `--` are always the
prompt, even ones that look like options:

```sh
ask -m mycli:atlas-2.1 "Summarize src/"
git diff | ask -m othercli:nova-4 -
ask -m mycli:atlas-2.1 < task.md
```

## Read or write

| Option | Access |
| --- | --- |
| `-r`, `--read` | Read only. The default. |
| `-w`, `--write` | Read and write: the agent may edit files and run commands, as you. |

```sh
ask -m mycli:atlas-2.1 "Which functions have no tests?"           # read
ask -m mycli:atlas-2.1 -w "Add tests for parseFlags, then run them" # write
```

Read runs start with a short instruction to search and read the files before answering, so the
agent answers from the code rather than from memory. The harness enforces read-only in its CLI's
own way; see [Harnesses](harnesses.md#read-only).

Running tests or builds writes files, so it needs `-w`.

## What a write run changed

When a write run's directory is in a git repository, ask compares the repository before and after
and reports what the run changed: in the status line, and in full with `ask show RUN --json`.

```text
ask k3f9a2 · Atlas 2.1 · ok · 41.2s · 3 files changed, 1 commit · 52.1k in · 2.0k out · $0.3100
```

```json
"changes": [
  { "path": "src/parser.js", "change": "modified" },
  { "path": "test/parser.test.js", "change": "added" }
],
"commits": 1
```

Files that were already modified before the run are reported only if the run changed them again,
so your own uncommitted work never shows up as the agent's.

## Working in a worktree

`--worktree` (with `-w`) gives the run its own git worktree and branch, so it can't touch your
checkout and several write runs can go in parallel:

```sh
ask -m mycli:atlas-2.1 -w --worktree -C ~/code/app "Add rate limiting to the login route."
```

```text
ask k3f9a2 · mycli:atlas-2.1 · write · worktree ~/.ask/worktrees/k3f9a2 · started
ask k3f9a2 · Atlas 2.1 · ok · 3m 05s · 4 files changed · branch ask/k3f9a2 · 120.4k in · 6.2k out
```

The worktree starts from the repository's `HEAD`, at `~/.ask/worktrees/RUN`, on branch `ask/RUN`.
If the run changed something, both are kept for you to review and merge:

```sh
git -C ~/code/app diff main...ask/k3f9a2
git -C ~/code/app merge ask/k3f9a2
git -C ~/code/app worktree remove ~/.ask/worktrees/k3f9a2 && git -C ~/code/app branch -d ask/k3f9a2
```

If it changed nothing, ask removes them. Uncommitted changes in your checkout are not in the
worktree; ask says so when it starts.

## Where the agent works

`-C DIR` sets the directory the agent works in (default: where you run ask):

```sh
ask -m mycli:atlas-2.1 -C ~/code/app "How does login work?"
```

## Time limit

`-t SECONDS` stops a run that takes too long (default 900). The run fails with `timed out`, and
everything the agent started is stopped.

```sh
ask -m othercli:nova-4 -w -t 3600 "Upgrade the project to Node 24 and fix what breaks."
```

## JSON answers

`--json` requires the answer to be JSON. `--schema FILE` requires JSON that matches a JSON Schema.
ask tells the agent the format, strips a code fence if the agent adds one, and checks the answer
itself, whatever the harness. A harness whose CLI supports schemas natively can also enforce it
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
ask -m mycli:atlas-2.1 --schema findings.json "Find bugs in src/parser.js" | jq '.bugs[].file'
```

ask checks `type`, `enum`, `properties`, `required`, `additionalProperties: false` and `items`. An
answer that is not JSON, or does not match, fails the run with the reason:

```text
ask k3f9a2 · Atlas 2.1 · failed · 12.3s · answer does not match the schema: $.bugs[0]: missing "file"
```

## Follow-ups

Every run gets an id. Continue the same agent conversation with `-c`; see [Runs](runs.md).

```sh
ask -m mycli:atlas-2.1 "Why does the login test fail?"
ask -c k3f9a2 -w "Fix it."
```

## Output

- **stdout** has only the answer: text, or JSON under `--json`/`--schema`. It is safe to pipe.
- **stderr** has a status line when the run starts and one when it ends. Each begins with the run's
  id. The first names what you asked for, the second the model that ran, with the outcome, time,
  what changed and usage when the agent reports it.

```text
ask k3f9a2 · mycli:atlas-2.1 · read · ~/code/app · started
ask k3f9a2 · Atlas 2.1 · ok · 14.2s · 31.0k in · 812 out · $0.0874
```

- **Exit code**: 0 on success, 1 when the run failed, 2 when ask was called wrong (the message says
  what to fix).

Stopping ask (Ctrl-C) stops the agent too.
