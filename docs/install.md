# Install

ask needs Node 22 or newer. It has no dependencies.

```sh
git clone https://github.com/fschrhunt/ask ~/.local/share/ask
mkdir -p ~/.local/bin
ln -s ~/.local/share/ask/bin/ask ~/.local/bin/ask
```

Make sure `~/.local/bin` is on your `PATH`.

## Add an agent

ask runs each coding agent through an agent: an executable in `~/.ask/agents/` that runs that
agent's CLI. ask ships none: write one for each CLI you use, installed and logged in. [Agents](agents.md) has
the contract and examples; a minimal one is a few lines of shell.

Then check what ask can reach:

```sh
ask models
```

```text
mycli:atlas-2.1-mini
mycli:atlas-2.1
```

And try a first run:

```sh
ask -m mycli:atlas-2.1-mini "What is in this directory?"
```

## Update

```sh
git -C ~/.local/share/ask pull
```

## Where ask keeps things

Everything ask uses is in `~/.ask` (set `ASK_HOME` to move it):

```text
~/.ask/
├── agents/       one executable per coding agent (see agents.md)
├── hooks/        change tasks and check results (see hooks.md)
├── commands/     your own ask commands (see commands.md)
├── packages/     installed packages (see packages.md)
├── models.json   extra model ids per agent (see models.md)
└── runs/         recorded batches (see batches.md)
```

ask creates `runs/` on the first batch.
