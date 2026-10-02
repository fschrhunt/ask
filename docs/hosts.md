# Host titles

A host, like Claude Code, can label the ask commands its agents run, without guessing the model
or parsing ask flags:

```sh
ask title --command "$command" --description "$description"
```

`--command` is the complete shell command. ask splits literal shell words, quotes, backslash
escapes, assignments, redirects, pipes, `&&`, `||`, semicolons and newlines, then finds the
first simple command whose program basename is `ask`. It never executes the command. Shell
expansions, compound shell syntax and incomplete input return no title.

A task title is `Model · Job`, with effort, and a ` · write` or ` · worktree` suffix.
Follow-ups inherit the recorded model and access unless overridden. A readable batch task file
with known models gets `Batch of N`, plus distinct display names when there are at most three;
unknown tasks get `Batch`. Tasks may also come from a literal heredoc, given to `ask batch -` or
written by an earlier `cat > FILE <<'EOF'` in the same command, since that file doesn't exist yet
when the host asks. A batch file path is resolved from the invocation
directory, even when `-C` sets the tasks' working directory. Resumes get `Resume RUN`.

Job text comes from the trimmed description, with its first letter capitalized; leading
` · ` parts the title already says, like the model in `Sonnet 5.5 · Fix it`, are dropped. Without one,
ask uses the first prompt line (including saved follow-up or task-file prompts), clipped to
60 characters with `…`. Models use the same display names as `ask models --names`.

Non-task commands, user commands, help and unparseable commands print nothing. Both cases
exit 0; misuse of `ask title` exits 2. Title hooks can replace the title; `--no-hooks` in the
inner invocation skips them. Only model-listing agent processes can start.

## Claude Code

Claude Code shows each background command in its task list by the command's description. Let ask
write that description: add a `PreToolUse` hook for `Bash` to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      { "matcher": "Bash", "hooks": [{ "type": "command", "command": "ask title --hook" }] }
    ]
  }
}
```

`ask title --hook` reads Claude Code's hook event on stdin. For a command that runs ask, it answers
with the same tool input, the title as its description, and `run_in_background` set, so every ask
run shows as `Sonnet 5.5 · Fix the login test · write` and runs in the background, whatever the
agent wrote. For any other command it prints nothing and Claude Code carries on. It never blocks
a command: unreadable input prints nothing and exits 0. The original command is never changed.

## Other hosts

Any host that can relabel a shell command before running it can use the same title:

```sh
ask title --command "$command" --description "$description"
```

It prints the title, or nothing when the command isn't an ask task. Hosts that speak Claude Code's
hook format can use `ask title --hook` as it is. Codex accepts that format, but its shell tool has
no description to show, so there is nothing to title there. See [Hooks](hooks.md) for ask's own
`title` event, which can change any title.
