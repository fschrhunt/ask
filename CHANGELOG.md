# ask releases

## Unreleased

- `ask settings`: every setting in one place. In a terminal, a menu of Agents (each agent's models,
  cost limits, install and remove; `ask settings NAME` opens one), Defaults, Worktrees and Apps.
  Nothing is written while you edit; Review and save shows each file's changes as a diff, then
  asks. For scripts, `ask settings get [KEY]`, `set KEY VALUE` and `unset KEY` show the same
  review, with `--dry-run` and `--json`. `ask models` changes show the diff too.
- `ask setup` is the first run: a walkthrough that ends with that review, `--yes` and `--check`.
  Its setting flags moved to `ask settings set`.
- `ask docs`: the docs, built in. `ask docs PAGE` (a prefix is enough), `--search TERM`, `--raw`
  and `--url`; the official agents' READMEs are pages too. Help footers name their page.
- The terminal look is black, white and greys; color marks only diffs and outcomes.
- Turn models on and off: `ask models MODEL... --enable|--disable`, or `ask settings NAME` for a
  filterable list of everything an agent offers. Off models are hidden from `ask models` and help
  and refused by `-m`; `ask models --all` shows both. `models.json` keeps only your choices per
  agent: `false` for off, `true` to add a model, `{"enabled", "max_cost"}` for a model's own cost
  limit; the older list form still works.
- The Opencode agent lists every model of the providers you have set up in Opencode, instead of
  none. Help shows a count for an agent with more than a dozen models.
- Cost limits, off unless set: the `max_cost` setting (`ask settings set max_cost 2`), a model's own
  limit (`ask models claude:opus-5.5 --max-cost 10`) and `--max-cost` per run or batch, which wins.
  Agents get the limit in `ASK_MAX_COST`; ask stops an agent whose reported cost passes it, keeping
  its session for `ask -c`. The Claude Code agent hands it to Claude Code's `--max-budget-usd`,
  since Claude Code reports cost only when it ends; Codex reports no cost, so limits skip it.
- Lists in `ask setup` and `ask settings` filter as you type; ctrl-a selects everything shown.
- `ask wait RUN...` blocks until runs finish, then prints them like `ask show` (several runs as one
  JSON array); `-t` gives up after that many seconds. The ask skill tells agents to collect their
  background runs with it.
- `ask clean` removes worktrees whose work has landed (merged, squash-merged or unchanged) with
  their branches, and run records older than `--days` (30), after showing the plan; `--dry-run`
  and `--yes` for scripts.
- `ask --version` reports the module version for `go install` builds instead of `dev`.
- `ask help` marks a command of yours that one of ask's own commands hides.
- The official agents for Claude Code, Codex and Opencode live in ask's repository, in
  `packages/`, and are built into the binary: `ask install claude` installs without the network,
  to `~/.ask/packages/ask/packages/claude`, and an installed official agent is rewritten whenever
  ask updates, so it always matches. A bare name that isn't an official agent is now an error.
  The Opencode agent counts the usage of the step that writes the answer, which Opencode 2.0
  no longer prints.
- `ask setup`: in a terminal, a walkthrough the first time (agents for the CLIs it finds, a default
  model, where worktrees go, the ask skill for Claude Code, Codex, Opencode, Cursor and pi, and
  task titles in Claude Code); `--yes` applies the recommended setup and `--check` (with `--json`)
  reports. ask points to it wherever no agent is set up.
- `ask install NAME` always means the official agent, even beside a folder of that name; name a
  local repository by a path. One-letter words no longer end up in run names (`ask's` was `ask-s`).
- `ask install claude`, `codex` or `opencode` installs the official agent package for that CLI
  and `ask install` takes several sources at once. After installing, ask
  checks every agent a package brings and says whether it is ready, or why not, like a missing
  CLI; it exits 1 when one is not ready.
- `~/.ask/settings.json` holds your defaults: `model` (so `-m` is optional), `timeout`, `jobs`,
  `worktrees` (where `--worktree` works, like `~/code/worktrees/ask-{name}`) and `branches`
  (`ask/{name}` by default). Unknown keys and bad values are errors that name the key.
