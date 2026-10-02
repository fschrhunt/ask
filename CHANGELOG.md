# ask releases

## Unreleased

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
