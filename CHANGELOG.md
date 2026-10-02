# ask releases

## Unreleased

- Install with `curl -fsSL https://fschrhunt.com/ask/install.sh | sh` (checksum-verified, into
  `~/.local/bin`) or Homebrew, from ask's own repository: `brew tap fschrhunt/ask
  https://github.com/fschrhunt/ask && brew install ask`.
- `ask update` replaces an installer or archive install with the latest release, after checking
  its checksum; `--check` only reports. Homebrew and go install are pointed to their own update.
  In a terminal, ask mentions a newer release at most once a day (`ASK_NO_UPDATE_CHECK=1` stops it).
- Releases carry build provenance (`gh attestation verify`), run CI's checks and `govulncheck`
  first, and are installed for real on macOS and Linux. `scripts/release.sh` releases in two runs.
- A new README.

## v0.1.0 · 2026-10-02

The first release. ask hands tasks to the coding agents you already use, Claude Code, Codex,
Opencode or your own, as subagents: one task or many in parallel, read-only or with write access,
recorded, with live usage.

**Running work**
- `ask -m MODEL PROMPT` runs a task read-only, `-w` lets it change files, `--worktree` gives it its
  own git worktree and branch. The answer goes to stdout; status lines name the run, model, time,
  what changed, tokens and cost, live in a terminal.
- Runs are named after their prompt (`login-test-fail`) and keep a fixed id. `ask -c RUN` follows
  up in the same conversation; `ask show`, `ask wait`, `ask stop` and `ask runs` (this repository's
  runs, or `--all`) work on them; `ask clean` removes worktrees whose work has landed and old runs.
- `ask batch` runs many tasks in parallel, on one model or several, and resumes a stopped batch.
- `--json` and `--schema` require JSON answers; `-t` bounds time; `--max-cost` bounds spend.

**Agents and models**
- Official agents for Claude Code, Codex and Opencode are built in: `ask install claude`. They find
  their CLI even off `PATH` and say what's missing. Your own agent is any executable under a small
  contract (`ask docs agents`).
- Models are `agent:id`, named plainly (`claude:sonnet-5.5`, `codex:gpt-6.1-sol`). Turn models on
  and off with `ask models` or `ask settings NAME`; `models.json` keeps only your choices.
- Cost limits, off unless set: a default, a model's own, or one run's. ask stops an agent that
  passes its limit and keeps its session, so `ask -c` continues it.

**Setting up and changing things**
- `ask setup` walks through the first run; `ask setup --check` reports.
- `ask settings` holds every control: agents, defaults, worktrees, and the apps that use ask. Edits
  wait in a draft and are saved only after a review that shows each file's diff. `get`, `set` and
  `unset` do the same for scripts.
- `ask docs` has every page built in; `ask help` and `COMMAND --help` give the short form.

**Extending**
- Hooks change tasks and check results (a `verify` hook can run the tests and hand failures back),
  commands add `ask NAME` workflows, and packages share them: `ask install OWNER/REPO`.
- `ask title --hook` names ask runs in Claude Code's task list; the ask skill teaches Claude Code,
  Codex, Opencode, Cursor and pi to hand work to ask.

ask is one Go binary for macOS and Linux with no dependencies. The contracts it keeps across
releases are in `ask docs compatibility`.
