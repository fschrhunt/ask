# Harnesses

A harness is a small program that ask runs to reach one agent CLI. ask itself knows nothing about
any particular agent; everything specific to Claude Code, Codex or Opencode lives in its harness.
That is how ask supports any agent: add a harness.

ask finds a harness named `NAME` at:

1. `~/.ask/harnesses/NAME`, your own, if it exists, so yours wins;
2. otherwise the one shipped with ask in `harnesses/NAME`.

Three ship with ask: `claude`, `codex` and `opencode`.

## Read-only

Each harness decides how to keep a read run (`ASK_ACCESS=read`) from writing:

- **codex** runs Codex in its read-only OS sandbox: the agent may run any command, but nothing can
  write. Write runs use `workspace-write` with network access, so installs and tests work.
- **claude** and **opencode** have no read-only shell, so read runs get only their file-reading and
  search tools plus these inspection commands: `git diff`, `git log`, `git show`, `git status`,
  `git blame`, `git ls-files`, `rg`, `grep`, `ls`, `wc`, `cat`, `head` and `tail`. A command with a
  redirect, pipe, chain, quote, substitution, or an option that writes files or runs programs
  (`--output`, `--ext-diff`, `--textconv`, `--pre`, `--hostname-bin`) is refused.

These rules read the command, not what a repository configures; git can still run a diff filter
that a repository defines. For a repository you don't trust, use `codex` for read runs.

**Your own harness makes its own promise.** ask passes `ASK_ACCESS=read` and trusts the harness to
keep it. If your harness can't make its agent read-only, refuse read runs (exit 1 with a reason)
rather than ignore them.

## The contract

A harness is any executable: a shell script, Node, Python, a binary.

### Listing models: `NAME models`

Print one model per line, as `id` or `id<TAB>name`. The name is what status lines show.

```text
small	Small One
big
```

Print nothing if the CLI has too many models to list; users add the ones they use to
`~/.ask/models.json` (see [Models](models.md)).

### Running: `NAME`

ask runs the harness with no arguments, in the directory the agent should work in (`-C`).

| Input | |
| --- | --- |
| stdin | The prompt, complete. ask has already added any read-run or JSON instructions. |
| `ASK_MODEL` | The model id, without the harness or effort: `opus`. |
| `ASK_EFFORT` | The effort from `#effort`, or empty. |
| `ASK_ACCESS` | `read` or `write`. |
| `ASK_SCHEMA` | Set only with `--schema`: a file holding the JSON Schema, for CLIs that enforce one. |
| `ASK_REPORT` | A file path for the optional report. |

| Output | |
| --- | --- |
| stdout | The answer, and nothing else. |
| exit code | 0 when the answer is good, anything else when the run failed. |
| stderr | On failure, the reason as the last line. ask shows that line. |
| `$ASK_REPORT` | Optional JSON: `{"name", "input", "output", "cached", "cost", "note"}`. |

In the report, `name` is the model that actually ran (`Opus 5.5`), the counts are tokens, `cost` is
in USD, and `note` is a short remark ask adds to the status line (`hit step cap; answer may be
partial`). Every field is optional.

ask handles everything else: timeouts, stopping, batches, recording runs and checking JSON answers.
It runs each harness in its own process group, so stopping a run also stops the CLI your harness
started.

## Example: a harness in shell

A harness for a made-up CLI, `mycli`, that takes a prompt with `-p`, a model with `--model`, and
has a `--readonly` flag:

```sh
#!/bin/sh
# ~/.ask/harnesses/mycli: runs mycli for ask. See ask's docs/harnesses.md.
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
ask models | grep mycli
ask -m mycli:smart "What does this project do?"
```

## Example: changing a shipped harness

Copy it into `~/.ask/harnesses/` and edit the copy. Yours wins from then on. The shipped harnesses
import helpers from ask's `src/`, so point the copy's imports at your ask checkout:

```sh
mkdir -p ~/.ask/harnesses
sed "s|'\.\./src/|'$HOME/.local/share/ask/src/|" ~/.local/share/ask/harnesses/codex > ~/.ask/harnesses/codex
chmod +x ~/.ask/harnesses/codex
```

Delete the copy to go back to the shipped one.
