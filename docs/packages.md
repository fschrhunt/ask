# Packages

A package is a git repository shaped like `~/.ask`: any of `agents/`, `hooks/` and `commands/`.
Packages are how people share what they built for ask and keep it up to date, without forking ask
or copying files by hand.

```text
ask-claude/
├── agents/     claude
└── lib/        what the agent runs
```

## Official agents

ask ships no agents, but there is an official package for each common coding agent CLI. A bare
name installs it:

```sh
ask install claude                # fschrhunt/ask-claude: Claude Code
ask install codex opencode        # fschrhunt/ask-codex and fschrhunt/ask-opencode
```

```text
ask: installed github.com/fschrhunt/ask-claude: agents: claude
ask: claude is ready: 4 models, see ask models
```

After installing, ask checks each agent the package brings by asking it for its models. An agent
that can't run says why, like a CLI that isn't installed, and `ask install` exits 1:

```text
ask: codex is not ready: Codex not found: install it from https://developers.openai.com/codex, or set ASK_CODEX_BIN to its path
```

## Install, update, remove

```sh
ask install fschrhunt/ask-claude         # OWNER/REPO on GitHub
ask install https://gitlab.com/team/tools.git
ask install ~/code/my-ask-tools          # a local repository, while you work on it
ask install                              # update every installed package
ask packages                             # what is installed, and what each offers
ask remove ask-claude
```

```text
github.com/fschrhunt/ask-claude  agents: claude
```

`ask install` clones into `~/.ask/packages/HOST/OWNER/REPO`, and updating pulls fast-forward only.
Nothing runs at install time except the readiness check: no scripts, no prompts. A bare NAME
means `fschrhunt/ask-NAME`, unless a folder of that name is here. `OWNER/REPO` always means
GitHub; name a local repository by a path such as `./tools`, `../team/tools` or `~/code/tools`.
Installing a source where another one is already installed, like a second local `team/tools`,
fails: remove the first.

Your own agent can be a package too: put it in `agents/` of a repository and
`ask install you/your-agents`.

## Which one runs

ask looks for an agent, hook or command in `~/.ask` first, then in each package in path order. The
first one with the name wins, so **yours always win**: to change something a package offers, put
your own of the same name in `~/.ask`, and your package updates still apply to everything else.

A package's executables run with your permissions, like any software you install. Install packages
from people you trust, and read what they do.
