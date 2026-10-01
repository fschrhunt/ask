# Install

ask needs Node 22 or newer. It has no dependencies.

```sh
git clone https://github.com/fschrhunt/ask ~/.local/share/ask
mkdir -p ~/.local/bin
ln -s ~/.local/share/ask/bin/ask ~/.local/bin/ask
```

Make sure `~/.local/bin` is on your `PATH`.

## Add a harness

ask reaches each agent through a harness, an executable you keep in `~/.ask/harnesses/`. ask ships
none: write one for each agent CLI you use, installed and logged in. [Harnesses](harnesses.md) has
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
├── harnesses/    one executable per agent (see harnesses.md)
├── models.json   extra model ids per harness (see models.md)
└── runs/         recorded batches (see batches.md)
```

ask creates `runs/` on the first batch.
