# ask

Hands tasks to coding agents, one task or a batch in parallel. Each coding agent is reached through
an agent: an executable in `~/.ask/agents` or a package. ask's core (`internal/`) knows nothing about
any of them; the official agents live in `packages/` and are built into the binary. Go 1.26 or
newer, standard library only. ask runs as one binary; agents may use any language (the official
ones use Node.js 18+, standard library only).

## Commands

- `go test ./...`: the whole suite; hermetic and offline.
- `go test ./test -run 'TestCLI/no_arguments'`: one named behavior.
- `gofmt -l .`: must be empty; `go vet ./...`: must pass.
- `go build -o ask ./cmd/ask`: build the binary; `./ask --help` is the contract for flags and output.
- `cd packages/claude && node --test`: one official agent's tests, against a fake CLI in `test/bin`.

## Code map

- `cmd/ask/main.go`: entry point only; calls `internal/cli` and holds the release version.
- `internal/`: ask itself, which knows no particular agent. Packages by purpose:
  - `cli`: help text, options per command, the commands, exit codes.
  - `runs`: runs on disk (ids, `RUN/TASK` references, lock, results), preparing tasks
    (validation, follow-ups), running them in parallel, stopping.
  - `task`: one task: worktree, prompt, agent run, what changed, JSON checks.
  - `agent`: finding agents, listing their models, running one under the contract, model
    names.
  - `hooks`: running hooks before and after each task; they fail open.
  - `packages`: installing, updating and removing packages from git; official agent names.
  - `find`: finding agents, hooks and commands in `~/.ask`, then in packages; yours win.
  - `git`: what a write run changed, and worktrees.
  - `process`: process groups, timeouts, stopping everything on SIGINT/SIGTERM; Linux parent-death signals.
  - `status`: status lines and the `ask runs` table.
  - `setup`: what `ask setup` offers: official agents and their CLIs, the ask skill per app,
    Claude Code's title hook.
  - `tui`: gh-style terminal prompts (confirm, select, multi-select, input) and raw mode.
  - `schema`: the `--schema` check.
  - `home`: `~/.ask` paths, the contract environment, `models.json` and `settings.json`, atomic JSON writes,
    typed JSON records and readable encoding, `UsageError`.
- `test/`: `cli`, `batch`, `subagent` (follow-ups, changes, worktrees, show, stop), `extend`
  (hooks, commands, packages), `compat` (released run records, pinned in `fixtures/`), `contracts` (raw JSON, streams and process guarantees) and `local`
  (the agent contract, with shell agents) black-box Go tests, `helpers_test.go`, and `fake/`,
  the Go fake agent every test installs. `TestMain` builds ask and fake once. `FAKE_LOG` records
  runs; `FAKE_FAIL`, `FAKE_HANG`, `FAKE_SLOW` and `FAKE_SPEND` (reports $3 spent, then waits) match the prompt; `FAKE_WRITE` and `FAKE_COMMIT`
  change the repository.
- `packages/`: the official agent packages, `claude`, `codex` and `opencode`, each `agents/NAME`
  (a sh launcher that finds Node.js), `lib/NAME.mjs` (the agent), `test/` and `README.md`.
  `packages.go` embeds them; `ask install NAME` writes one to `~/.ask/packages/ask/packages/NAME`,
  and ask rewrites an installed copy whenever its build differs.
- `docs/`: user docs with examples. Update them with any user-visible change.
- `assets/`: the logo, wordmark and lockup SVGs in black and white; see `assets/README.md`.

## Conventions

- Fewest moving parts. No dependencies, no configuration beyond what a change needs.
- Nothing about a particular agent goes in `internal/`; it belongs in an agent, official ones in
  `packages/`. An official agent uses only the contract any agent could. The agent, hook and
  command contracts change only as `docs/compatibility.md` says: additions are free, a breaking
  change bumps `ASK_CONTRACT`. Files in `test/fixtures/` are never rewritten.
- Grow by need: a new hook event or contract field only when a real use needs it.
- Comments state purpose and contract, on packages and functions; exported identifiers have doc comments.
  No line-by-line comments. Update the
  comments and docs a change touches.
- One test per behavior change. Never call a network or a real model in a test.
- A user-visible change gets a `CHANGELOG.md` entry under Unreleased.
