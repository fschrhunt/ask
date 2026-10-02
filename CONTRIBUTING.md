# Contributing

Smallness is the point: no dependencies, and nothing about any particular agent.
A change that makes ask simpler is welcome; one that adds a moving part needs a strong reason.

You need Go 1.26 or newer, git, a POSIX shell, Node.js 18+ for the official agents' tests and
shellcheck for `./x shell`.

```sh
./x hooks     # once: gofmt and vet before each commit
./x check     # before a pull request; CI runs the same command
```

- [AGENTS.md](AGENTS.md): commands, where things live, conventions.
- [docs/contributing/architecture.md](docs/contributing/architecture.md): how a task runs, the
  packages, and the rules the layout keeps.
- [test/README.md](test/README.md): the tests, the fake agent, and which file covers what.
- [docs/contributing/releases.md](docs/contributing/releases.md): cutting a release.

A behavior change comes with one test for it, and a user-visible change with a line in
`CHANGELOG.md` and an update to `docs/`. Support for a coding agent belongs in its own agent, not
in ask (see `docs/agents.md`).

To report an issue, include the output of `ask --help` and `ask models`.

By contributing you agree that your work is licensed under the MIT license of this project.
