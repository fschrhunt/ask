# Models

A model id is `agent:id`, with an optional `#effort`:

```text
claude:sonnet-5.5
claude:sonnet-5.5#high
codex:gpt-6.1-sol
```

- **agent** names the executable in `~/.ask/agents/` or an installed package that ask runs (see
  [Agents](agents.md)).
- **id** is the model as the agent names it. By convention that is the lowercase family,
  version and variant, like `gpt-6.1-sol` or `sonnet-5.5`, never an alias like `latest` (see
  [Naming models](agents.md#naming-models)). Ids ignore case: ask lowercases them, so
  `claude:Sonnet-5.5` is `claude:sonnet-5.5`, with its `models.json` entry.
- **effort** is passed to the agent, which passes it on in its CLI's own form. Supported
  values depend on the model and CLI; ask does not validate them.

## Listing

```sh
ask models          # the models that are on
ask models --all    # every model, on or off
```

```text
claude:sonnet-5.5
claude:haiku-4.5
codex:gpt-6.1-sol
```

ask calls each agent with `models` to get its list. The official Claude Code agent has a built-in
list; Codex reads its local model cache; Opencode queries `opencode models`. Listing a model
does not verify that your account can use it. Every model is on until you turn it off.

`ask models --names` prints `agent:id<TAB>Display Name`, and `--all` in a pipe prints
`agent:id<TAB>on` or `off`. At a terminal, `ask models` groups each agent's ids with their names
and cost limits. `ask help` shows each agent's models on a line, or how many when there are
more than a dozen, including listing failures. `NO_COLOR` removes styling.

## Turning models on and off

Turn off what you don't use, so `ask models`, help and the agents that pick from them see only
the rest:

```sh
ask settings opencode                               # pick in a list you can filter
ask models codex:gpt-6.1-sol opencode:gpt-4o --disable
ask models codex:gpt-6.1-sol --enable
```

A model that is off is refused: `-m codex:gpt-6.1-sol` says it is off and how to turn it on.

## models.json

Your choices are in `~/.ask/models.json`, by agent: the models you turned off, the ones you added,
and the ones with their own cost limit. Ids ignore case; a file naming one id twice in different
case is refused. Everything else is on, so the file stays short and never
copies the whole catalog. Models you explicitly add stay even if the agent does not list them;
`ask models --all` combines the agent's list with those additions.

```json
{
  "claude": {
    "opus-5.5": { "enabled": true, "max_cost": 10 },
    "sonnet-5": true
  },
  "opencode": {
    "gpt-4o": false,
    "gpt-4o-mini": false
  }
}
```

`false` turns a model off. `true` adds a model the agent doesn't list, like `sonnet-5`, an older
Claude model. An object sets `enabled` and `max_cost`, its own [cost limit](usage.md#cost-limits).
`ask settings NAME` writes the file for you, as do `ask models MODEL --enable`,
`ask models MODEL --disable` and `ask models MODEL --max-cost USD`. Replace `MODEL` with an
`agent:id` and `USD` with a dollar amount; `--max-cost 0` clears the model limit and uses your
default. You can also edit the file. The older form, a list of ids per agent (`{"opencode": ["glm-5.3-flash"]}`), still works
and adds those models. Adding an id does not make a model available to the underlying CLI or
account. If the file exists it must be valid JSON; mistakes name the agent and model.

## Names in output

Status lines and results use the model's own name, not its id. ask takes the first of:

1. the name the agent reports for the run (`Opus 5.5`, from what Claude Code says it ran);
2. the name the agent lists next to the id (`GPT-6.1 Sol`, from Codex's model list);
3. the id in title case (`glm-5.3-flash` becomes `Glm 5.3 Flash`).

Effort is appended: `Sonnet 5.5 (high)`.
