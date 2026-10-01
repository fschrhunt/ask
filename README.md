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

ask hands a task to a coding agent you already use — Claude Code, Codex, Opencode, or any other
you add a harness for — and gives you its answer. Run one task or many at once, read-only or with
write access, on any model those agents offer. Each agent keeps its own login, tools and sandbox.

### Install

```sh
git clone https://github.com/fschrhunt/ask ~/.local/share/ask
ln -s ~/.local/share/ask/bin/ask ~/.local/bin/ask
```

Requires Node 22+ and at least one of `claude`, `codex` or `opencode`, installed and logged in.

### Use

```sh
ask models                                         # what you can run here
ask -m claude:sonnet "Where is the retry logic?"   # read-only, the default
ask -m codex:gpt-6.1-sol#high -w "Fix the failing test."
ask batch -j 4 -m claude:sonnet tasks.json         # many tasks in parallel
```

The answer goes to stdout; a status line naming the model that ran goes to stderr:

```text
ask: Sonnet 5.5 14.2s 31.0k in 812 out $0.0874
```

### Any agent

Each agent is reached through a harness, a small program with a [simple contract](docs/harnesses.md).
`claude`, `codex` and `opencode` ship with ask; put your own in `~/.ask/harnesses/` to add an agent
or change how one runs.

### Docs

[Install](docs/install.md) · [Usage](docs/usage.md) · [Batches](docs/batches.md) ·
[Models](docs/models.md) · [Harnesses](docs/harnesses.md)

### License

MIT
