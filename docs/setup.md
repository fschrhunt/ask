# Setup

`ask setup` connects ask to the coding agents you have, sets your defaults, and teaches the apps
you work in to use ask.

## The first time

In a terminal, `ask setup` walks you through it:

```text
Welcome to ask. Let's connect your coding agents.

? Which coding agents should ask use? claude, codex
✓ claude is ready: 4 models, see ask models
✓ codex is ready: 8 models, see ask models
? Default model, when you don't give -m claude:sonnet-5.5
? Where should --worktree runs work? Somewhere else
? Folder for each worktree, with {name} ~/code/worktrees/ask-{name}
? Teach these apps to use ask (adds the ask skill) Claude Code, Codex
✓ added the ask skill to Claude Code
✓ added the ask skill to Codex
? Show ask runs by model and task in Claude Code's task list? Yes
✓ Claude Code titles ask runs, like Sonnet 5.5 · Fix the login test

ask is set up. Try it in a project:

  ask "What does this project do?"
```

- **Agents** are the official ones for the CLIs it finds (`ask install claude`, `codex`,
  `opencode`); each is checked once installed.
- **Defaults** go to [settings](settings.md): the model, and where worktrees go.
- **The ask skill** goes to each app that reads skills and is installed here: Claude Code, Codex,
  Opencode, Cursor and pi. It tells their agents that ask exists and how to hand it work.
- **Task titles** register `ask title --hook` in Claude Code (see [Hosts](hosts.md)).

Arrow keys move, typing filters a list, space selects, ctrl-a selects everything shown, enter
confirms, and Ctrl-C stops at any point; what was answered so far is kept.

## Later

Run `ask setup` again for your settings, with their current values:

```text
? What do you want to change?
> Agents                      claude, codex
  Default model               claude:sonnet-5.5
  Worktrees                   ~/code/worktrees/ask-{name}
  Branches                    ask/{name} (default)
  Timeout                     900 seconds (default)
  Batch jobs                  4 at once (default)
  Models and cost limits      by agent
  Cost limit                  none (default)
  The ask skill               Claude Code, Codex
  Task titles in Claude Code  on
  Done
```

Each change is saved as you make it. Unpicking an official agent removes its package.

## One agent

`ask setup NAME` opens one agent: which of its models ask uses, and their cost limits.

```text
? opencode: what do you want to change?
> Models       148 of 148 on
  Cost limits  the default for every model
  Done
```

Models is a list of everything the agent offers, with the ones that are on checked. Type to
filter it, space to turn one on or off, ctrl-a for everything shown. The ones you turn off go to
[models.json](models.md#modelsjson). Cost limits sets a model's own limit, which replaces your
default for its runs. The settings screen reaches the same place through Models and cost limits.

## For scripts and agents

Flags change things without asking, and `--check` reports:

```sh
ask setup --agents claude,codex -m claude:sonnet-5.5 --skills all --hook
ask setup --worktrees '~/code/worktrees/ask-{name}' -t 1800
ask setup -m ""            # back to no default model
ask setup --yes            # the recommended setup: agents for the CLIs found, skills, hook
ask setup --check          # exit 1 while an installed agent can't run
ask setup --check --json
```

| Flag | |
| --- | --- |
| `--agents LIST` | Install these official agents and check them |
| `-m`, `--model M` | The default model |
| `-t`, `--timeout S` | Seconds per task |
| `-j`, `--jobs N` | Batch tasks at once |
| `--max-cost USD` | Dollars a task may spend |
| `--worktrees T` | Where `--worktree` works, with `{name}` |
| `--branches T` | A worktree's branch, with `{name}` |
| `--skills LIST` | Add the ask skill to `claude-code`, `codex`, `opencode`, `cursor`, `pi`, or `all` installed |
| `--hook`, `--no-hook` | Add or remove ask's title hook in Claude Code |
| `--yes` | Apply the recommended setup |
| `--check` | Report; with `--json`, as JSON |

An empty value (`-m ""`) returns a setting to its built-in default. Without a terminal and
without flags, `ask setup` prints the report.

`--check --json` prints `agents` (each with `name`, `cli`, `installed`, `ready`, `models` and
`reason`), `settings`, `skills` (by app: `missing`, `current`, `outdated` or `yours`) and `hook`.

## What it changes

Only what you choose. ask's own files go in `~/.ask`. The skill is `skills/ask/SKILL.md` in each
app's folder, like `~/.claude/skills/ask/SKILL.md`; ask setup updates the ones it wrote and never
replaces a skill you wrote (it marks its own with a comment line; delete that line to keep your
edits). The hook is one entry in `~/.claude/settings.json`; everything else in that file stays as
it is, in the same order.
