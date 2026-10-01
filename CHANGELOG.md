# ask releases

## Unreleased

- Brand assets: the ask logo, wordmark and lockup in black and white under `assets/`, and a
  rewritten README with the lockup header.
- `ask runs` shows an unfinished run whose ask process is gone as stopped, with the command that
  resumes it.
- A run that reports no usage (Opencode, for an answer that needed no steps) prints none instead of
  `0 in 0 out`.
- First public release: `ask -m MODEL PROMPT`, `ask batch` with `--resume`, `ask runs` and
  `ask models` across Claude Code, Codex and Opencode.
- `models.json` is optional; without it there are no Opencode models.
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
