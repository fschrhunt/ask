<h1 align="center">ask</h1>
<p align="center">Every coding agent, behind one command.</p>
<p align="center"><a href="https://github.com/fschrhunt/ask/actions/workflows/ci.yml"><img src="https://github.com/fschrhunt/ask/actions/workflows/ci.yml/badge.svg" alt="CI"></a></p>

ask hands tasks to the coding agents you already have — Claude Code, Codex and Opencode — and
returns their answers: one task or many in parallel, read-only or with write access, on whichever
model each agent offers. Each agent runs with its own login, tools and sandbox; ask records every
batch so you can resume it and see what it used. Agents call it to hand work to other agents. It is
one file of Node with no dependencies.

## Install

```sh
git clone https://github.com/fschrhunt/ask ~/.local/share/ask
mkdir -p ~/.local/bin
ln -s ~/.local/share/ask/ask ~/.local/bin/ask
```

You need Node 20 or newer, and whichever of Claude Code, Codex and Opencode you use, installed and
logged in. ask finds `claude`, `codex` and `opencode` on your PATH.

## Use

One task. Every run names its model with `-m`:

```sh
ask -m claude:sonnet "Where is the retry logic, and what are its limits?"
ask -m codex:MODEL#high -C ~/code/app "Review the last commit."
```

Many tasks in parallel. A batch file is a JSON array, or one JSON object per line:

```sh
ask batch -j 4 -m claude:sonnet tasks.json
```

```json
[
  {"id": "api", "prompt": "Summarize the API surface."},
  {"id": "tests", "prompt": "Which tests are flaky?", "model": "codex:MODEL"}
]
```

Resume a batch that was interrupted or had failures. Only unfinished tasks run again:

```sh
ask batch --resume ~/.ask/runs/RUN
```

List recent recorded runs:

```sh
ask runs
```

`--json` requires a JSON answer, `--schema FILE` requires one that matches a JSON Schema, `-C DIR`
sets the working directory and `-t SECONDS` the time limit. ask itself checks each answer against
the schema's `type`, `enum`, `properties`, `required`, `additionalProperties: false` and `items`,
and fails a run whose answer does not match. `ask --help` has the rest.

## Access

- `-r` (the default) is read-only, enforced differently by each agent:
  - Codex runs in its `read-only` OS sandbox. It may run any command, but nothing it runs can
    write files.
  - Claude Code and Opencode have no read-only shell. They get their own file-reading and search
    tools plus an allowlist of inspection commands: `git diff`, `git log`, `git show`,
    `git status`, `git blame`, `git ls-files`, `rg`, `grep`, `ls`, `wc`, `cat`, `head` and `tail`.
    A command must be plain words. Any command containing `>`, `|`, `;`, `&`, `` ` ``, `$`, `{`,
    quotes or a backslash is refused, and so is any command with an option that writes files or
    runs another program: `--output`, `--ext-diff`, `--textconv`, `--pre` (with `--pre-glob`) and
    `--hostname-bin`. `git grep` is not allowed, because its options can be abbreviated and
    bundled past any rule; use the agent's own search.

  Limit: these rules match command text, not what a command does. They close the options above
  and the ways to spell them in the shell, not every behavior configured outside the command.
  For example, git still runs an external diff or textconv filter set in the repository's own
  configuration. In a repository you do not trust, prefer Codex for read runs.
- `-w` reads and writes: the model may edit files and run any command as you. Use it where that is
  acceptable.

## Models

`ask models` lists what is available: the Claude Code aliases, the Codex models from Codex's own
cache, and the Opencode models you list in `~/.ask/models.json`:

```json
{ "opencode": ["provider/model"] }
```

The file is optional; see `models.json` here for a sample. A model id is `harness:id`, with an
optional `#effort` suffix. Set `ASK_HOME` to keep `models.json` and `runs/` somewhere other than
`~/.ask`.

## Output

Answers go to stdout and nothing else does. A single run prints its answer as text, or as JSON
with `--json` or `--schema`. A batch prints a JSON array of
`{id, model, name, ok, answer, error, seconds, usage}`, in task order. `name` is the model's own
name, such as Opus 5.5 or GPT-6.1 Sol.

Status goes to stderr: one line per run, naming the model, with its time and usage. Usage is input, output and
cached tokens, plus the cost in USD when the harness reports one. Exit status is 0 on success, 1
when a run failed and 2 on a usage error. Stopping ask stops every model it started.

## License

MIT
