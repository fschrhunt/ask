# Contributing

Smallness is the point: no dependencies, and nothing about a particular agent outside its harness.
A change that makes ask simpler is welcome; one that adds a moving part needs a strong reason.
`AGENTS.md` has the code map.

- `npm test` runs the tests. They are offline and use fake `claude`, `codex` and `opencode`
  programs from `test/bin`, so no model is called.
- A behavior change comes with one test for it, and a user-visible change with a line in
  `CHANGELOG.md` and an update to `docs/`.
- Support for a new agent is usually best as your own harness in `~/.ask/harnesses`
  (see `docs/harnesses.md`); propose shipping it once it is solid.
- To report an issue, include the output of `ask --help` and `ask models`.

By contributing you agree that your work is licensed under the MIT license of this project.
