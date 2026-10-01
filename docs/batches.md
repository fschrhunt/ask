# Batches

`ask batch` runs many tasks in parallel, on one model or several, and prints all the answers as one
JSON array.

```sh
ask batch [-j N] [-m MODEL] [-w] [--worktree] FILE
ask batch [-j N] [-m MODEL] [-w] [--worktree] -        # tasks on stdin
```

## Tasks

A batch is a JSON array, or one JSON object per line:

```json
[
  { "id": "api", "prompt": "Summarize the public API in src/." },
  { "id": "tests", "prompt": "Which tests are flaky, and why?", "model": "othercli:pro" },
  { "id": "fix", "prompt": "Fix the typo in README.md.", "write": true }
]
```

| Field | Meaning |
| --- | --- |
| `prompt` | The task. Required. |
| `id` | A name for the result. Default: the task's position, from 1. |
| `model` | `harness:id[#effort]`. Required unless the batch's `-m` gives a default or the task continues a run. |
| `write` | `true` for read and write. Default: the batch's `-w`, else read only. |
| `worktree` | `true` to work in its own git worktree and branch (needs write). Default: the batch's `--worktree`. |
| `continue` | A run to follow up, like `"k3f9a2"` or `"p81c0d/api"` (see [Runs](runs.md#follow-ups)). |
| `dir` | Directory the agent works in. Default: the batch's `-C`, else the current directory. |
| `json`, `schema` | Like `--json` and `--schema`; `schema` is the schema itself, not a file. |
| `timeout` | Seconds for this task. Default: the batch's `-t`, else 900. |

Options given to `ask batch` are defaults; a task's own fields win.

## Running

```sh
ask batch -j 4 -m mycli:fast tasks.json
```

`-j` is how many tasks run at once (default 4). Status lines go to stderr as tasks start and end:

```text
ask p81c0d · batch of 3 · 3 at a time
ask p81c0d/api · mycli:fast · read · ~/code/app · started
ask p81c0d/tests · othercli:pro · read · ~/code/app · started
ask p81c0d/fix · mycli:fast · write · ~/code/app · started
ask p81c0d/api · My Fast · ok · 18.1s · 22.4k in · 640 out · $0.0710
ask p81c0d/fix · My Fast · ok · 25.3s · 1 file changed · 30.2k in · 410 out · $0.0340
ask p81c0d/tests · Other Pro · failed · 15m 00s · timed out
ask p81c0d · 2/3 ok · 15m 00s · 52.6k in · 1.1k out · $0.1050
```

## Results

stdout gets one JSON array, in task order, whatever order the tasks finished in:

```json
[
  {
    "run": "p81c0d/api",
    "id": "api",
    "model": "mycli:fast",
    "name": "My Fast",
    "ok": true,
    "answer": "The public API is...",
    "seconds": 18.1,
    "usage": { "input": 22400, "output": 640, "cached": 18000, "cost": 0.071 },
    "session": "5f0c...",
    "dir": "/home/me/code/app"
  },
  {
    "run": "p81c0d/tests",
    "id": "tests",
    "model": "othercli:pro",
    "name": "Other Pro",
    "ok": false,
    "error": "timed out",
    "seconds": 900,
    "usage": null,
    "session": "a21e...",
    "dir": "/home/me/code/app"
  }
]
```

Write tasks in a git repository also have `changes` and `commits`, and worktree tasks that changed
something have `worktree: {path, branch}`. The batch exits 1 if any task failed. With `jq`:

```sh
ask batch tasks.json | jq -r '.[] | select(.ok) | "\(.id): \(.answer)"'
```

## Patterns

**The same question to several models:**

```sh
for m in mycli:smart mycli:fast othercli:pro; do
  echo "{\"id\": \"$m\", \"model\": \"$m\", \"prompt\": \"Is there a race in src/queue.js?\"}"
done | ask batch -
```

**Parallel fixes that can't collide:** give each task its own worktree, then review each branch.

```sh
ask batch -w --worktree -m mycli:smart issues.json
```

**Follow up on every task of a batch:**

```sh
ask show p81c0d | jq -c '.[] | select(.ok) | {continue: .run, prompt: "Now write a test for that."}' | ask batch -w -
```

## Resuming

A batch that was stopped, or had failures, can be resumed. Only tasks that did not finish ok run
again, exactly as recorded:

```sh
ask batch --resume p81c0d
```

`--resume` takes only `-j`. Only one ask can run a batch at a time; a second is refused while the
first is going. See [Runs](runs.md) for listing, showing and stopping runs.
