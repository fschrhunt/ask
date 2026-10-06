# vibe

The ask adapter for Mistral Vibe's **programmatic CLI**. Requires Node.js 18+ and a configured
`vibe` executable with the public-history streaming output described below. Install Vibe using
its [upstream instructions](https://github.com/mistralai/mistral-vibe#installation),
then run `ask install vibe`. `ASK_VIBE_BIN` overrides executable discovery; otherwise the adapter
checks PATH, `~/.local/bin`, `/opt/homebrew/bin` and `/usr/local/bin`;
`ASK_NODE` overrides Node. Older releases with raw LLM-message JSON are not supported.

## Explicit models

Set Vibe's documented `VIBE_MODELS` environment field to a JSON array of concrete backend
definitions, including provider and alias. Existing provider authentication remains in Vibe's
config/environment; this adapter does not read or rewrite your TOML files. For example:

```sh
export VIBE_MODELS='[{"name":"devstral-2512","provider":"mistral","alias":"devstral-2512"}]'
```

Use a backend id your provider currently offers. `vibe models` lists these backend names, not
shortcuts such as `devstral`. `ASK_MODEL` must match exactly one definition ignoring case.
Missing, duplicate, unknown and auto/default/latest model selections are refused before starting
Vibe. The child gets the selected definition with its alias set to its backend name,
`VIBE_ACTIVE_MODEL` set to that name, and `VIBE_ALLOWED_MODELS` restricted to the same name.
Provider settings and model pricing fields are preserved. This avoids Vibe's fallback from an
unknown configured alias to a default model. Organization-enforced config still takes precedence
in upstream Vibe; use model definitions permitted by your organization.

## Read and write

Write prompts arrive on stdin. The adapter runs `vibe -p --output streaming --agent auto-approve
--auto-approve`, allowing tool calls without approval prompts. It uses Vibe's configured tools
and adds no OS sandbox. Repository trust/configuration is handled by Vibe; the adapter does not
pass `--trust`.

**Read runs are refused before Vibe starts.** Although `--enabled-tools` is documented as an
exclusive programmatic filter, upstream's agent profile layer has higher priority than the CLI
session overrides. Custom profiles can replace built-in `auto-approve` and override that filter;
organization config and startup hooks add further override paths. A filter alone cannot establish
ask's read-only boundary in the user's existing configuration.

Model environment overrides also remain subject to higher-priority profile and organization
configuration. Use this adapter with trusted Vibe configuration whose selected `auto-approve`
profile does not override the model fields; there is no documented CLI model flag that overrides
those layers. No native schema flag is used; ask's JSON/schema prompt instruction and validation
apply.

## Sessions, usage and limits

Completed public history entries expose `sessionId`, which is reported immediately. Follow-ups
use `--resume ID` and preserve the prompt as given, with the single allowed backend model configuration. Session replay text is discarded when the current user prompt
arrives; only completed assistant text is returned, excluding reasoning and tool results.

The streaming CLI format supplies history entries without usage totals, so no token counts or
cost are reported. `ASK_MAX_COST` is passed to Vibe's native `--max-price`; its estimate depends
on the pricing in your model definition. A limit stop fails the run and preserves any reported
session. Without accurate pricing, a price cap cannot reliably limit charges; ask's timeout is
also available.

The programmatic CLI has no title setter or per-run effort flag: `ASK_TITLE` is not applied and
`ASK_EFFORT` is refused. CLI diagnostics are suppressed to avoid exposing provider credentials;
use Vibe directly to investigate a failure.

## Verified upstream contract

Research preceded implementation, against Vibe commit `7cb91894c40bb25173abcfa36e5ea2b4b81eb28c`:

- [CLI argument definitions](https://github.com/mistralai/mistral-vibe/blob/7cb91894c40bb25173abcfa36e5ea2b4b81eb28c/vibe/cli/entrypoint.py):
  programmatic output, exclusive enabled-tool filters, resumption and price limits.
- [Programmatic runner](https://github.com/mistralai/mistral-vibe/blob/7cb91894c40bb25173abcfa36e5ea2b4b81eb28c/vibe/cli/programmatic.py),
  [public history model](https://github.com/mistralai/mistral-vibe/blob/7cb91894c40bb25173abcfa36e5ea2b4b81eb28c/vibe/app_server/models.py)
  and [wire aliases](https://github.com/mistralai/mistral-vibe/blob/7cb91894c40bb25173abcfa36e5ea2b4b81eb28c/vibe/app_server/_model.py):
  completed entries, camelCase fields, and limit failures.
- [Environment config](https://github.com/mistralai/mistral-vibe/blob/7cb91894c40bb25173abcfa36e5ea2b4b81eb28c/vibe/core/config/layers/environment.py)
  and [model config](https://github.com/mistralai/mistral-vibe/blob/7cb91894c40bb25173abcfa36e5ea2b4b81eb28c/vibe/core/config/vibe_schema.py):
  JSON environment values, model filtering and default fallback.
- [Config layer order](https://github.com/mistralai/mistral-vibe/blob/7cb91894c40bb25173abcfa36e5ea2b4b81eb28c/vibe/core/config/default_orchestrator.py)
  and [agent discovery](https://github.com/mistralai/mistral-vibe/blob/7cb91894c40bb25173abcfa36e5ea2b4b81eb28c/vibe/core/agents/registry.py):
  profile overrides outrank CLI filters and custom profiles can replace built-ins.

Run `cd packages/vibe && node --test`; the launcher tests use a fake CLI without network/models.
See also [the ask agent contract](../../docs/agents.md). This page: `packages/vibe/README.md`.
