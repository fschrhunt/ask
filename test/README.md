# Tests

ask's behavior, tested from the outside: each test runs the real `ask` binary in a temporary home
against a fake agent, and checks what a person or a script would see (stdout, stderr, exit codes,
files on disk). No test calls a model or the network.

```sh
./x test                                    # everything, including internal/ unit tests
./x test ./test -run 'TestRuns'             # one feature
./x test ./test -run 'TestRuns/stop_stops'  # one behavior
```

## One file per feature

Each file holds one `TestX`, a list of subtests that read as plain sentences, and matches the
docs page for that feature.

| File | Covers | Docs |
| --- | --- | --- |
| `run_test.go` | One task: options, prompts, answers, `--json`, `--schema`, timeouts, signals | usage.md |
| `batch_test.go` | `ask batch`: task files, defaults, parallel results, `--resume` | batches.md |
| `runs_test.go` | Saved runs: `runs`, `show`, `wait`, `stop`, `clean`, locks, private state | runs.md |
| `names_test.go` | Run names: from the prompt, taken over by follow-ups, name hooks | runs.md |
| `followups_test.go` | `-c`: which session, model and access a follow-up gets | runs.md |
| `git_test.go` | What write runs report from git, and `--worktree` | usage.md |
| `agents_test.go` | The agent contract: stdin, environment, schema files, reports | agents.md |
| `models_test.go` | `models.json`, models on and off, cost limits | models.md |
| `hooks_test.go` | Task and result hooks: order, refusals, follow-ups, failing open | hooks.md |
| `commands_test.go` | Your own `ask NAME` commands | commands.md |
| `packages_test.go` | `ask install`, `update`, `remove`; official agents; refused sources | packages.md |
| `setup_test.go` | `ask setup`: its report, exit status and flags | setup.md |
| `settings_test.go` | `settings.json` and `ask settings`: review, save, bad values | settings.md |
| `title_test.go` | `ask title` and the host hook | hosts.md |
| `help_test.go` | `--help`, command pages, one-line errors, suggestions | |
| `docs_test.go` | `ask docs`, and that every help footer names a real page | |
| `update_test.go` | `ask update`, against a local stand-in for GitHub releases | install.md |
| `compat_test.go` | Released run records and the contract version | compatibility.md |
| `main_test.go` | `TestMain`, which builds ask and the fake once, and the helpers | |

A new behavior goes in its feature's file as one more `t.Run("what it does", ...)`. A new file is
for a new feature.

## The harness

`fresh(t)` gives a test its own `HOME` and `ASK_HOME` with the fake agent installed as `fake`.
From there:

- `s.ask(args...)` runs ask; `s.run(args, stdin, env)` adds input and environment; `s.start`
  returns the running process for signal tests.
- `s.calls()` lists what the fake was called with (stdin, cwd, model, access, session);
  `s.runFile` and `s.latest` read the saved run.
- `s.repo()` makes a committed git repository; `s.localAgent`, `s.script` and `s.hook` install
  shell agents, commands and hooks.
- `eq`, `match`, `obj`, `objects` and `runID` check output.

## The fake agent

`fake/main.go` speaks the agent contract with no model. It answers `fake: ` plus the prompt's last
line, reports usage of 10 in, 5 out and $0.01, and records every call when `FAKE_LOG` is set (the
harness sets it). Environment variables change what it does:

| Variable | Effect |
| --- | --- |
| `FAKE_ANSWER` | Answer with this text instead |
| `FAKE_FAIL`, `FAKE_HANG`, `FAKE_SLOW`, `FAKE_SPEND` | When the prompt contains the value: fail, hang, take 300ms, or report $3 spent and hang |
| `FAKE_WRITE=file=text` | Write a file in the working directory |
| `FAKE_COMMIT` | Commit everything |
| `FAKE_EXIT` | Exit with this code after answering |
| `FAKE_NOTE`, `FAKE_NO_USAGE` | Report a note, or no usage |
| `FAKE_IGNORE_TERM` | Ignore SIGTERM, so ask has to kill it |

## Fixtures

`fixtures/run-*` are run records written by released versions of ask. `compat_test.go` checks the
current ask can still show, list and continue them. They are never rewritten: a release that
changes the record format adds a new one.
