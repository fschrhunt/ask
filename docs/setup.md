# Setup

`ask setup` sets ask up the first time: it connects the coding agents you have, sets your defaults,
and teaches the apps you work in to use ask. Change anything later with
[`ask settings`](settings.md).

## The first time

In a terminal, `ask setup` walks you through it, then shows everything it will change before it
saves anything:

```text
ask setup  connects your coding agents; nothing else changes until you review it

✔ Which coding agents should ask use? claude, codex
✔ claude is ready: 4 models, see ask models
✔ codex is ready: 8 models, see ask models
✔ Default model, when you don't give -m claude:sonnet-5.5
✔ Where should --worktree runs work? Somewhere else
✔ Folder for each worktree, with {name} ~/code/worktrees/ask-{name}
✔ Teach these apps to use ask Claude Code, Codex
✔ Show ask runs by model and task in Claude Code's task list? Yes

~/.ask/settings.json new  +4
  {
    "model": "claude:sonnet-5.5",
    "worktrees": "~/code/worktrees/ask-{name}"
  }

~/.claude/settings.json  +12
  ...

~/.claude/skills/ask/SKILL.md new  +18
  18 new lines

3 files changed, 34 insertions(+)

? Save these changes? (Y/n)
```

- **Agents** are the official ones for the CLIs it finds, built into ask; they install as you pick
  them and are checked at once.
- **Defaults** go to your [settings](settings.md): the model, and where worktrees go.
- **The ask skill** goes to each app that reads skills and is installed here: Claude Code, Codex,
  Opencode, Cursor and pi. It tells their agents that ask exists and how to hand it work.
- **Task titles** register `ask title --hook` in Claude Code (see [Hosts](hosts.md)).

Saying no saves nothing but the agents. Once ask is set up, `ask setup` opens your settings.

## Without a terminal

```sh
ask setup --yes            # the recommended setup: agents for the CLIs found, skills, task titles
ask setup --check          # report, and exit 1 while an installed agent can't run
ask setup --check --json
```

`--yes` shows what it changes as it saves. `--check --json` prints `agents` (each with `name`,
`cli`, `installed`, `ready`, `models` and `reason`), `settings`, `skills` (by app: `missing`,
`current`, `outdated` or `yours`) and `hook`. Without a terminal or options, `ask setup` prints the
report.

## What it changes

Only what you choose. ask's own files go in `~/.ask`. The skill is `skills/ask/SKILL.md` in each
app's folder, like `~/.claude/skills/ask/SKILL.md`; ask updates the ones it wrote and never
replaces a skill you wrote (it marks its own with a comment line; delete that line to keep your
edits). The hook is one entry in `~/.claude/settings.json`; everything else in that file stays as
it is, in the same order.
