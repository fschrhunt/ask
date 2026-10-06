# opencode

The official ask agent for [Opencode](https://opencode.ai). It runs
`opencode run` for ask, so `ask -m opencode:deepseek-4.1-flash "..."` hands a task to Opencode.

## Install

```sh
ask install opencode
ask models
```

Requires Opencode, with at least one provider connected, and Node.js 18 or newer. The agent finds
both even when they are not on ask's `PATH`: Opencode in `~/.opencode/bin`, `~/.local/bin`,
`/opt/homebrew/bin`, `/usr/local/bin` or `~/.bun/bin`; Node.js in Homebrew, `/usr/local/bin`, Volta,
nvm (the newest), fnm or `~/.local/bin`. Without one, `ask models` shows what to install.

Installing the ask package does not install its CLI, Node.js or credentials. `ask models`
checks model listing, not whether a task can authenticate and run.

## Models

The agent lists every model `opencode models` shows, which is every model of the providers you
have set up in Opencode, once each. That can be a hundred or more; turn off the ones you don't
use, in a list you can filter:

```sh
ask settings opencode
ask models opencode:gpt-4o --disable     # or one at a time
```

Name a model like ask's other agents, family-version-variant in lowercase: `deepseek-4.1-flash` for
`deepseek-v4.1-flash`, `qwen-3.8-flash` for `qwen3.8-flash`. The agent finds it in `opencode models`
across all providers. When several providers offer it, the agent takes the one in
`ASK_OPENCODE_PROVIDER`, else `opencode-go`, else `opencode`, else the first alphabetically. A
`provider/model` id is used as it is. Names match without regard to case. An effort
(`opencode:deepseek-4.1-flash#max`) is passed as the model variant (`provider/model#max`).

## Read and write

Opencode has no read-only flag, so read runs inject permission rules for its `plan` agent:

- **Read runs** may use only the read, grep and glob tools; the shell is denied, so no command
  runs, not even `git log`: rules over a command's text cannot follow everything a shell expands.
  Git runs Opencode starts itself take no optional locks and no fsmonitor. The prompt starts with
  an instruction to answer from the files in the working directory and never guess. They stop
  after 25 steps.
- **Write runs** (`ask -w`) use the `build` agent with your own Opencode permissions, for up to 100
  steps.

A run that hits its step cap is noted in ask's status line: its answer may be partial. Runs use
`--standalone`, so stopping a run also ends its Opencode session.

## Follow-ups

The session id of Opencode's first event is reported to ask at once, so `ask -c RUN` continues it
with `--session`. A follow-up's prompt is passed as is.

## Usage

The agent rewrites ask's report after every step with the tokens and cost so far, so both show
live. Opencode 2.0 prints no usage for the step that writes the answer, so once Opencode exits the
agent tries to read that step's tokens and cost from `opencode session export --standalone`
and add them. If export fails, usage can be incomplete.

## Cost limits

Opencode reports cost after every step, so ask enforces its limit (`--max-cost`, models.json or
the `max_cost` setting) itself: once the reported cost passes it, ask stops Opencode. The run fails
as `stopped at the $2.00 cost limit`, keeps its session, and `ask -c RUN` continues it. Cost is
known after each step, so a run can go over the limit by up to one step's cost.

## Environment

| Variable | |
| --- | --- |
| `ASK_OPENCODE_BIN` | The `opencode` executable to run, instead of looking on `PATH` and in the usual places. |
| `ASK_OPENCODE_PROVIDER` | The provider to prefer when several offer a model. |
| `ASK_NODE` | The Node.js executable to run the agent with. |

The agent also reads ask's contract variables: `ASK_MODEL`, `ASK_EFFORT`, `ASK_ACCESS`,
`ASK_SESSION` and `ASK_REPORT` (see [Agents](../../docs/agents.md)).
Opencode enforces no schema, so `ASK_SCHEMA` is ignored; ask still checks JSON answers.

## Testing

```sh
cd packages/opencode && node --test
```

The tests drive a fake `opencode` in `test/bin`; they need no network or account. To try the real
thing by hand:

```sh
echo "say hi" | ASK_MODEL=deepseek-4.1-flash ASK_ACCESS=read ASK_REPORT=/tmp/r.json \
  ~/.ask/packages/ask/packages/opencode/agents/opencode
cat /tmp/r.json
```
