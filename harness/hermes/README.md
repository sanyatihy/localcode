# Hermes — working

Hermes Agent completes the same fixture as Pi and OpenCode, verified by the unseen test.
It took the longest route to get there, and the reasons are worth keeping.

## The configuration

Hermes has **no project-local config**: it reads `$HERMES_HOME/config.yaml`, and
`HERMES_HOME` defaults to `~/.hermes`. Pointing it at a directory of its own per run makes
the copy committed here the configuration that actually runs, and gives a Hermes with no
sessions, memories or learned skills — see [the harness README](../README.md).

```yaml
model:
  default: bartowski/Qwen3.8-27B-GGUF:Q4_K_M
  provider: custom          # first-class; routes to any OpenAI-compatible endpoint
  base_url: http://127.0.0.1:8081/v1
  api_key: ''               # llama-server checks nothing
  context_length: 65536
```

    hermes --yolo --cli --in <workdir> -z "<task>"

## Hermes requires a 64k minimum context

    Model ... has a context window of 32,768 tokens, which is below the
    minimum 64,000 required by Hermes Agent.

This is a hard refusal, checked before any request is made. **It is the reason the
32k baseline could never have worked**, and it has a cost 0003 already measured: a cold
64k ingest is 13.1 minutes against 5.4 at 32k.

That matters for 0010. Pi and OpenCode run happily at 32k; Hermes cannot. A like-for-like
comparison must put **all three at 64k**, which is the expensive end of the range for
every one of them — so the harness comparison inherits a context cost that is Hermes'
requirement rather than anyone's choice.

## Trap: it reports a configuration refusal as a connection error

`API call failed after 3 retries: Connection error` is what a misconfigured Hermes says, and
an instrumented listener confirmed it never opens a connection at all. It means check the
config, not the network. The `providers.<name>` map in the config is real and is **not** how a
local endpoint is configured; the `model:` block above is.

## How it ranked

Single runs at mismatched contexts used to sit here; 0010 replaced them with 60 runs at one
context, and that is the number to read — see
[docs/TECH.md](../../docs/TECH.md#nothing-displaces-claude-code-and-the-two-axes-disagree).
The short version: **Hermes is the most reliable harness measured and the dearest**, 15/15
against the incumbent's 14/15, for 4.02× the tokens per task and 9.2 turns against 3.9.

The early observation that suggested it survived the ranking: Hermes took **3 m 13 s to
answer "reply with ready"**, which is a large fixed system prompt being ingested. Its
preamble is 17,465 tokens over 17 tools, second only to Claude Code on defaults.
