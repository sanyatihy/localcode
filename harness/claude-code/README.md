# Claude Code — working, and unlike the other three

Claude Code is the only candidate that speaks the **Anthropic Messages** API rather than
OpenAI chat-completions, and the only one configured entirely through environment
variables: [`claude-code.env`](claude-code.env) is its provider block, and every variable
in it comes from Claude Code's own documentation read against 2.1.233.

    set -a; . harness/claude-code/claude-code.env; set +a
    claude -p "…" --tools Bash,Edit,Read,Write

## It needs the server configured for it

llama-server serves `/v1/messages` and `/v1/messages/count_tokens` and converts to its
chat-completions path internally, so there is nothing in the request path. Two things on
the server side are not optional:

- **`config/agent.env`, not `config/tuned.env`.** Sampling and the thinking toggle are
  per-request for the scorer and Claude Code sends neither, so they have to be served as
  defaults. Without that this model thinks at `xhigh`.
- **A one-line chat-template override.** Claude Code sends a `role: "system"` message
  *after* the user turn — the `mid-conversation-system-2026-04-07` capability — on every
  request, with 21 tools defined or with none, and no documented variable stops it.
  Qwen3.8's own template raises on a non-leading system message; llama.cpp returns that as
  a 500 and Claude Code retries ten times and dies.
  `config/templates/qwen3.8-system-anywhere.jinja` renders it as its own ChatML block
  instead.

Anthropic documents an automatic retry that disables the capability after such a
rejection, but it matches on the upstream's error wording — a Jinja exception carries
none, so the recovery path never fires.

## `--tools` is the whole story

Restricted to four coding tools it is the cheapest harness measured; on its defaults it is
the most expensive. The `CLAUDE_CODE_DISABLE_*` variables move the total by 7% and remove
no tool from the request. See the table in [../README.md](../README.md).

## Driving it from the editor

The extension reads `claudeCode.environmentVariables` before it launches, so that is where
the configuration has to be — values in `~/.claude/settings.json` reach the spawned process
but not the extension's own login check.

**Put it in the folder's settings, not the user's.** These variables point Claude Code at a
27B model on loopback; in user settings they would do that to *every* session on the
machine, including whatever else the editor is open on. `.vscode/settings.json` in the
checkout you are testing applies only while that folder is open. The extension honours
folder scope for this setting — `initialPermissionMode` is documented as the one that
ignores workspace values.

The file is [`claude-code.env`](claude-code.env) in the editor's form:

```json
{
  "claudeCode.environmentVariables": [
    { "name": "ANTHROPIC_BASE_URL", "value": "http://127.0.0.1:8081" },
    { "name": "ANTHROPIC_AUTH_TOKEN", "value": "local" }
  ],
  "claudeCode.disableLoginPrompt": true
}
```

Every variable from the env file goes in that array; keep the two in step or the terminal
and the editor are running different configurations. It is deliberately not committed: a
tracked `.vscode/settings.json` would route every session opened on this repository at the
local model, which is a decision for whoever opens it and not for the repository.

Confirm it took with `/status`, which names the base URL in force. If the server is not
running the session fails to connect rather than falling back to a hosted model, because
the base URL is fixed to loopback.

## It cannot be offline

`ANTHROPIC_BASE_URL` routes every model call, so no prompt reaches a hosted model. It does
not route the rest: OAuth refresh, feature-flag fetches, the fast-mode availability check
and WebFetch's domain-safety preflight go to Anthropic hosts regardless.
`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` covers some of that and the observed hosts
are measured rather than assumed — see
[0008](../../docs/features/0008-wire-the-winning-config-into-the-coding-agent.md).
