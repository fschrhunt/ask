# ask

Hands tasks to coding agents, one task or a batch in parallel. Each coding agent is reached through
an agent: a local executable in `~/.ask/agents`. ask ships none and knows nothing about any of them.
Node, no dependencies.

## Commands

- `npm test`: the whole suite; hermetic and offline.
- `node --test test/cli.test.js`: one file; add `--test-name-pattern=REGEX` for one test.
- `bin/ask --help`: the usage text, which is also the contract for flags and output.

## Code map

- `bin/ask`: the entry point; calls `src/cli.js`.
- `src/`: ask itself, which knows no particular agent.
  - `cli.js`: help text, options per command, the commands, exit codes.
  - `runs.js`: runs on disk (ids, `RUN/TASK` references, lock, results), preparing tasks
    (validation, follow-ups), running them in parallel, stopping.
  - `task.js`: one task: worktree, prompt, agent run, what changed, JSON checks.
  - `agent.js`: finding agents, listing their models, running one under the contract, model
    names.
  - `hooks.js`: running hooks before and after each task; they fail open.
  - `packages.js`: installing, updating and removing packages from git.
  - `find.js`: finding agents, hooks and commands in `~/.ask`, then in packages; yours win.
  - `git.js`: what a write run changed, and worktrees.
  - `process.js`: spawning in a process group, timeouts, stopping everything on SIGINT/SIGTERM.
  - `status.js`: status lines and the `ask runs` table.
  - `schema.js`: the `--schema` check.
  - `home.js`: `~/.ask` paths, the contract environment, `models.json`, atomic JSON writes,
    `UsageError`.
- `test/`: `cli`, `batch`, `subagent` (follow-ups, changes, worktrees, show, stop), `extend`
  (hooks, commands, packages), `compat` (released run records, pinned in `fixtures/`) and `local`
  (the agent contract, with shell agents) tests, `helpers.js`, and `fake`, the fake agent
  every test installs. `FAKE_LOG` records runs; `FAKE_FAIL`, `FAKE_HANG` and `FAKE_SLOW` match the
  prompt; `FAKE_WRITE` and `FAKE_COMMIT` change the repository.
- `docs/`: user docs with examples. Update them with any user-visible change.
- `assets/`: the logo, wordmark and lockup SVGs in black and white; see `assets/README.md`.

## Conventions

- Fewest moving parts. No dependencies, no configuration beyond what a change needs.
- Nothing about a particular agent goes in this repository; it belongs in a user's agent. The
  agent, hook and command contracts change only as `docs/compatibility.md` says: additions are
  free, a breaking change bumps `ASK_CONTRACT`. Files in `test/fixtures/` are never rewritten.
- Grow by need: a new hook event or contract field only when a real use needs it.
- Comments state purpose and contract, on modules and functions; none line by line. Update the
  comments and docs a change touches.
- One test per behavior change. Never call a network or a real model in a test.
- A user-visible change gets a `CHANGELOG.md` entry under Unreleased.
