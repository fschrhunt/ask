# openhands

The ask adapter for the **dedicated OpenHands CLI**, not its platform, web server or SDK.
Requires Node.js 18+ and a configured `openhands` executable. Install the upstream CLI with
`uv tool install openhands --python 3.12`; configure its provider credentials before use.
Run `ask install openhands`. `ASK_OPENHANDS_BIN` overrides executable discovery; otherwise the
adapter checks PATH, `~/.local/bin`, `/opt/homebrew/bin` and `/usr/local/bin`.
`ASK_NODE` overrides Node.

## Models and write access

`openhands models` deliberately prints no catalog: add the backend ids you use to ask's model
configuration. Every run requires a concrete `ASK_MODEL`; auto/default/latest selections are
refused. The adapter runs:

```text
openhands --headless --json --always-approve --override-with-envs --task PROMPT
```

It sets `LLM_MODEL` to ask's explicit id. `--override-with-envs` overrides saved settings for this
invocation, including a resumed session. Preserve your `LLM_API_KEY` and optional `LLM_BASE_URL`;
a fresh upstream setup requires both model and API key. Write runs approve tool calls without
interactive input. They use the upstream local runtime and its configured tools/integrations;
this adapter adds no filesystem sandbox.

**Read runs are refused before starting OpenHands.** The CLI exposes no enforced read-only
sandbox or exclusive read-tool policy; LLM approval is not a read-only boundary.

## Answers and sessions

The adapter reads the CLI's JSONL events, returning the assistant `MessageEvent` text or the
final `FinishAction.message`. Tool observations, reasoning and Rich UI summaries are excluded.
A `ConversationErrorEvent` fails the run even if the process exits zero. Empty answers and
nonzero exits also fail. CLI diagnostics are suppressed to avoid leaking provider credentials.

The upstream `Conversation ID:` trailer is reported as the session id as soon as it appears;
it usually arrives only at exit, so a run stopped earlier may have no resumable id. `ASK_SESSION`
uses `--resume ID`, never latest-session selection. No token counts or cost are synthesized:
the consumed CLI event/trailer contract supplies no aggregate usage. ask's cost limits therefore
do not apply; use its timeout.

There is no documented headless title setter, effort flag or schema flag. `ASK_TITLE` is not
applied, `ASK_EFFORT` is refused, and ask validates JSON/schema answers using the prompt it passes.

## Verified upstream contract

Research preceded implementation, against OpenHands CLI commit
`954f2ba646e8d749261a8f2b2b7e3031fa39be9f`:

- [CLI command reference](https://github.com/OpenHands/docs/blob/main/openhands/usage/cli/command-reference.mdx):
  headless mode, JSONL, resumption, approval and environment overrides.
- [CLI entry point](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/entrypoint.py):
  the conversation trailer and headless initialization.
- [JSON callback](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/utils.py):
  serialized SDK events, rather than a separate CLI response envelope.
- [Message event definition](https://github.com/OpenHands/software-agent-sdk/blob/250dd4817c7174e349f5872e70f5b5a5c17de79d/openhands-sdk/openhands/sdk/event/llm_convertible/message.py),
  [conversation error](https://github.com/OpenHands/software-agent-sdk/blob/250dd4817c7174e349f5872e70f5b5a5c17de79d/openhands-sdk/openhands/sdk/event/conversation_error.py),
  and [finish action](https://github.com/OpenHands/software-agent-sdk/blob/250dd4817c7174e349f5872e70f5b5a5c17de79d/openhands-sdk/openhands/sdk/tool/builtins/finish.py):
  inspected only to decode what the CLI emits; the adapter imports no SDK.

Run `cd packages/openhands && node --test`; tests drive the launcher with a fake CLI.
See also [the ask agent contract](../../docs/agents.md). This page: `packages/openhands/README.md`.
