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

ask hands a task to a coding agent you already use and gives you its answer. Run one task or many at once, read-only or with
write access, on any model those agents offer. Each agent keeps its own login, tools and sandbox.

### Install

```sh
git clone https://github.com/fschrhunt/ask ~/.local/share/ask
mkdir -p ~/.local/bin
ln -s ~/.local/share/ask/bin/ask ~/.local/bin/ask
```

Requires Node 22+, `~/.local/bin` on your `PATH`, and a harness for each agent you use (below).

### Use

```sh
ask models                                         # what you can run here
ask -m mycli:smart "Where is the retry logic?"     # read-only, the default
ask -m mycli:smart#high -w "Fix the failing test."
ask batch -j 4 -m mycli:fast tasks.json            # many tasks in parallel
```

The answer goes to stdout; a status line naming the model that ran goes to stderr:

```text
ask: My Smart 2 14.2s 31.0k in 812 out $0.0874
```

### Any agent

ask reaches each agent through a harness: a small executable in `~/.ask/harnesses/` with a
[simple contract](docs/harnesses.md). Harnesses are yours and stay local; a minimal one is a few
lines of shell.

### Docs

[Install](docs/install.md) · [Usage](docs/usage.md) · [Batches](docs/batches.md) ·
[Models](docs/models.md) · [Harnesses](docs/harnesses.md)

### License

MIT
