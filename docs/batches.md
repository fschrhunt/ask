# Batches

`ask batch` runs many tasks in parallel, on one model or several, and prints all the answers as one
JSON array.

```sh
ask batch [-j N] [-m MODEL] [-w] FILE
ask batch [-j N] [-m MODEL] [-w] -        # tasks on stdin
```

## Tasks

A batch is a JSON array, or one JSON object per line:

```json
[
  { "id": "api", "prompt": "Summarize the public API in src/." },
  { "id": "tests", "prompt": "Which tests are flaky, and why?", "model": "codex:gpt-6.1-sol" },
  { "id": "fix", "prompt": "Fix the typo in README.md.", "write": true }
]
```

| Field | Meaning |
| --- | --- |
| `prompt` | The task. Required. |
| `id` | A name for the result. Default: the task's position, from 1. |
| `model` | `harness:id[#effort]`. Required unless the batch's `-m` gives a default. |
| `write` | `true` for read and write. Default: the batch's `-w`, else read only. |
| `dir` | Directory the agent works in. Default: the batch's `-C`, else the current directory. |
| `json`, `schema` | Like `--json` and `--schema`; `schema` is the schema itself, not a file. |
| `timeout` | Seconds for this task. Default: the batch's `-t`, else 900. |

## Running

```sh
ask batch -j 4 -m claude:sonnet tasks.json
```

`-j` is how many tasks run at once (default 4). A status line per task goes to stderr as it starts
and ends:

```text
ask: run /home/me/.ask/runs/20261001T120000-4242
ask: [api] Sonnet started
ask: [tests] GPT-6.1 Sol started
ask: [api] Sonnet 5.5 ok 18.1s 22.4k in 640 out $0.0710
ask: [tests] GPT-6.1 Sol ok 41.0s 51.2k in 1.9k out
ask: 3/3 ok, 98.2k in 3.1k out $0.1050 (/home/me/.ask/runs/20261001T120000-4242)
```

## Results

stdout gets one JSON array, in task order, whatever order the tasks finished in:

```json
[
  {
    "id": "api",
    "model": "claude:sonnet",
    "name": "Sonnet 5.5",
    "ok": true,
    "answer": "The public API is...",
    "seconds": 18.1,
    "usage": { "input": 22400, "output": 640, "cached": 18000, "cost": 0.071 }
  },
  {
    "id": "tests",
    "model": "codex:gpt-6.1-sol",
    "name": "GPT-6.1 Sol",
    "ok": false,
    "error": "timed out",
    "seconds": 900,
    "usage": null
  }
]
```

The batch exits 1 if any task failed. With `jq`:

```sh
ask batch tasks.json | jq -r '.[] | select(.ok) | "\(.id): \(.answer)"'
```

## The same question to several models

```sh
for m in claude:opus codex:gpt-6.1-sol opencode:opencode-go/glm-5.3-flash; do
  echo "{\"id\": \"$m\", \"model\": \"$m\", \"prompt\": \"Is there a race in src/queue.js?\"}"
done | ask batch -
```

## Recorded runs and resuming

Every batch is recorded in `~/.ask/runs/<time>-<pid>/` as `tasks.json` and `results.json`.
`results.json` is rewritten after each task, so you can read it while the batch runs.

```sh
ask runs
```

```text
/home/me/.ask/runs/20261001T120000-4242	2 ok	1 failed	0 pending
/home/me/.ask/runs/20260930T090000-3131	5 ok	0 failed	3 stopped (ask batch --resume /home/me/.ask/runs/20260930T090000-3131)
```

`--resume` reruns only the tasks that did not finish ok, and keeps the rest:

```sh
ask batch --resume ~/.ask/runs/20260930T090000-3131
```
