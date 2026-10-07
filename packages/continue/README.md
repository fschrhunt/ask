# continue

The ask adapter for Continue's **`cn` CLI**, using its documented headless mode.
Requires Node.js 18+ and an authenticated/configured `cn` installed with
`npm install -g @continuedev/cli`. `ask install continue` installs this package.
`ASK_CONTINUE_BIN` overrides executable discovery; otherwise the adapter checks PATH,
`~/.local/bin`, `/opt/homebrew/bin` and `/usr/local/bin`. `ASK_NODE` overrides Node.

## Models and configuration

Set `ASK_CONTINUE_CONFIG` to an absolute path containing a **JSON-encoded Continue YAML config**.
JSON is valid YAML; the adapter uses Node built-ins and deliberately does not parse arbitrary YAML.
The normal `~/.continue/config.yaml` is not silently used. For example:

```json
{
  "name": "ask models",
  "version": "1.0.0",
  "schema": "v1",
  "models": [
    {
      "name": "Devstral",
      "provider": "mistral",
      "model": "devstral-2512",
      "apiKey": "${{ secrets.MISTRAL_API_KEY }}"
    }
  ]
}
```

Use a backend id your provider offers. `continue models` lists concrete chat-model ids from this
file. `ASK_MODEL` must match exactly one entry, ignoring case; missing, unknown, duplicate,
Hub-package and floating/default model selections are refused. The adapter writes a private
temporary config containing only that model, preserving its provider settings and the rest of
the config, then runs `cn -p --silent --config FILE --allow '*'` with the prompt on stdin.
It removes the temporary config after ordinary completion, including CLI failure.

**`cn --model` does not select a backend id:** it adds a model package from the Continue Hub.
Passing ask's model id to that flag would be incorrect. A single-model config also prevents a
saved interactive selection from choosing a different model.

## Access and output

**Read runs are refused before cn starts.** Upstream `--readonly` plan mode permits Bash and MCP,
and its mode policy replaces CLI exclusions. It is not an ask read-only boundary. Write runs
explicitly allow tools and preserve the user's configured integrations. They can execute shell
commands and edit files outside the working directory; this is not an OS sandbox.

Headless text is returned as the answer. The adapter avoids `--format json`: that flag wraps or
validates the model's answer rather than returning a session/usage envelope. ask's JSON/schema
instructions remain in the prompt, and ask validates the answer; there is no native schema flag.

No resumable session, token counts, or cost are reported. `--resume` chooses the latest session;
`--fork ID` creates a different session, so `ASK_SESSION` is explicitly refused. `ASK_TITLE` has
no documented CLI setter and is not applied. `ASK_EFFORT` is refused. Cost limits cannot be
enforced through this adapter; use ask's timeout. CLI diagnostics are suppressed to avoid exposing
provider credentials; run cn directly to investigate a failure.

## Verified upstream contract

Research preceded implementation, against Continue source commit
`5522c6f44ca0ac3528b37244818fbfa39b5af470`:

- [Headless mode](https://docs.continue.dev/cli/headless-mode) and
  [configuration](https://docs.continue.dev/cli/configuration).
- [CLI options](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/shared-options.ts):
  Hub model addition, explicit config, and permission flags.
- [Permission defaults](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/permissions/defaultPolicies.ts)
  and [mode precedence](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/services/ToolPermissionService.ts):
  plan mode allows shell/MCP and replaces CLI rules.
- [Headless output](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/commands/chat.ts):
  answer text versus JSON wrapping.

The source is authoritative where older docs differ about headless permissions.
Run `cd packages/continue && node --test`; the launcher tests use a fake cn, without network calls.

See also [the ask agent contract](../../docs/agents.md). This page: `packages/continue/README.md`.
