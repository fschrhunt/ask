# Official agents

The agents ask ships for the coding agents most people use. Each folder is an ordinary ask package,
built into the ask binary so `ask install NAME` needs no download:

| Folder | Reaches | Install |
| --- | --- | --- |
| [`claude/`](claude/) | Claude Code (`claude -p`) | `ask install claude` |
| [`codex/`](codex/) | Codex (`codex exec`) | `ask install codex` |
| [`opencode/`](opencode/) | Opencode (`opencode run`) | `ask install opencode` |

Installing these packages connects ask to existing CLIs; it does not install or authenticate
them. All three agents need Node.js 18 or newer. See each package's README for prerequisites.

They get no special treatment: each uses only the agent contract in
[docs/agents.md](../docs/agents.md), the same one anyone's agent uses. ask's core knows nothing
about them; `scripts/guard.sh` checks that.

## Layout

```text
packages/NAME/
├── agents/NAME      a small sh launcher that finds Node.js 18+ and runs lib/NAME.mjs
├── lib/NAME.mjs     the agent: runs the CLI, maps access and models, writes the report
├── test/            node --test, against a fake CLI in test/bin
└── README.md        what a user needs: install, models, read and write
```

`packages.go` embeds `agents/`, `lib/` and `README.md` of each. `ask install NAME` writes them to
`~/.ask/packages/ask/packages/NAME`, and ask rewrites that copy whenever its own build differs, so
an installed agent always matches the ask running it.

## Working on one

```sh
./x node                      # every official agent's tests
cd packages/claude && node --test
```

The agents use Node.js built-ins only. A change to what users see goes in that folder's
`README.md` and in `CHANGELOG.md`.
