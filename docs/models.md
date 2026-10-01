# Models

A model id is `harness:id`, with an optional `#effort`:

```text
claude:opus
codex:gpt-6.1-sol#high
opencode:opencode-go/glm-5.3-flash
```

- **harness** is the program ask runs: `claude`, `codex`, `opencode`, or one of yours (see
  [Harnesses](harnesses.md)).
- **id** is the model as that harness names it.
- **effort** is passed to the harness, which passes it on in its CLI's own form (`--effort high`
  for Claude Code, `model_reasoning_effort` for Codex, a model variant for Opencode).

## Listing

```sh
ask models
```

Each harness lists its own models:

| Harness | Models it lists |
| --- | --- |
| `claude` | Claude Code's aliases: `fable`, `opus`, `sonnet`, `haiku` |
| `codex` | Codex's model cache (`~/.codex/models_cache.json`, or `$CODEX_HOME`), without review models |
| `opencode` | None: Opencode offers hundreds, so you name the ones you use |

## Adding model ids

`~/.ask/models.json` adds ids to any harness's list, by harness name:

```json
{
  "opencode": ["opencode-go/glm-5.3-flash", "opencode-go/deepseek-v4.1-flash"],
  "codex": ["gpt-6-luna"]
}
```

The file is optional, but if it exists it must be valid JSON. ask still runs any id you give it,
listed or not. The list is for you and for agents that pick from `ask models`.

## Names in output

Status lines and results use the model's own name, not its id:

| You ran | ask shows |
| --- | --- |
| `claude:opus` | `Opus 5.5`, the model the alias resolved to |
| `codex:gpt-6.1-sol#high` | `GPT-6.1 Sol (high)`, Codex's display name |
| `opencode:opencode-go/glm-5.3-flash` | `Glm 5.3 Flash`, the id in title case |

A harness gives the name in its report, or next to the id when it lists its models; otherwise ask
title-cases the id.
