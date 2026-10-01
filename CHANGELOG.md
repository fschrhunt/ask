# ask releases

## Unreleased

- Harnesses are local only: ask ships none and finds them in `~/.ask/harnesses`. The `claude`,
  `codex` and `opencode` harnesses left the repository; keep your own copies there. The docs show
  how to write one.
- `ask models` says where to add a harness when there is none.
- Any agent: each is reached through a harness, an executable with a small contract (prompt on
  stdin, `ASK_MODEL`, `ASK_EFFORT`, `ASK_ACCESS`, `ASK_SCHEMA`, answer on stdout, an optional
  report). See `docs/harnesses.md`.
- `models.json` adds model ids to any harness, keyed by harness name.
- The entry point moved to `bin/ask`; repoint your link:
  `ln -sf ~/.local/share/ask/bin/ask ~/.local/bin/ask`.
- Node 22 or newer is required (Node 20 has reached end of life).
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
