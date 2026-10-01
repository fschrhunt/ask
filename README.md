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

ask hands a task to a coding agent you already use and gives you its answer, like a native
subagent: follow it up in the same conversation, run many at once, or give one its own git
worktree. Read-only or with write access, on any model those agents offer. Each agent keeps its
own login, tools and sandbox.

### Install

```sh
git clone https://github.com/fschrhunt/ask ~/.local/share/ask
mkdir -p ~/.local/bin
ln -s ~/.local/share/ask/bin/ask ~/.local/bin/ask
```

Requires Node 22+, `~/.local/bin` on your `PATH`, and a harness for each agent you use (below).

### Use

```sh
ask models                                          # what you can run here
ask -m mycli:smart "Why does the login test fail?"  # read-only, the default
ask -c k3f9a2 -w "Fix it, then run the test."       # follow up: same agent, same conversation
ask -m mycli:smart -w --worktree "Add rate limits." # its own git worktree and branch
ask batch -j 4 -m mycli:fast tasks.json             # many tasks in parallel
```

The answer goes to stdout. Status lines on stderr name the run, the model that ran, and what it
changed and cost:

```text
ask k3f9a2 · mycli:smart · read · ~/code/app · started
ask k3f9a2 · My Smart 2 · ok · 48.0s · 31.0k in · 812 out · $0.0874
```

Every run is recorded: `ask runs` lists them, `ask show RUN` prints one again, and `ask stop RUN`
stops one.

### Any agent

ask reaches each agent through a harness: a small executable in `~/.ask/harnesses/` with a
[simple contract](docs/harnesses.md). Harnesses are yours and stay local; a minimal one is a few
lines of shell.

### Docs

[Install](docs/install.md) · [Usage](docs/usage.md) · [Runs](docs/runs.md) ·
[Batches](docs/batches.md) · [Models](docs/models.md) · [Harnesses](docs/harnesses.md)

### License

MIT
