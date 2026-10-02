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
  { "id": "tests", "prompt": "Which tests are flaky, and why?", "model": "codex:gpt-6.1-sol" },
  { "id": "fix", "prompt": "Fix the typo in README.md.", "write": true }
]
```

| Field | Meaning |
| --- | --- |
| `prompt` | The task. Required. |
| `id` | A name for the result. Default: the task's position, from 1. |
| `model` | `agent:id[#effort]`. Required unless the batch's `-m` gives a default or the task continues a run. |
| `write` | `true` for read and write. Default: the batch's `-w`, else read only. |
| `worktree` | `true` to work in its own git worktree and branch (creating one needs write). Default: the batch's `--worktree`. |
| `continue` | A run to follow up, like `"login-test-fail"` or `"summarize-public-api-src/api"` (see [Runs](runs.md#follow-ups)). |
| `dir` | Directory the agent works in. Default: the batch's `-C`, else the current directory. |
| `json`, `schema` | Like `--json` and `--schema`; `schema` is the schema itself, not a file. |
| `timeout` | Seconds for this task. Default: the batch's `-t`, else 900. |

Options given to `ask batch` are defaults; a task's own fields win.

## Running

```sh
ask batch -j 4 -m claude:haiku-4.5 tasks.json
```

`-j` is how many tasks run at once (default 4). In a terminal, stderr shows live task rows (queued, running with elapsed time, ok or failed),
then a summary with total usage and cost. In pipes, the same status lines go to stderr as tasks start and end:

```text
ask summarize-public-api-src · started · batch of 3 · 3 at a time
ask summarize-public-api-src/api · started · Haiku 4.5 · read · ~/code/app
ask summarize-public-api-src/tests · started · GPT-6.1 Sol · read · ~/code/app
ask summarize-public-api-src/fix · started · Haiku 4.5 · write · ~/code/app
ask summarize-public-api-src/api · ok · Haiku 4.5 · 18.1s · 22.4k in · 640 out · $0.07
ask summarize-public-api-src/fix · ok · Haiku 4.5 · 25.3s · 1 file changed · 30.2k in · 410 out · $0.03
ask summarize-public-api-src/tests · failed · GPT-6.1 Sol · 15:00 · timed out
ask summarize-public-api-src · 2/3 ok · 15:00 · 52.6k in · 1.1k out · $0.11
```

## Results

stdout gets one JSON array, in task order, whatever order the tasks finished in:

```json
[
  {
    "run": "summarize-public-api-src/api",
    "id": "api",
    "model": "claude:haiku-4.5",
    "write": false,
    "name": "Haiku 4.5",
    "ok": true,
    "answer": "The public API is...",
    "seconds": 18.1,
    "usage": { "input": 22400, "output": 640, "cached": 18000, "cost": 0.071 },
    "session": "5f0c...",
    "dir": "/home/me/code/app"
  },
  {
    "run": "summarize-public-api-src/tests",
    "id": "tests",
    "model": "codex:gpt-6.1-sol",
    "write": false,
    "name": "GPT-6.1 Sol",
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
something have `worktree: {path, branch}`. The batch exits 1 if any task failed. Result records include the effective `model` and `write` access after task hooks, so follow-ups inherit what actually ran. With `jq`:

```sh
ask batch tasks.json | jq -r '.[] | select(.ok) | "\(.id): \(.answer)"'
```

## Patterns

**The same question to several models:**

```sh
for m in claude:sonnet-5.5 codex:gpt-6.1-sol opencode:deepseek-4.1-flash; do
  echo "{\"id\": \"$m\", \"model\": \"$m\", \"prompt\": \"Is there a race in src/queue.js?\"}"
done | ask batch -
```

**Parallel fixes that can't collide:** give each task its own worktree, then review each branch.

```sh
gh issue list --label bug --limit 5 --json number,title,body |
  jq -c '.[] | {id: "issue-\(.number)", prompt: "Fix issue #\(.number): \(.title)\n\n\(.body)"}' |
  ask batch -w --worktree -m claude:sonnet-5.5 -
```

Each issue becomes a task with its own branch (`ask/RUN-1-issue-12`, where RUN is the batch's
name), so you review and merge them one by one.

**Follow up on every task of a batch:**

```sh
ask show summarize-public-api-src | jq -c '.[] | select(.ok) | {continue: .run, prompt: "Now write a test for that."}' | ask batch -w -
```

## Resuming

A batch that was stopped, or had failures, can be resumed. Only tasks that did not finish ok run
again, exactly as recorded:

```sh
ask batch --resume summarize-public-api-src
```

`--resume` takes only `-j` and `--no-hooks`. Only one ask can run a batch at a time; a second is refused while the
first is going. See [Runs](runs.md) for listing, showing and stopping runs.
