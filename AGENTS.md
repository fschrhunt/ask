# ask

Hands tasks to coding agents through local harnesses, one task or a batch in parallel. Node, no
dependencies. ask ships no harness and knows no particular agent.

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
  - `task.js`: one task: worktree, prompt, harness run, what changed, JSON checks.
  - `harness.js`: finding harnesses in `~/.ask/harnesses`, listing their models, running one under
    the contract, model names.
  - `git.js`: what a write run changed, and worktrees.
  - `process.js`: spawning in a process group, timeouts, stopping everything on SIGINT/SIGTERM.
  - `status.js`: status lines and the `ask runs` table.
  - `schema.js`: the `--schema` check.
  - `home.js`: `~/.ask` paths, `models.json`, atomic JSON writes, `UsageError`.
- `test/`: `cli`, `batch`, `subagent` (follow-ups, changes, worktrees, show, stop) and `local`
  (the harness contract, with shell harnesses) tests, `helpers.js`, and `fake`, the fake harness
  every test installs. `FAKE_LOG` records runs; `FAKE_FAIL`, `FAKE_HANG` and `FAKE_SLOW` match the
  prompt; `FAKE_WRITE` and `FAKE_COMMIT` change the repository.
- `docs/`: user docs with examples. Update them with any user-visible change.
- `assets/`: the logo, wordmark and lockup SVGs in black and white; see `assets/README.md`.

## Conventions

- Fewest moving parts. No dependencies, no configuration beyond what a change needs.
- Nothing about a particular agent goes in this repository; it belongs in a user's harness. The
  harness contract in `docs/harnesses.md` changes only with care: every harness depends on it.
- Comments state purpose and contract, on modules and functions; none line by line. Update the
  comments and docs a change touches.
- One test per behavior change. Never call a network or a real model in a test.
- A user-visible change gets a `CHANGELOG.md` entry under Unreleased.
