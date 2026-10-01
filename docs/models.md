# Models

A model id is `harness:id`, with an optional `#effort`:

```text
mycli:atlas-2.1
mycli:atlas-2.1-mini#high
othercli:nova-4
```

- **harness** is the executable in `~/.ask/harnesses/` that ask runs (see
  [Harnesses](harnesses.md)).
- **id** is the model as the harness names it. By convention that is the lowercase family,
  version and variant, like `gpt-6.1-sol` or `sonnet-5.5`, never an alias like `latest` (see
  [Naming models](harnesses.md#naming-models)).
- **effort** is passed to the harness, which passes it on in its CLI's own form.

## Listing

```sh
ask models
```

```text
mycli:atlas-2.1
mycli:atlas-2.1-mini
othercli:nova-4
```

Each harness lists its own models with `NAME models`. A harness whose CLI offers too many to list
lists none, and you name the ones you use.

## Adding model ids

`~/.ask/models.json` adds ids to any harness's list, by harness name:

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

1. the name the harness reports for the run (the model an alias resolved to, say `Atlas 2.1`);
2. the name the harness lists next to the id (`Atlas 2.1`);
3. the id in title case (`nova-4-mini` becomes `Nova 4 Mini`).

Effort is appended: `Atlas 2.1 (high)`.
