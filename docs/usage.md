# Usage

```sh
ask -m MODEL [options] PROMPT
```

Every run names its model. ask never picks one for you, the same way you name a model when you
start a subagent.

```sh
ask -m claude:sonnet "Where is the retry logic, and what are its limits?"
ask -m codex:gpt-6.1-sol#high "Review the last commit for bugs."
```

## The prompt

Give it as arguments, or on stdin with `-` or no prompt at all:

```sh
ask -m claude:opus "Summarize src/"
git diff | ask -m codex:gpt-6.1-sol -
ask -m claude:opus < task.md
```

## Read or write

| Option | Access |
| --- | --- |
| `-r`, `--read` | Read only. The default. |
| `-w`, `--write` | Read and write: the agent may edit files and run commands, as you. |

```sh
ask -m claude:sonnet "Which functions have no tests?"           # read
ask -m claude:sonnet -w "Add tests for parseFlags, then run them" # write
```

Read runs start with a short instruction to search and read the files before answering, so the
agent answers from the code rather than from memory. Each harness enforces read-only its own way;
see [Harnesses](harnesses.md#read-only).

Running tests or builds writes files, so it needs `-w`.

## Where the agent works

`-C DIR` sets the directory the agent works in (default: where you run ask):

```sh
ask -m claude:opus -C ~/code/app "How does login work?"
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
itself, whatever the harness. Harnesses that support schemas natively (Claude Code, Codex) also
enforce it there.

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
ask -m claude:opus --schema findings.json "Find bugs in src/parser.js" | jq '.bugs[].file'
```

ask checks `type`, `enum`, `properties`, `required`, `additionalProperties: false` and `items`. An
answer that is not JSON, or does not match, fails the run with the reason:

```text
ask: Opus 5.5 failed after 12.3s: answer does not match the schema: $.bugs[0]: missing "file"
```

## Output

- **stdout** has only the answer: text, or JSON under `--json`/`--schema`. It is safe to pipe.
- **stderr** has one status line: the model that ran, the time, and usage when the agent reports it.

```text
ask: Opus 5.5 14.2s 31.0k in 812 out $0.0874
```

- **Exit code**: 0 on success, 1 when the run failed, 2 when ask was called wrong (the message says
  what to fix).

Stopping ask (Ctrl-C) stops the agent too.
