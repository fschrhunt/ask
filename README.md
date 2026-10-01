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

Requires Node 22+, `~/.local/bin` on your `PATH`, and an agent for each coding agent you use (below).

### Use

```sh
ask models                                          # what you can run here
ask -m mycli:atlas-2.1 "Why does the login test fail?"  # read-only, the default
ask -c k3f9a2 -w "Fix it, then run the test."       # follow up: same agent, same conversation
ask -m mycli:atlas-2.1 -w --worktree "Add rate limits." # its own git worktree and branch
ask batch -j 4 -m mycli:atlas-2.1-mini tasks.json             # many tasks in parallel
```

The answer goes to stdout. Status lines on stderr name the run, the model that ran, and what it
changed and cost:

```text
ask k3f9a2 · mycli:atlas-2.1 · read · ~/code/app · started
ask k3f9a2 · Atlas 2.1 · ok · 48.0s · 31.0k in · 812 out · $0.0874
```

Every run is recorded: `ask runs` lists them, `ask show RUN` prints one again, and `ask stop RUN`
stops one.

### Make it yours

ask stays small and you add to it with executables in `~/.ask`, in any language:

- **agents/** run each coding agent's CLI: Claude Code, Codex, Opencode, or any other.
- **hooks/** change tasks and check results: add context, guard writes, run the tests and have the
  agent fix what fails.
- **commands/** add `ask NAME` workflows, like a review by several models.

Share them as [packages](docs/packages.md): `ask install owner/repo`, and `ask install` keeps them up
to date. Yours always win over a package's, and the [contracts](docs/compatibility.md) they rely on
stay stable across releases.

### Docs

[Install](docs/install.md) · [Usage](docs/usage.md) · [Runs](docs/runs.md) ·
[Batches](docs/batches.md) · [Models](docs/models.md) · [Agents](docs/agents.md) ·
[Hooks](docs/hooks.md) · [Commands](docs/commands.md) · [Packages](docs/packages.md) ·
[Compatibility](docs/compatibility.md)

### License

MIT
