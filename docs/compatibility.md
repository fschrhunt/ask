# Compatibility

What you build on ask keeps working when ask updates. These are the contracts ask keeps, and how
they may change.

## The contracts

| Contract | Where it is described |
| --- | --- |
| The command line: commands, options, exit codes (0 ok, 1 a run failed, 2 a usage error), stdout carrying only answers | `ask --help`, [Usage](usage.md) |
| Status lines start with `ask RUN ·` | [Runs](runs.md) |
| Result fields (`run`, `id`, `model`, `name`, `ok`, `answer`/`error`, `seconds`, `usage`, `session`, `dir`, `changes`, `commits`, `worktree`, `followups`) | [Batches](batches.md) |
| Run records: `tasks.json` and `results.json` in `~/.ask/runs/` | [Runs](runs.md) |
| The agent contract | [Agents](agents.md) |
| The hook contract | [Hooks](hooks.md) |
| The command contract | [Commands](commands.md) |
| The package layout | [Packages](packages.md) |

## How they change

- **Additions don't break anything.** New result fields, report fields, environment variables,
  events and options can appear in any release. Ignore what you don't know.
- **A breaking change bumps the contract version.** Every agent, hook and command gets
  `ASK_CONTRACT`, now `1`. A change that could break one changes that number and is called out in
  the changelog with what to do.
- **Run records stay readable.** Records written by a release are pinned in ask's tests
  (`test/fixtures/`) and never rewritten; newer versions must keep showing and continuing them.
