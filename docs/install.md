# Install

ask is a single binary with no language runtime or library dependencies. Git is needed for packages,
worktrees and reporting changes. Your agents keep their own CLI and runtime requirements.

## Release binary

Download `ask_VERSION_OS_ARCH.tar.gz` for your system from
[Releases](https://github.com/fschrhunt/ask/releases/latest): `linux` or `darwin` (macOS),
and `amd64` (Intel/AMD) or `arm64` (Apple Silicon/ARM). Download `checksums.txt` from the same release.
Verify the archive's SHA-256 against that file (`sha256sum` on Linux, `shasum -a 256` on macOS),
then extract the archive and install the executable:

```sh
mkdir -p ~/.local/bin
cp ask ~/.local/bin/ask
```

Make sure `~/.local/bin` is on your `PATH`.

## With Go

Go 1.26 or newer can build and install ask:

```sh
go install github.com/fschrhunt/ask/cmd/ask@latest
```

Put `$(go env GOPATH)/bin` on your `PATH`.

## From source

```sh
git clone https://github.com/fschrhunt/ask
cd ask
go build -o ask ./cmd/ask
mkdir -p ~/.local/bin
cp ask ~/.local/bin/ask
```

`ask --version` prints the release tag, or `dev` for an unversioned source build.

## Set up

```sh
ask setup
```

It finds the coding agent CLIs you have, connects ask to them, sets your defaults and teaches the
apps you work in to use ask; see [Setup](setup.md). ask runs each coding agent through an agent:
a small executable that runs that agent's CLI. The official ones are built into ask and install by
name, which is what setup does:

```sh
ask install claude        # Claude Code
ask install codex         # Codex
ask install opencode      # Opencode
```

Each needs its CLI installed and logged in, and Node.js 18 or newer. The agent finds the CLI even
when it isn't on your `PATH`, and setup says what is missing if anything is. To write your own,
see [Agents](agents.md): a minimal one is a few lines of shell.

Then check what ask can reach:

```sh
ask models
```

```text
claude:sonnet-5.5
claude:haiku-4.5
```

And try a first run:

```sh
ask -m claude:haiku-4.5 "What does this project do?"
```

## Update

Replace the binary with the latest release, or rerun `go install ...@latest`. For a source checkout,
pull the changes, rebuild and copy the binary again. Your `~/.ask` agents, hooks, commands, packages
and recorded runs keep working.

## Where ask keeps things

Everything ask uses is in `~/.ask` (set `ASK_HOME` to move it):

```text
~/.ask/
├── agents/       one executable per coding agent (see agents.md)
├── hooks/        change tasks and check results (see hooks.md)
├── commands/     your own ask commands (see commands.md)
├── packages/     installed packages (see packages.md)
├── models.json   extra model ids per agent (see models.md)
├── settings.json your defaults: model, timeout, worktree folders (see settings.md)
├── runs/         recorded runs (see runs.md)
└── worktrees/    isolated write runs (see usage.md)
```

ask creates `runs/` on the first run, and `worktrees/` when needed.
