# Models

A model id is `agent:id`, with an optional `#effort`:

```text
claude:sonnet-5.5
claude:haiku-4.5#high
codex:gpt-6.1-sol
```

- **agent** is the executable in `~/.ask/agents/` that ask runs (see
  [Agents](agents.md)).
- **id** is the model as the agent names it. By convention that is the lowercase family,
  version and variant, like `gpt-6.1-sol` or `sonnet-5.5`, never an alias like `latest` (see
  [Naming models](agents.md#naming-models)).
- **effort** is passed to the agent, which passes it on in its CLI's own form.

## Listing

```sh
ask models
```

```text
claude:sonnet-5.5
claude:haiku-4.5
codex:gpt-6.1-sol
```

`ask models --names` prints `agent:id<TAB>Display Name`. `ask --help` groups available ids
on one line per agent, including listing failures.
At a terminal, `ask models` groups each agent's ids with a dim display-name column. In
a pipe it keeps the one-`agent:id`-per-line form for scripts. `NO_COLOR` removes styling.

Each agent lists its own models with `NAME models`. An agent whose CLI offers too many to list
lists none, and you name the ones you use.

## Adding model ids

`~/.ask/models.json` adds ids to any agent's list, by agent name:

```json
{
  "opencode": ["deepseek-4.1-flash", "glm-5.3-flash"],
  "claude": ["sonnet-5"]
}
```

The file is optional, but if it exists it must be valid JSON. ask still runs any id you give it,
listed or not. The list is for you and for agents that pick from `ask models`.

## Names in output

Status lines and results use the model's own name, not its id. ask takes the first of:

1. the name the agent reports for the run (`Opus 5.5`, from what Claude Code says it ran);
2. the name the agent lists next to the id (`GPT-6.1 Sol`, from Codex's model list);
3. the id in title case (`glm-5.3-flash` becomes `Glm 5.3 Flash`).

Effort is appended: `Sonnet 5.5 (high)`.
