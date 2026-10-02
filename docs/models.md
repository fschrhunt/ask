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
ask models          # the models that are on
ask models --all    # every model, on or off
```

```text
claude:sonnet-5.5
claude:haiku-4.5
codex:gpt-6.1-sol
```

Each agent lists every model its CLI offers with `NAME models`: Claude Code's current models,
Codex's model list, and every model of the providers you have set up in Opencode. Every model is
on until you turn it off.

`ask models --names` prints `agent:id<TAB>Display Name`, and `--all` in a pipe prints
`agent:id<TAB>on` or `off`. At a terminal, `ask models` groups each agent's ids with their names
and cost limits. `ask --help` shows each agent's models on a line, or how many when there are
more than a dozen, including listing failures. `NO_COLOR` removes styling.

## Turning models on and off

Turn off what you don't use, so `ask models`, help and the agents that pick from them see only
the rest:

```sh
ask setup opencode                                  # pick in a list you can filter
ask models codex:gpt-5.6-sol opencode:gpt-4o --disable
ask models codex:gpt-5.6-sol --enable
```

A model that is off is refused: `-m codex:gpt-5.6-sol` says it is off and how to turn it on.

## models.json

Your choices are in `~/.ask/models.json`, by agent: the models you turned off, the ones you added,
and the ones with their own cost limit. Everything else is on, so the file stays short and never
keeps a model an agent stopped offering; `ask models --all` is always the full list.

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
`ask setup NAME` and `ask models MODEL --enable|--disable|--max-cost` write the file for you, or
edit it. The older form, a list of ids per agent (`{"opencode": ["glm-5.3-flash"]}`), still works
and adds those models. If the file exists it must be valid JSON; mistakes name the agent and model.

## Names in output

Status lines and results use the model's own name, not its id. ask takes the first of:

1. the name the agent reports for the run (`Opus 5.5`, from what Claude Code says it ran);
2. the name the agent lists next to the id (`GPT-6.1 Sol`, from Codex's model list);
3. the id in title case (`glm-5.3-flash` becomes `Glm 5.3 Flash`).

Effort is appended: `Sonnet 5.5 (high)`.
