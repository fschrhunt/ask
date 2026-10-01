# ask

Hands tasks to Claude Code, Codex and Opencode, one task or a batch in parallel.
One Node file, no dependencies.

## Commands

- `npm test`: the whole suite; hermetic and offline.
- `node ask --help`: the usage text, which is also the contract for flags and output.

## Code map

- `ask`: the program. Reading order: `HELP` and constants, usage and model helpers, `run` (spawn
  with timeout), `opencodeConfig` and `strictSchema`, `harnesses` (one adapter each for Opencode,
  Codex and Claude), `runTask`, argument parsing, `runBatch` and `listRuns`, `main`.
- `test/ask.mjs`: the tests. They run `ask` against the fakes in `test/bin`.
- `test/bin/`: fake `claude`, `codex` and `opencode` printing canned output in each harness's real
  format. `FAKE_LOG` records calls; `FAKE_FAIL`, `FAKE_HANG` and `FAKE_SLOW` match the prompt.
- `models.json`: a sample of `~/.ask/models.json`.
- `assets/`: the logo, wordmark and lockup SVGs in black and white; see `assets/README.md`.

## Conventions

- Fewest moving parts. No dependencies, no configuration beyond what a change needs.
- Comments state purpose and contract, on modules and functions; none line by line. Update the
  comments and docs a change touches.
- One test per behavior change. Never call a network or a real model in a test.
- A user-visible change gets a `CHANGELOG.md` entry under Unreleased.
- Name products in prose Claude Code, Codex and Opencode; keep commands and paths literal.
