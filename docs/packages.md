# Packages

A package is a git repository shaped like `~/.ask`: any of `agents/`, `hooks/` and `commands/`.
Packages are how people share what they built for ask and keep it up to date, without forking ask
or copying files by hand.

```text
team-tools/
├── agents/     my-cli
├── hooks/      verify
└── commands/   review
```

## Official agents

ask comes with an official package for each common coding agent CLI, in
[`packages/`](../packages) of its repository and built into the binary. A bare name installs one,
without the network:

```sh
ask install claude                # Claude Code
ask install codex opencode        # Codex and Opencode
```

```text
ask: installed ask/packages/claude: agents: claude
ask: claude is ready: 4 models, see ask models
```

An installed official package always matches the ask that runs it: when ask updates, its
packages update with it. `ask remove claude` removes one.

Installing these packages does not install their CLIs or Node.js. The official agents need
Node.js 18 or newer and an authenticated CLI (Opencode needs a connected provider); see
[Install](install.md#set-up).

After installing, ask checks each agent the package brings by asking it for its models. An agent
that can't run says why, like a CLI that isn't installed, and `ask install` exits 1:

```text
ask: codex is not ready: Codex not found: install it from https://developers.openai.com/codex, or set ASK_CODEX_BIN to its path
```

## Install, update, remove

```sh
ask install team/tools                   # OWNER/REPO on GitHub
ask install https://gitlab.com/team/tools.git
ask install ~/code/my-ask-tools          # a local repository, while you work on it
ask install                              # update every installed package
ask packages                             # what is installed, and what each offers
ask remove tools
```

```text
ask/packages/claude         agents: claude
github.com/team/tools       agents: my-cli · commands: review · hooks: verify
```

`ask install` clones into `~/.ask/packages/HOST/OWNER/REPO`, and updating pulls fast-forward only;
official packages go to `~/.ask/packages/ask/packages/NAME`. Nothing runs at install time except
the readiness check: no scripts, no prompts. A bare NAME always means an official package and
`OWNER/REPO` always means GitHub, whatever folders are here; name a local repository by a path
such as `./tools`, `../team/tools` or `~/code/tools`.
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
