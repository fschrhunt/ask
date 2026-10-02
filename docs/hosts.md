# Host titles

A host can label background tasks without guessing the model or parsing ask flags:

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
stdin or unknown tasks get `Batch`. A batch file path is resolved from the invocation
directory, even when `-C` sets the tasks' working directory. Resumes get `Resume RUN`.

Job text comes from the trimmed description, with its first letter capitalized. Without one,
ask uses the first prompt line (including saved follow-up or task-file prompts), clipped to
60 characters with `…`. Models use the same display names as `ask models --names`.

Non-task commands, user commands, help and unparseable commands print nothing. Both cases
exit 0; misuse of `ask title` exits 2. Title hooks can replace the title; `--no-hooks` in the
inner invocation skips them. Only model-listing agent processes can start.

## Claude Code PreToolUse

See Claude Code's [hook reference](https://code.claude.com/docs/en/hooks#pretooluse-decision-control)
for the host output contract. Configure a `PreToolUse` command hook matching `Bash`. The hook below reads Claude's input,
asks for a title and returns `updatedInput` only when one is available:

```sh
#!/bin/sh
# ~/.claude/hooks/ask-title: name every ask run in Claude Code's task list and run it in the background.
input=$(cat)
case "$input" in *ask*) ;; *) exit 0 ;; esac   # most Bash calls never mention ask
command=$(printf '%s' "$input" | jq -r '.tool_input.command // ""')
description=$(printf '%s' "$input" | jq -r '.tool_input.description // ""')
title=$(ask title --command "$command" --description "$description")
[ -n "$title" ] || exit 0
printf '%s' "$input" | jq --arg title "$title" '{
  hookSpecificOutput: {
    hookEventName: "PreToolUse",
    updatedInput: (.tool_input + {description: $title, run_in_background: true})
  }
}'
```

The original command stays in `updatedInput`; the host description becomes the title and the
command runs in the background. See [Hooks](hooks.md) for ask's own `title` event.

Save it as `~/.claude/hooks/ask-title`, make it executable, and register it in
`~/.claude/settings.json` with its full path (it needs `jq`):

```json
{
  "hooks": {
    "PreToolUse": [
      { "matcher": "Bash", "hooks": [{ "type": "command", "command": "/Users/you/.claude/hooks/ask-title" }] }
    ]
  }
}
```

From then on an agent's `ask -m claude:sonnet-5.5 -w "Fix the login test"` shows in Claude Code's
task list as `Sonnet 5.5 · Fix the login test · write`, running in the background, whatever
description the agent wrote. Other Bash calls are left alone.
