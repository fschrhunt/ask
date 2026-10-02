# Packages

A package is a git repository shaped like `~/.ask`: any of `agents/`, `hooks/` and `commands/`.
Packages are how people share what they built for ask and keep it up to date, without forking ask
or copying files by hand.

```text
ask-tools/
├── agents/     claude, codex
├── hooks/      verify
└── commands/   review
```

## Install, update, remove

```sh
ask install fschrhunt/ask-tools          # OWNER/REPO on GitHub
ask install https://gitlab.com/team/tools.git
ask install ~/code/my-ask-tools          # a local repository, while you work on it
ask install                              # update every installed package
ask packages                             # what is installed, and what each offers
ask remove ask-tools
```

```text
github.com/fschrhunt/ask-tools	agents: claude, codex · commands: review · hooks: verify
```

`ask install` clones into `~/.ask/packages/HOST/OWNER/REPO`, and updating pulls fast-forward only.
Nothing runs at install time: no scripts, no prompts. `OWNER/REPO` always means GitHub; name a local
repository by a path such as `./tools`, `../team/tools` or `~/code/tools`. Installing a source
where another one is already installed, like a second local `team/tools`, fails: remove the first.

## Which one runs

ask looks for an agent, hook or command in `~/.ask` first, then in each package in path order. The
first one with the name wins, so **yours always win**: to change something a package offers, put
your own of the same name in `~/.ask`, and your package updates still apply to everything else.

A package's executables run with your permissions, like any software you install. Install packages
from people you trust, and read what they do.
