# Runs

Every time ask starts agents, that is a run, with a short id like `k3f9a2`. Status lines start
with it:

```text
ask k3f9a2 · mycli:atlas-2.1 · read · ~/code/app · started
ask k3f9a2 · Atlas 2.1 · ok · 14.2s · 31.0k in · 812 out · $0.0874
```

A task in a batch is `RUN/TASK`, using the task's id (or its position, from 1): `p81c0d/api`.

## Follow-ups

`-c RUN` continues that run's agent conversation: the agent remembers what it read, said and did.

```sh
ask -m mycli:atlas-2.1 "Why does the login test fail?"
# ask k3f9a2 · Atlas 2.1 · ok · 48.0s · ...
ask -c k3f9a2 -w "Fix it, then run the test."
ask -c b7x01q "Now add a test for the expired-token case."
```

A follow-up:

- runs where the first run ran: the same directory, or the same worktree;
- keeps the model and access (`-r`/`-w`) unless you give new ones. The model may change within
  the same agent (`-c k3f9a2 -m mycli:atlas-2.1-mini`), but not to another agent;
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
b7x01q  14:12    ok       Atlas 2.1   2m 31s  ↪ k3f9a2 Fix it, then run the test.
k3f9a2  14:09    ok       Atlas 2.1   48.0s   Why does the login test fail?
p81c0d  13:50    2/3 ok   3 tasks              Summarize the public API in src/.
x7d2e1  Sep 30   stopped  5 tasks              resume: ask batch --resume x7d2e1
```

`running` means an ask is working on it now; `stopped` means it was interrupted before every task
finished.

## Showing a run again

```sh
ask show k3f9a2           # the answer on stdout, the status line on stderr
ask show k3f9a2 --json    # the whole result, with session, changes and worktree
ask show p81c0d           # a batch: all its results, as a JSON array
ask show p81c0d/api       # one task of a batch
```

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
└── lock           while running: the pid of the ask running it
```

Delete old folders whenever you like; nothing else refers to them.