- `ask runs` lists the runs of the repository you are in, from any of its checkouts; `--all`
  lists every run.
- ask passes prompts to agents exactly as given. The read-run instruction to search and read the
  files first moved into the official agents; your own agents can add their own.
- `ask title --hook` answers Claude Code's `PreToolUse` hook directly: register
  `ask title --hook` as the command, with no script or `jq`.
- The agents page no longer suggests syncing agents with dotfiles; packages are how agents move
  between machines.
- Host titles drop a model the description repeats, so `Sonnet 5.5 · Mine sessions` titles as
  `Sonnet 5.5 · Mine sessions`, not `Sonnet 5.5 · Sonnet 5.5 · Mine sessions`.
- `ask install` refuses sources whose host, owner or repository is `.`, `..` or starts with `-`,
  and SCP-style sources whose user part hides another host, so a package always lands in
  `~/.ask/packages/HOST/OWNER/REPO`. `OWNER/REPO` always means GitHub, even beside a folder of that
  name; installing a different source where a package is installed fails instead of updating it.
  Installing the same source again updates it even when a git `insteadOf` rule rewrites its URL,
  and a local folder whose name is only dots and `.git` (like `...git`) is refused.
- A task hook's `refuse` or a result hook's `fail` that is not a string is noted instead of
  silently ignored.
- Follow-ups and runs fixed: `-c RUN --worktree` (or `"worktree": true` with `"continue"`) is
  refused for a run in your checkout instead of writing there; a batch task's follow-up never takes
  the batch's or another run's name; `-c` accepts a single run's folder and says when a run has
  not finished; a timeout above 2000000 seconds says so. A follow-up of a worktree task whose
  worktree was removed claims a fresh one instead of running in another run's worktree of that
  name. `ask show DIR` of a single run's folder prints its answer, like `ask show RUN`.
- Worktrees: parallel runs with the same name each claim their own worktree and branch, and task
  ids with dots (`v1..v2`, `api.lock`) give valid branch names. ask's own git calls ignore a
  `core.fsmonitor` a write run sets.
- `ask stop` fails when the run is still going after 10 seconds instead of saying `stopped`.
  Checking whether a run is running no longer takes its lock, so `ask runs` or `ask stop` can't
  make a concurrent `--resume` fail; the lock file no longer holds a pid, as the kernel reports it.
  A run started outside a sandbox or container shows as running inside one, and `ask stop` there
  says it cannot signal it. Usage sums keep only the counts an agent reported, so a cost-only
  agent's records hold no zero token counts.
  **Before upgrading, let running asks finish or stop them:** the run lock moved from `flock` to
  `fcntl` locks, and older and newer binaries do not see each other's locks.
- Stopping a batch (Ctrl-C or `ask stop`) no longer prints `[null]` results or a `0/1 ok` summary;
  it ends with the stopped line, like a single run. A batch's usage total leaves out cost when
  only some tasks reported one. A JSON line that does not parse is reported by its line in the
  input, counting leading blank lines, and `-t` above 2000000 seconds is rejected by name.
||||||| bb4d816
- Host titles read batch tasks from heredocs: piped to `ask batch -`, or written by `cat > FILE`
  earlier in the same command, so a batch started that way is titled `Batch of N · Model · …`.
- The live view shows tokens and cost while agents run, and a batch's footer keeps a running
  total. ask reads an agent's report as the agent rewrites it; agents that report usage only at
  the end show it at the end, as before. Plain status lines are unchanged.
- Runs are named after their prompt, like `login-test-fail`, and the name works wherever a run id
  does: `ask -c login-test-fail`, `ask show`, `ask stop`, `--resume`, and worktree branches
  (`ask/login-test-fail`). Every run keeps its fixed id too. A follow-up takes over its
  conversation's name, so the name reaches the latest turn. A new `name` hook event lets a hook
  choose names, for example with a fast model; the hooks page has one. `ask runs` adds an ID
  column. Runs from before names keep their ids.
