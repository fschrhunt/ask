# Models

A model id is `agent:id`, with an optional `#effort`:

```text
mycli:atlas-2.1
mycli:atlas-2.1-mini#high
othercli:nova-4
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
mycli:atlas-2.1
mycli:atlas-2.1-mini
othercli:nova-4
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
  "mycli": ["atlas-2.0"],
  "othercli": ["nova-4-mini", "nova-3"]
}
```

The file is optional, but if it exists it must be valid JSON. ask still runs any id you give it,
listed or not. The list is for you and for agents that pick from `ask models`.

## Names in output

Status lines and results use the model's own name, not its id. ask takes the first of:

1. the name the agent reports for the run (the model an alias resolved to, say `Atlas 2.1`);
2. the name the agent lists next to the id (`Atlas 2.1`);
3. the id in title case (`nova-4-mini` becomes `Nova 4 Mini`).

Effort is appended: `Atlas 2.1 (high)`.
