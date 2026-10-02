# Settings

`~/.ask/settings.json` (or `$ASK_HOME/settings.json`) holds your defaults. Every key is optional;
without the file, ask uses the built-in defaults below.

```json
{
  "model": "claude:sonnet-5.5",
  "timeout": 1800,
  "jobs": 6,
  "worktrees": "~/code/worktrees/ask-{name}",
  "branches": "ask/{name}"
}
```

| Key | What it sets | Built in |
| --- | --- | --- |
| `model` | The model when neither `-m`, a batch task, nor a follow-up gives one | none: `-m` is required |
| `timeout` | Seconds a task may run, unless `-t` or the task says otherwise | `900` |
| `jobs` | Batch tasks at once, unless `-j` says otherwise | `4` |
| `worktrees` | Where `--worktree` puts a run's worktree: an absolute or `~` path whose last part holds `{name}` once | `~/.ask/worktrees/{name}` |
| `branches` | The branch of a run's worktree, holding `{name}` once | `ask/{name}` |

`{name}` is the run's name, like `add-rate-limiting-login`, or `NAME-POSITION-TASK` for a batch
task. With the example above, `ask -w --worktree "Add rate limiting to login"` works in
`~/code/worktrees/ask-add-rate-limiting-login` on branch `ask/add-rate-limiting-login`.

An unknown key or a value of the wrong kind is an error that names the key, so a typo never
silently does nothing. Settings change only new runs: a follow-up finds its worktree where the
current `worktrees` setting puts it.