- Docs examples are real and tested: a `verify` hook that runs any project's tests and has the
  agent fix failures, a `context` hook that tells agents where you are in git, a batch built from
  GitHub issues, the Claude Code title hook, and real sample output from runs and follow-ups.
- Docs use real agents and models throughout (Claude Code, Codex, Opencode) instead of made-up ones,
  and the agents page has two tested Claude Code agents: a minimal one in shell, and one in Node
  that reports sessions for `ask -c` and usage.
- Built with Go 1.27; building from source needs Go 1.26 or newer.
- Preserve caller stdin for argument prompts; keep worktrees whenever git sees changes or a
  moved HEAD, and report mode, symlink and newline-named file changes. Worktrees preserve the
  caller's subdirectory through symlinked paths, and batch tasks always get distinct branches.
- Guard runs with an operating system lock, keep interrupted task slots empty, and write private
  run records through unique temporary files. Read-only follow-ups can use kept worktrees;
  follow-ups inherit the model and access selected by task hooks.
- User commands replace ask so signals and exit status reach them directly. Bound output-pipe
  waits after agent exit, clamp long timeouts, and describe signal deaths by signal name.
- Match host titles to ask's command and batch-file parsing; failed saved batches exit 1.
  Plain status removes control characters, empty task records are skipped, and terminal batch
  views show total usage. Terminal width falls back to a positive `COLUMNS` value.


- One compact help overview for `ask`, `ask help` and help flags; every command has a short
  `--help` page, and `-h` works throughout. `-V` aliases `--version`; near-miss options
  suggest a correction. Existing public names remain stable for scripts and run records.
- Status lines put the outcome directly after the run id, use display names at start,
  compact times and cents for costs of at least $0.01. Live rows use marks, aligned batch
  tasks and notes beneath each task; non-UTF-8 locales use ASCII marks. `ask runs` shows
  relative times and hides empty columns. Terminal `ask models` groups display names;
  piped output keeps one id per line. `ask packages` uses aligned columns. The start-line
  label `continues` is now `follow-up`; the `-c` option and JSON `continue` field stay.
- Live stderr terminals show a spinner or batch task rows with elapsed time, clipped to the
  screen, with minimal color (`NO_COLOR` disables it). Hook notes appear beneath their task;
  answers print after it finishes. Pipes and `TERM=dumb` keep plain status lines. Ctrl-C
  restores the cursor and leaves the stopped state visible.
- Replace Node-compatible JSON parsing/serialization with `encoding/json`, typed records and
  reports, and raw JSON answers/schema payloads. Released records still load and continue;
  readable JSON preserves field order and does not escape HTML. Parse errors now use Go wording.


- Compact help includes local models grouped by agent. Add `ask help COMMAND` and
  `ask models --names`; usage errors give one fix and a topic pointer, with grouped models
  for missing or invalid model specifications.

- Add `ask title --command STRING [--description TEXT]` for host background titles, with
  literal shell parsing, model names, follow-up/batch titles and fail-open `title` hooks.

- Honor false schemas and `items: false`; retain earlier worktree changes across idle follow-ups;
  finish promptly when descendants retain agent output pipes; keep all hook start failures
  fail-open with notes; report the model and access after task hooks; reject surplus command arguments.

- Rewritten in Go: ask is one binary, with no Node requirement. Install a release binary,
  use `go install github.com/fschrhunt/ask/cmd/ask@latest`, or build from source. Agents, hooks
  and commands may still be written in any language. Existing contracts and run records are
  unchanged. `ask --version` prints the release tag, or `dev` for a source build.
- Make ask yours. Hooks in `~/.ask/hooks` change tasks before they run and check results after:
  refuse, fail, leave a note, or ask the same agent for a follow-up (up to three). They fail open,
  and `--no-hooks` skips them. Commands in `~/.ask/commands` run as `ask NAME`. Packages share all
  of these from git: `ask install`, `ask packages`, `ask remove`; yours always win.
- Every agent, hook and command gets `ASK_CONTRACT` (now 1), `ASK_BIN` and `ASK_HOME`;
  `docs/compatibility.md` says what stays stable and how it may change.
