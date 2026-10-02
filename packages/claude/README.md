# claude

The official ask agent for [Claude Code](https://claude.com/claude-code).
It runs `claude -p` for ask, so `ask -m claude:sonnet-5.5 "..."` hands a task to Claude Code.

## Install

```sh
ask install claude
ask models
```

Requires Claude Code, logged in, and Node.js 18 or newer. The agent finds both even when they are
not on ask's `PATH`: Claude Code in `~/.local/bin`, `~/.claude/local`, `/opt/homebrew/bin`,
`/usr/local/bin`, `~/.npm-global/bin` or `~/.local/share/pnpm`; Node.js in Homebrew, `/usr/local/bin`,
Volta, nvm (the newest), fnm or `~/.local/bin`. Without one, `ask models` shows what to install.

## Models

`claude models` lists the current models: `fable-5.1`, `opus-5.5`, `sonnet-5.5`, `haiku-4.5`.
Name any other the same way, family then version (`sonnet-5` becomes Claude Code's
`claude-sonnet-5`), or by Claude Code's full id (`claude-haiku-4-5-20251001`), and add the ones you
use to `~/.ask/models.json`. Names match without regard to case.

Claude Code's aliases (`opus`, `sonnet`, `best`, ...) are refused with the exact name to use: they
change meaning when a new model ships, so a run's record would no longer say which model answered.
An effort (`claude:opus-5.5#high`) is passed as `--effort`.

## Read and write

- **Read runs** use Claude Code's default permission mode with only Read, Grep and Glob and a short
  list of inspection commands (`git diff`, `git log`, `git show`, `git status`, `git blame`,
  `git ls-files`, `rg`, `grep`, `ls`, `wc`, `cat`, `head`, `tail`). Commands with redirects, pipes,
  chaining, substitution, quoting, escaping or options that write files are refused, and so are
  Edit, Write and NotebookEdit. The prompt starts with an instruction to answer from the files in
  the working directory and never guess.
- **Write runs** (`ask -w`) bypass permissions: the agent edits files and runs commands freely.

A `--schema` is passed to Claude Code's `--json-schema`, and the answer is its structured output.

## Follow-ups

Every run starts with its own session id, reported to ask at once, so `ask -c RUN` resumes it with
`--resume`, even after a run that was stopped. A follow-up's prompt is passed as is.

## Usage

The agent streams Claude Code's events and rewrites ask's report as each model call starts and
finishes, so token counts show live; the cost arrives with the result. The report names the model
that did most of the work (`Opus 5.5`).

## Environment

| Variable | |
| --- | --- |
| `ASK_CLAUDE_BIN` | The `claude` executable to run, instead of looking on `PATH` and in the usual places. |
| `ASK_NODE` | The Node.js executable to run the agent with. |

The agent also reads ask's contract variables: `ASK_MODEL`, `ASK_EFFORT`, `ASK_ACCESS`,
`ASK_SCHEMA`, `ASK_SESSION` and `ASK_REPORT` (see [Agents](../../docs/agents.md)).

## Testing

```sh
cd packages/claude && node --test
```

The tests drive a fake `claude` in `test/bin`; they need no network or account. To try the real
thing by hand:

```sh
echo "say hi" | ASK_MODEL=haiku-4.5 ASK_ACCESS=read ASK_REPORT=/tmp/r.json \
  ~/.ask/packages/ask/packages/claude/agents/claude
cat /tmp/r.json
```
