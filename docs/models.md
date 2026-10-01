# Models

A model id is `harness:id`, with an optional `#effort`:

```text
mycli:smart
mycli:fast#high
othercli:pro
```

- **harness** is the executable in `~/.ask/harnesses/` that ask runs (see
  [Harnesses](harnesses.md)).
- **id** is the model as that harness's CLI names it.
- **effort** is passed to the harness, which passes it on in its CLI's own form.

## Listing

```sh
ask models
```

```text
mycli:fast
mycli:smart
othercli:pro
```

Each harness lists its own models with `NAME models`. A harness whose CLI offers too many to list
lists none, and you name the ones you use.

## Adding model ids

`~/.ask/models.json` adds ids to any harness's list, by harness name:

```json
{
  "mycli": ["experimental"],
  "othercli": ["pro-mini", "pro-max"]
}
```

The file is optional, but if it exists it must be valid JSON. ask still runs any id you give it,
listed or not. The list is for you and for agents that pick from `ask models`.

## Names in output

Status lines and results use the model's own name, not its id. ask takes the first of:

1. the name the harness reports for the run (the model an alias resolved to, say `My Smart 2`);
2. the name the harness lists next to the id (`My Smart`);
3. the id in title case (`pro-mini` becomes `Pro Mini`).

Effort is appended: `My Smart (high)`.