- A process that crashes reports its error line, not the runtime's closing banner.
- Harnesses are now called agents: they live in `~/.ask/agents/`, a model is `agent:id`, and the
  contract is in `docs/agents.md`. Move `~/.ask/harnesses` to `~/.ask/agents`.
- Docs: a model naming convention for harnesses (lowercase family-version-variant, no aliases or
  provider prefixes), and every example follows it.
- Runs are subagents you can come back to. Every run gets a short id, shown at the start of its
  status lines and recorded in `~/.ask/runs`; batch tasks are `RUN/TASK`.
  - `ask -c RUN PROMPT` continues the agent's own conversation, where it ran, keeping its model
    and access unless given new ones. Batch tasks can continue runs with `"continue"`.
  - `--worktree` runs a write task in its own git worktree and branch, kept only if it changed
    something.
  - Write runs in a git repository report the files they changed and the commits they made.
  - `ask show RUN [--json]`, `ask stop RUN`, and an `ask runs` table with status, model and task.
- Harness contract: `ASK_SESSION` in, `"session"` in the report out. ask reads the report even
  after a failure or timeout. Contract variables inherited from an outer ask are cleared.
- Status lines start when a run starts, and read `ask RUN · outcome · model · time · changes ·
  usage`.
- Records survive interruptions: results are written atomically, a damaged `results.json` is
  reported instead of rerunning finished tasks, and a lock keeps two asks off one run.
- `--resume` takes a run id and refuses options that would change the recorded tasks.
- Options are checked per command; `-t`, `-j` and task fields are validated; `--` ends options;
  a missing file or a prompt-less run in a terminal gives a clear error.
- Schema checks use own properties and compare `enum` and `const` by value.
- Stopping a run also stops whatever its harness left running, and output is decoded as UTF-8
  across chunk boundaries.
- Harnesses are local only: ask ships none and finds them in `~/.ask/harnesses`. The `claude`,
  `codex` and `opencode` harnesses left the repository; keep your own copies there. The docs show
  how to write one.
- `ask models` says where to add a harness when there is none.
- Any agent: each is reached through a harness, an executable with a small contract (prompt on
  stdin, `ASK_MODEL`, `ASK_EFFORT`, `ASK_ACCESS`, `ASK_SCHEMA`, answer on stdout, an optional
  report). See `docs/harnesses.md`.
- `models.json` adds model ids to any harness, keyed by harness name.
- Earlier source installations used a Node entry point; replace that link with the Go binary
  (see `docs/install.md`).
- `--json` and `--schema` runs always tell the agent the answer format in the prompt.
- Docs with examples in `docs/`.
- Brand assets: the ask logo, wordmark and lockup in black and white under `assets/`, and a
  rewritten README with the lockup header.
- `ask runs` shows an unfinished run whose ask process is gone as stopped, with the command that
  resumes it.
- A run that reports no usage (Opencode, for an answer that needed no steps) prints none instead of
  `0 in 0 out`.
- First public release: `ask -m MODEL PROMPT`, `ask batch` with `--resume`, `ask runs` and
  `ask models` across Claude Code, Codex and Opencode.
- `CODEX_HOME` is honored when looking for Codex's model cache.
- An empty batch is a usage error.
- Read runs in Claude Code and Opencode refuse options that write files or run other programs
  (`--output`, `--ext-diff`, `--textconv`, `--pre`, `--hostname-bin`), and commands with quotes,
  backslashes, `$` or `{`. `git grep` is no longer allowed in read runs.
- `--schema` answers are checked against the schema's core keywords; a mismatch fails the run.
- A JSON string answer prints as JSON under `--json` and `--schema`.
- A harness that exits nonzero fails its run, even if it printed an answer.
- A batch task's `"write"` must be a boolean, and defaults to the batch's `-w`.
- Relative `-C` and task `"dir"` values are resolved once, against where ask runs.
- `--resume` matches saved results by task position, so duplicate ids no longer skip failed tasks.
- Codex strict schemas no longer change a property that is named like a schema keyword.
- Stopping ask waits for its models to exit, and kills any still running after 5 seconds.
