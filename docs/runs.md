# Runs

Every time ask starts agents, that is a run. ask names it after the first meaningful words of its
prompt, and status lines start with the name:

```text
ask login-test-fail · started · Sonnet 5.5 · read · ~/code/app
ask login-test-fail · ok · Sonnet 5.5 · 14.2s · 31.0k in · 812 out · $0.09
```

`RUN` in commands is that name. A name stays apart from earlier runs with a number
(`login-test-fail-2`), and a [name hook](hooks.md#naming-runs) can choose better ones. Every run
also has a fixed six-character id, like `k3f9a2`, that works wherever a name does.

A task in a batch is `RUN/TASK`, using the task's id (or its position, from 1):
`summarize-public-api-src/api`.

## Follow-ups

`-c RUN` continues that run's agent conversation: the agent remembers what it read, said and did.

```sh
ask -m claude:sonnet-5.5 "Why does the login test fail?"
# ask login-test-fail · ok · Sonnet 5.5 · 48.0s · ...
ask -c login-test-fail -w "Fix it, then run the test."
ask -c login-test-fail "Now add a test for the expired-token case."
```

A follow-up is a new run that takes over the conversation's name, so the name always reaches the
latest turn. An earlier turn is still reachable by its id. A follow-up to a batch task is named
`RUN-TASK`.

A follow-up:

- runs where the first run ran: the same directory, or the same worktree if it is still present;
- keeps the model and access (`-r`/`-w`) unless you give new ones. The model may change within
  the same agent (`-c login-test-fail -m claude:haiku-4.5`), but not to another agent;
- can continue a run that failed or timed out, so the agent can finish what it started;
- gets your prompt as it is, without the read-run preamble, since the agent already has its bearings.

A batch task can be a follow-up too, with `"continue": "RUN"` (see [Batches](batches.md)).

Continuing needs an agent that reports sessions (see [Agents](agents.md#sessions)); ask says
so when one doesn't.

## Listing

```sh
ask runs          # the last 20
ask runs -n 50
```

```text
RUN                       ID      STARTED  STATUS   MODEL       TIME   TASK
login-test-fail           b7x01q  2m ago   ok       Sonnet 5.5  2:31   ↪ login-test-fail Fix it, then run the test.
login-test-fail           k3f9a2  5m ago   ok       Sonnet 5.5  48.0s  Why does the login test fail?
summarize-public-api-src  p81c0d  24m ago  2/3 ok   3 tasks            Summarize the public API in src/.
find-bugs-internal-runs   x7d2e1  Sep 30   stopped  5 tasks            resume: ask batch --resume find-bugs-internal-runs
```

`running` means ask holds the run's operating system lock; `stopped` means it was interrupted
before every task finished. Interrupted tasks keep empty result slots, so they can be resumed.

## Showing a run again

```sh
ask show login-test-fail           # the latest turn's answer on stdout, its status line on stderr
ask show k3f9a2                    # an earlier turn, by id
ask show login-test-fail --json    # the whole result, with session, changes and worktree
ask show summarize-public-api-src  # a batch: all its results, as a JSON array
ask show summarize-public-api-src/api  # one task of a batch
```

Showing a whole batch prints its outcome count on stderr and exits 1 if any task failed; saved wall time is not recorded.

## Stopping a run

`ask stop RUN` stops a run going in another terminal or in the background, and every agent it
started. It waits up to 10 seconds for them to exit.

```sh
ask stop login-test-fail
```

A stopped batch can be resumed with `ask batch --resume RUN`, which reruns only the tasks that did
not finish ok.

## Where runs live

Each run is a folder in `~/.ask/runs/` (or `$ASK_HOME/runs/`), named by its start time, id and
name (runs from before names have none):

```text
~/.ask/runs/20261001T140912345-k3f9a2-login-test-fail/
├── tasks.json     the tasks, as they were given
├── results.json   the results so far, rewritten whole after each task
└── lock           the ask pid, valid only while ask holds its operating system lock
```

ask creates new run directories with mode 0700 and records with mode 0600. Delete old folders
whenever you like; nothing else refers to them.

Hosts can name a background run with `ask title --command STRING --description TEXT`; see
[Hosts](hosts.md) for title forms and a Claude Code integration.
