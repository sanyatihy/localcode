---
id: 0001
title: Serve Qwen3.8-27B on llama.cpp
status: Draft
created: 2026-08-17
shipped:
check:
checked:
review:
needs:
related: 0003
---

## Problem

Nothing serves a model yet. Every later feature — the harness, all three A/Bs, the
fine-tune decision — measures against a running endpoint, so until one exists and is
reproducible from the repo, no number this project produces can be regenerated.

## Non-goals

- **No quantisation comparison.** One quant, chosen to fit, so later work has a baseline.
  Choosing the best quant is 0004's whole job and needs the harness to judge it.
- **No context tuning.** A conservative context that certainly fits; finding the ceiling
  is 0003.
- **No MLX.** Second runtime is 0006; a single working endpoint comes first.
- **No agent integration.** Pointing a real coding tool at this is 0008.
- **Not exposed beyond loopback.** The vision rules out multi-user and remote serving.

## Design

`llama-server` from Homebrew (build 10450) already supports the `qwen35` architecture
that `Qwen/Qwen3.8-27B` reports (`Qwen3_5ForConditionalGeneration`), so no build from
source is needed — confirmed by the arch table in `libllama.dylib`. Recheck after any
`brew upgrade`, because this is the one dependency that silently breaks the model.

Weights come from `unsloth/Qwen3.8-27B-GGUF` at **Q4_K_M** (~16.4 GB). It is the
largest quant leaving room for a usable KV cache, and the point of the baseline is to
fit with margin, not to be optimal. Downloads go to a gitignored `models/` — GGUFs must
never enter git history.

Context is set to **16384** for the baseline: KV at q8_0 costs ~128 KiB/token, so 16k is
~2.0 GB, landing near 20 GB total with weights and compute buffers. That fits under the
default macOS GPU wired limit without touching `sysctl`, which keeps this feature free
of system tuning. `--flash-attn` on, KV cache `q8_0` for both K and V.

The invocation is **not** a documented command line; it is a script in the repo reading
a config file, because a flag string in a README drifts from what was actually measured.
Every later feature overrides that config rather than inventing its own arguments.

## Tasks

- [ ] `models/` is gitignored and a documented `make models` fetches the Q4_K_M GGUF, verifying its checksum
- [ ] A committed config file holds model path, context, KV type, flash-attn and port; a `make serve` script starts `llama-server` from it and nothing else
- [ ] `make serve` answers an OpenAI-compatible `/v1/chat/completions` request with a correct response, proven by a committed smoke script
- [ ] The smoke script also asserts a tool-call round-trip returns valid JSON matching the requested schema
- [ ] `docs/TECH.md` records the baseline config, the observed load time, and the arch-support check to repeat after `brew upgrade`

## Open questions

- Q4_K_M or unsloth's UD-Q4_K_XL? Leaning plain **Q4_K_M** for the baseline: dynamic
  quants are a variable 0004 should measure, not inherit silently.

## Log
