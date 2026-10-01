package cli

// helpText is the byte-compatible command and option reference.
const helpText = `ask: hand tasks to coding agents and get their answers back.

  ask -m MODEL [options] PROMPT    run one task (PROMPT may be - or piped on stdin)
  ask -c RUN [options] PROMPT      continue that run's agent with a follow-up
  ask batch [options] FILE|-       run a batch of tasks in parallel; prints a JSON array
  ask batch --resume RUN [-j N]    rerun the tasks of a batch that did not finish ok
  ask show RUN [--json]            print a run's answer again (--json: its whole result)
  ask runs [-n N]                  list recent runs
  ask stop RUN                     stop a run that is going
  ask models                       list the model ids available here
  ask install [SOURCE]             install a package from git (OWNER/REPO, a URL or a path),
                                   or update every package
  ask packages                     list installed packages and what they offer
  ask remove PACKAGE               remove a package
  ask COMMAND [ARGS]               run one of your commands (~/.ask/commands)

options
  -m, --model ID      agent:id[#effort] from ` + "`" + `ask models` + "`" + `, e.g. mycli:atlas-2.1#high
  -r, --read          read only (the default)
  -w, --write         read and write: may edit files and run commands
  --worktree          with -w: work in a new git worktree and branch, kept only if changed
  -c, --continue RUN  continue RUN's agent session, where it ran (RUN/TASK for a batch task)
  --json              the answer must be JSON
  --schema FILE       the answer must match this JSON Schema (implies --json)
  -C, --dir DIR       directory the agent works in (default: current)
  -t, --timeout S     seconds per task (default: 900)
  -j N                tasks at once in a batch (default: 4)
  --no-hooks          run without your hooks (~/.ask/hooks)

Every run gets an id, shown in its status lines as "ask RUN · ...". A batch task is RUN/TASK.
Batch tasks: a JSON array or one JSON object per line, each
  {"id", "prompt", "model", "write", "worktree", "continue", "json", "schema", "dir", "timeout"}
with "prompt" required, and "model" unless batch -m gives one or the task continues a run.
Results: {"run", "id", "model", "name", "ok", "answer" | "error", "seconds", "usage", "session",
"dir", "changes", "commits", "worktree"}; changes are the files a write run changed.

Make ask yours with executables in ~/.ask ($ASK_HOME), or from packages: agents/ run each coding
agent's CLI, hooks/ change tasks and check results, commands/ add commands. Runs are kept in
~/.ask/runs. Docs: https://github.com/fschrhunt/ask/tree/main/docs`
