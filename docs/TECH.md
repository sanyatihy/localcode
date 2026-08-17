# TECH — as built

Durable facts about what this repo actually does, moved here as features ship.
Design arguments stay in the feature docs; this is the state of the machine.

## Serving

`scripts/serve.sh <config>` starts `llama-server` from a config file and adds no
flags of its own. `make serve` runs it against `config/baseline.env`;
`make serve CONFIG=config/<variant>.env` runs a variant. Later features add a file
per variant rather than editing the baseline — that is what keeps 0003's ladder and
0004's grid mechanical instead of hand-run.

### Baseline, as measured 2026-08-17

| | |
|---|---|
| Model | `bartowski/Qwen3.8-27B-GGUF:Q4_K_M` |
| Resolved snapshot | `f0eec4a4bb4975114a030d048952d83c0a53c034` |
| Context | 32768 |
| KV cache | `q8_0` for K and V |
| Flash attention | `auto` |
| GPU layers | 999 (all) |
| Parallel slots | 1 |
| Chat template | `--jinja` (the model's own) |
| Load time | ~4.2 s, process cold and page cache warm |
| Resident memory | ~18.8 GB |

Load time is with the weights already in the OS page cache. A genuinely cold boot
reads ~17 GB off disk first and will be substantially slower.

Resident memory is measured at **low context occupancy** and is not a ceiling — KV
is not fully allocated until used. Finding the real envelope is
[0003](features/0003-establish-the-memory-and-context-ceiling.md).

### Model provenance

Weights live in llama.cpp's Hugging Face cache (`~/.cache/huggingface`), fetched by
`-hf`. The repo keeps no copy: llama.cpp already owns that cache and duplicating it
costs 17 GB for nothing.

`-hf` cannot pin a revision, so the snapshot hash above is the record. If upstream
re-uploads, `-hf` will resolve to something new and the recorded hash is what makes
that detectable. There is no automatic check for this.

## Dependencies

`llama.cpp` from Homebrew, build **10450**. It must support the `qwen35`
architecture, which is what `Qwen/Qwen3.8-27B` reports
(`Qwen3_5ForConditionalGeneration`).

**Re-check after every `brew upgrade`** — this is the dependency that breaks the
model silently rather than loudly:

```sh
strings /opt/homebrew/lib/libllama.dylib | grep -c '^qwen35$'   # expect 1
```

That build also serves the **Anthropic Messages API** at `/v1/messages` and
`/v1/messages/count_tokens`, converting to chat-completions internally. Claude Code
can therefore point at this server with `ANTHROPIC_BASE_URL` and no proxy — see
[0008](features/0008-wire-the-winning-config-into-the-coding-agent.md).

## Checks

`make check` runs `scripts/smoke.sh` against a running endpoint: health, a chat
completion that must return non-empty content, and a tool call that must return
valid JSON containing the schema's required field. It disables thinking for speed and
determinism — whether thinking helps is [0005](features/0005-tune-sampling-and-tool-call-adherence.md)'s
axis, not a gate's business.

It needs a server up. That is deliberate: the thing worth checking in this repo is
that the endpoint works, and a check that passes without one would be checking nothing.
