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

## What the wrong path cost, and why it looked like a network fault

The `providers.<name>` map in `~/.hermes/config.yaml` is real — the source normalises it,
and an unknown provider name is rejected differently from a configured one — but it is
**not** how a local endpoint is configured. Going down it produced
`API call failed after 3 retries: Connection error`, which reads as a transport problem
and is not one: an instrumented listener confirmed Hermes never opened a connection at
all, and the only hit was curl's own probe.

The lesson is that Hermes reports a configuration refusal as a connection error. Reading
the source was what suggested `providers.*`; reading the *documentation* gave the `model:`
block in one step.

## Measured

| harness | patch-nil-check | context served |
|---|---|---|
| Pi | 38.5 s | 32k |
| OpenCode | 3 m 06 s | 32k |
| Hermes | **4 m 45 s** | 64k |

Hermes also took **3 m 13 s to answer "reply with ready"**, a trivial prompt, which points
at a large fixed system prompt being ingested every turn. At 64k that is expensive, and it
is exactly the per-turn context overhead 0010 exists to measure. These are single runs on
different context sizes and are not a ranking.
