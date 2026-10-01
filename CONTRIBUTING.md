# Contributing

Smallness is the point: no dependencies, and nothing about any particular agent.
A change that makes ask simpler is welcome; one that adds a moving part needs a strong reason.
`AGENTS.md` has the code map.

- `npm test` runs the tests. They are offline and run ask against `test/fake`, a fake harness,
  so no model is called.
- A behavior change comes with one test for it, and a user-visible change with a line in
  `CHANGELOG.md` and an update to `docs/`.
- Support for an agent belongs in a harness in `~/.ask/harnesses`, not in ask (see
  `docs/harnesses.md`).
- To report an issue, include the output of `ask --help` and `ask models`.

By contributing you agree that your work is licensed under the MIT license of this project.
