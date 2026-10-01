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
  - `cli.js`: help text, flags, commands, exit codes.
  - `task.js`: one task: the prompt ask sends, the run, and the JSON and schema checks.
  - `batch.js`: parallel batches, recorded runs, `--resume`, `ask runs`.
  - `harness.js`: finding harnesses in `~/.ask/harnesses`, listing their models, running one
    under the contract, model names.
  - `process.js`: spawning in a process group, timeouts, stopping everything on SIGINT/SIGTERM.
  - `home.js`: `~/.ask` paths, `models.json`, `UsageError`.
  - `usage.js`: token and cost totals.
- `test/`: `cli`, `batch` and `local` (the harness contract, with shell harnesses) tests,
  `helpers.js`, and `fake`, the fake harness every test installs. `FAKE_LOG` records runs;
  `FAKE_FAIL`, `FAKE_HANG` and `FAKE_SLOW` match the prompt.
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
