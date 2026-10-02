# Contributing

Smallness is the point: no dependencies, and nothing about any particular agent.
A change that makes ask simpler is welcome; one that adds a moving part needs a strong reason.
`AGENTS.md` has the code map.

- Go 1.26 or newer, git and a POSIX shell are needed for development. `go test ./...` builds ask
  and `test/fake`, a Go fake agent, once in `TestMain` and runs the binary in temporary homes.
  The tests are offline; no model is called.
- `gofmt -l .` should be empty, and `go vet ./...` should pass. Build with
  `go build -o ask ./cmd/ask`; use `go test ./test -run 'TestCLI/no_arguments'` for one behavior.
- A behavior change comes with one test for it, and a user-visible change with a line in
  `CHANGELOG.md` and an update to `docs/`.
- Support for a coding agent belongs in its own agent executable in `~/.ask/agents`, not in ask (see
  `docs/agents.md`).
- To report an issue, include the output of `ask --help` and `ask models`.

By contributing you agree that your work is licensed under the MIT license of this project.
