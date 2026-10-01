# Install

ask needs Node 20 or newer and at least one agent CLI, installed and logged in: `claude`
(Claude Code), `codex` (Codex) or `opencode` (Opencode). It has no dependencies of its own.

```sh
git clone https://github.com/fschrhunt/ask ~/.local/share/ask
mkdir -p ~/.local/bin
ln -s ~/.local/share/ask/bin/ask ~/.local/bin/ask
```

Make sure `~/.local/bin` is on your `PATH`, then check what ask can reach:

```sh
ask models
```

```text
claude:fable
claude:opus
claude:sonnet
claude:haiku
codex:gpt-6.1-sol
...
```

A harness whose CLI is not installed still lists its models; running one fails with the CLI's
error. Try a first run:

```sh
ask -m claude:haiku "What is in this directory?"
```

## Update

```sh
git -C ~/.local/share/ask pull
```

## Where ask keeps things

Everything ask stores is in `~/.ask` (set `ASK_HOME` to move it):

```text
~/.ask/
├── harnesses/    your own harnesses (see harnesses.md)
├── models.json   extra model ids per harness (see models.md)
└── runs/         recorded batches (see batches.md)
```

None of it is required; ask creates `runs/` on the first batch.
