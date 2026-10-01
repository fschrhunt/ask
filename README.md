<p align="center">
  <picture>
    <source srcset="assets/white/lockup.svg" media="(prefers-color-scheme: dark)">
    <source srcset="assets/black/lockup.svg" media="(prefers-color-scheme: light)">
    <img src="assets/black/lockup.svg" alt="ask" height="48">
  </picture>
</p>
<p align="center">Every model, as a subagent.</p>
<p align="center"><a href="https://github.com/fschrhunt/ask/actions/workflows/ci.yml"><img src="https://github.com/fschrhunt/ask/actions/workflows/ci.yml/badge.svg" alt="CI"></a></p>

---

ask hands a task to a coding agent you already use — Claude Code, Codex or Opencode — and gives
you its answer. Run one task or many at once, read-only or with write access, on any model those
agents offer. Each agent keeps its own login, tools and sandbox. ask is one Node file with no
dependencies.

### Install

```sh
git clone https://github.com/fschrhunt/ask ~/.local/share/ask
mkdir -p ~/.local/bin
ln -s ~/.local/share/ask/ask ~/.local/bin/ask
```

Requires Node 20+ and at least one of `claude`, `codex` or `opencode`, installed and logged in.

### Use

```sh
ask -m claude:sonnet "Where is the retry logic, and what are its limits?"
ask -m codex:MODEL#high -C ~/code/app "Review the last commit."
```

Every run names its model with `-m harness:id`, plus an optional `#effort`. `ask models` lists the
choices.

**Many tasks at once.** A batch is a JSON array, or one object per line:

```json
[
  { "id": "api", "prompt": "Summarize the API surface." },
  { "id": "tests", "prompt": "Which tests are flaky?", "model": "codex:MODEL" }
]
```

```sh
ask batch -j 4 -m claude:sonnet tasks.json   # -m is the default for tasks without one
ask runs                                     # recent runs
ask batch --resume ~/.ask/runs/RUN           # rerun only what did not finish
```

**Structured answers.** `--json` requires JSON; `--schema FILE` requires JSON that matches a
schema, checked by ask for every agent.

| Option | Meaning |
| --- | --- |
| `-r` | Read-only (the default) |
| `-w` | Read and write: the agent may edit files and run commands as you |
| `-C DIR` | Directory the agent works in |
| `-t SECONDS` | Time limit per run (default 900) |
| `-j N` | Tasks running at once in a batch (default 4) |

`ask --help` has the full reference.

### Read-only, per agent

- **Codex** runs in its read-only OS sandbox: it may run any command, but nothing can write.
- **Claude Code and Opencode** get their own read and search tools, plus inspection commands:
  `git diff`, `git log`, `git show`, `git status`, `git blame`, `git ls-files`, `rg`, `grep`,
  `ls`, `wc`, `cat`, `head` and `tail`. Commands with redirects, pipes, chaining, quotes,
  substitutions or options that write files or run programs (`--output`, `--ext-diff`,
  `--textconv`, `--pre`, `--hostname-bin`) are refused.

These rules read the command, not what a repository configures; git can still run a diff filter a
repository defines. For a repository you don't trust, use Codex for read runs.

### Models

Claude Code models are its aliases (`opus`, `sonnet`, …) and Codex models come from Codex's own
list. Opencode models are the ones you name in `~/.ask/models.json`:

```json
{ "opencode": ["provider/model"] }
```

`ASK_HOME` moves `models.json` and `runs/` somewhere other than `~/.ask`.

### Output

- **stdout:** only answers: text, or JSON with `--json`/`--schema`. A batch prints a JSON array of
  `{id, model, name, ok, answer, error, seconds, usage}` in task order.
- **stderr:** one status line per run, naming the model that ran (Opus 5.5, GPT-6.1 Sol), with its
  time and usage — tokens, plus cost when the agent reports it.
- **Exit:** 0 on success, 1 when a run failed, 2 on a usage error. Stopping ask stops every agent
  it started.

### License

MIT
