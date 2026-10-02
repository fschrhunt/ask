# Bench

`ask bench` runs the same tasks on several models and compares them: how many passed, how long they
took, and what they cost. You decide what passing means, with a command of your own.

```sh
ask bench -m claude:sonnet-5.5 -m codex:gpt-6.1-sol -m opencode:kimi-k3 -n 3 bench.json
```

```text
model         pass   median   tokens in   cost    $/pass
Sonnet 5.5    5/6    1:12     310k        $0.84   $0.17
GPT-6.1 Sol   6/6    1:48     —           —       —
Kimi K3       3/6    0:41     220k        $0.09   $0.03
```

A dash means the agent did not report it; Codex reports no cost, for example.

## Tasks

A bench file is a [batch](batches.md) file without `model`, since every task runs on every `-m`
model. Each task may add a `check`:

```json
[
  { "id": "parser", "prompt": "Fix the failing test in parser/.", "write": true, "check": "go test ./parser/..." },
  { "id": "entry", "prompt": "Which file defines main? Answer with its path only.", "check": "grep -qx 'cmd/app/main.go'" }
]
```

| Field | Meaning |
| --- | --- |
| `check` | A shell command run in the folder the agent worked in, with the answer on stdin. Exit 0 passes. Without one, an attempt passes when its agent finishes (and its answer matches `schema`, when there is one). |

Every other field works as in a batch: `prompt`, `id`, `write`, `dir`, `json`, `schema`, `timeout`
and `max_cost`. A check has the task's timeout.

Keep checks where the agent cannot read them when that matters: a check that names the expected
answer in the repository gives it away.

## Attempts

Each task runs on each model `-n` times (default 1): that is the number of attempts, and ask asks
before starting them in a terminal (`--yes` skips the question). Agents vary from one run to the
next, so a few attempts per model say more than one.

A write attempt works in its own [worktree](usage.md#working-in-a-worktree) from the same commit, so every
model starts from the same code and none sees another's changes. After its check, the worktree and
its branch are removed; `--keep` keeps them to look at, and the report says where.

Cost limits apply to each attempt, as in a batch: `--max-cost`, else the model's limit or your
setting (see [Usage](usage.md#cost-limits)).

## Options

| Option | Meaning |
| --- | --- |
| `-m MODEL` | A model to compare. Give one `-m` per model, or several separated by commas. |
| `-n N` | Attempts per task and model. Default 1. |
| `-w`, `-C`, `-t`, `--max-cost` | Defaults for every task, as in a batch. |
| `-j N` | Attempts at once. Default 4, or the `jobs` [setting](settings.md). |
| `--keep` | Keep write attempts' worktrees. |
| `--json` | Print the report as JSON. The default when stdout is not a terminal. |
| `--yes` | Start without asking. |
| `--no-hooks` | Skip your hooks. |

ask bench exits 0 when the bench ran, whatever passed; the report says how each model did.

## The report

A bench is a run like any other: `ask runs` lists it, `ask stop` stops it, and `ask show RUN`
prints its table again (`--json` for the report). The report is a JSON object:

```json
{
  "models": [
    { "model": "claude:sonnet-5.5", "name": "Sonnet 5.5", "attempts": 6, "passed": 5,
      "median_seconds": 72.4, "input": 310200, "cost": 0.84 }
  ],
  "attempts": [
    { "task": "parser", "model": "claude:sonnet-5.5", "n": 1, "ok": true, "passed": false,
      "seconds": 80.1, "usage": { "input": 52000, "output": 1900, "cost": 0.15 },
      "note": "check: --- FAIL: TestParseEmpty" }
  ]
}
```

`ok` says the agent finished; `passed` says it also passed its check. `note` says why an attempt did
not pass. Each attempt's full result is in the run, as in a batch: `ask show RUN/TASK`.
