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

Download the archive for your system from [Releases](https://github.com/fschrhunt/ask/releases/latest),
extract it, and put `ask` on your `PATH`. ask is one binary; it needs no Node or other language runtime.
Git is needed for worktrees, tracking changes and packages. Add an agent for each coding agent you
use (below); agents may be written in any language and keep their own requirements.

With Go 1.26 or newer:

```sh
go install github.com/fschrhunt/ask/cmd/ask@latest
```

Or build from source:

```sh
git clone https://github.com/fschrhunt/ask
cd ask
go build -o ask ./cmd/ask
mkdir -p ~/.local/bin
cp ask ~/.local/bin/ask
```

Make sure your install directory (`$(go env GOPATH)/bin` for `go install`, or `~/.local/bin` above)
is on your `PATH`. See [Install](docs/install.md) for checksums and updates.

### Use

```sh
ask setup                                           # connect your agents, set defaults
ask models                                          # what you can run here
ask -m claude:sonnet-5.5 "Why does the login test fail?"  # read-only, the default
ask -c login-test-fail -w "Fix it, then run the test." # follow up: same agent, same conversation
ask -m claude:sonnet-5.5 -w --worktree "Add rate limiting to login." # own worktree and branch
ask batch -j 4 -m claude:haiku-4.5 tasks.json             # many tasks in parallel
```

The answer goes to stdout. A terminal shows live task state; pipes get concise status lines
on stderr naming the run, the model that ran, and what it
changed and cost:

```text
ask login-test-fail · started · Sonnet 5.5 · read · ~/code/app
ask login-test-fail · ok · Sonnet 5.5 · 48.0s · 31.0k in · 812 out · $0.09
```

Every run is named after its prompt and recorded: `ask runs` lists this repository's, `ask show
RUN` prints one again, and `ask stop RUN` stops one. [Settings](docs/settings.md) hold your
defaults, like a model so `-m` is optional, or where worktrees go.

### Make it yours

ask stays small and you add to it with executables in `~/.ask`, in any language:

- **agents/** run each coding agent's CLI: Claude Code, Codex, Opencode, or any other.
  `ask install claude`, `codex` or `opencode` adds the official one.
- **hooks/** change tasks and check results: add context, guard writes, run the tests and have the
  agent fix what fails.
- **commands/** add `ask NAME` workflows, like a review by several models.

Share them as [packages](docs/packages.md): `ask install owner/repo`, and `ask install` keeps them up
to date. Yours always win over a package's, and the [contracts](docs/compatibility.md) they rely on
stay stable across releases.

### Docs

[Install](docs/install.md) · [Usage](docs/usage.md) · [Runs](docs/runs.md) ·
[Setup](docs/setup.md) · [Batches](docs/batches.md) · [Models](docs/models.md) · [Settings](docs/settings.md) · [Agents](docs/agents.md) ·
[Hosts](docs/hosts.md) · [Hooks](docs/hooks.md) · [Commands](docs/commands.md) · [Packages](docs/packages.md) ·
[Compatibility](docs/compatibility.md)

### License

MIT
