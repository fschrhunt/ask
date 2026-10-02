# Runs

Every time ask starts agents, that is a run, with a short id like `k3f9a2`. Status lines start
with it:

```text
ask k3f9a2 · started · Sonnet 5.5 · read · ~/code/app
ask k3f9a2 · ok · Sonnet 5.5 · 14.2s · 31.0k in · 812 out · $0.09
```

A task in a batch is `RUN/TASK`, using the task's id (or its position, from 1): `p81c0d/api`.

## Follow-ups

`-c RUN` continues that run's agent conversation: the agent remembers what it read, said and did.

```sh
ask -m claude:sonnet-5.5 "Why does the login test fail?"
# ask k3f9a2 · ok · Sonnet 5.5 · 48.0s · ...
ask -c k3f9a2 -w "Fix it, then run the test."
ask -c b7x01q "Now add a test for the expired-token case."
```

A follow-up:

- runs where the first run ran: the same directory, or the same worktree if it is still present;
- keeps the model and access (`-r`/`-w`) unless you give new ones. The model may change within
  the same agent (`-c k3f9a2 -m claude:haiku-4.5`), but not to another agent;
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
RUN     STARTED  STATUS   MODEL        TIME    TASK
b7x01q  2m ago   ok       Sonnet 5.5   2:31  ↪ k3f9a2 Fix it, then run the test.
k3f9a2  5m ago   ok       Sonnet 5.5   48.0s   Why does the login test fail?
p81c0d  24m ago  2/3 ok   3 tasks              Summarize the public API in src/.
x7d2e1  Sep 30   stopped  5 tasks              resume: ask batch --resume x7d2e1
```

`running` means ask holds the run's operating system lock; `stopped` means it was interrupted
before every task finished. Interrupted tasks keep empty result slots, so they can be resumed.

## Showing a run again

```sh
ask show k3f9a2           # the answer on stdout, the status line on stderr
ask show k3f9a2 --json    # the whole result, with session, changes and worktree
ask show p81c0d           # a batch: all its results, as a JSON array
ask show p81c0d/api       # one task of a batch
```

Showing a whole batch prints its outcome count on stderr and exits 1 if any task failed; saved wall time is not recorded.

## Stopping a run

`ask stop RUN` stops a run going in another terminal or in the background, and every agent it
started. It waits up to 10 seconds for them to exit.

```sh
ask stop k3f9a2
```

A stopped batch can be resumed with `ask batch --resume RUN`, which reruns only the tasks that did
not finish ok.

## Where runs live

Each run is a folder in `~/.ask/runs/` (or `$ASK_HOME/runs/`), named by its start time and id:

```text
~/.ask/runs/20261001T140912345-k3f9a2/
├── tasks.json     the tasks, as they were given
├── results.json   the results so far, rewritten whole after each task
└── lock           the ask pid, valid only while ask holds its operating system lock
```

ask creates new run directories with mode 0700 and records with mode 0600. Delete old folders
whenever you like; nothing else refers to them.

Hosts can name a background run with `ask title --command STRING --description TEXT`; see
[Hosts](hosts.md) for title forms and a Claude Code integration.
