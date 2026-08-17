---
id: 0001
title: Serve Qwen3.8-27B on llama.cpp
status: In progress
created: 2026-08-17
shipped:
check:
checked:
review:
needs:
related: 0003
---

## Problem

A server is already running by hand, so the gap is not that nothing serves the model —
it is that nothing can **regenerate** it. Every A/B from here restarts the server with
different flags, so the launch has to come from committed config rather than a shell
history entry, or none of the numbers those features produce can be reproduced.

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

Weights are already cached: `bartowski/Qwen3.8-27B-GGUF:Q4_K_M`, fetched by
`llama-server -hf` into `~/.cache/huggingface`, snapshot
`f0eec4a4bb4975114a030d048952d83c0a53c034`. The config names the repo and quant and lets
`-hf` resolve them, rather than managing a second copy in-repo — llama.cpp already owns
that cache, and duplicating it would cost 17 GB for nothing. `-hf` cannot pin a revision,
so the resolved snapshot is recorded and a silent upstream re-upload stays detectable.

Context is **32768** with `q8_0` for both K and V, flash-attn on, all layers on GPU and
`--parallel 1` — the configuration already observed working at ~19.3 GB resident. The
baseline is what is known to run, not a more cautious guess: 0003 finds the ceiling and
0004 sweeps beneath it, and both start from a real anchor rather than an invented one.

`--jinja` is on so the model's own chat template applies. That is load-bearing rather
than cosmetic: the template is what emits Qwen3.8's thinking blocks and tool-call format,
and 0005's whole axis depends on it being the model's and not llama.cpp's fallback.

The invocation is **not** a documented command line; it is a script reading a config
file, because a flag string in a README drifts from what was actually measured. The script
takes the config path as an argument so later features add a file per variant rather than
editing the baseline or inventing their own flags — which is what makes 0003's ladder and
0004's grid mechanical instead of hand-run.

## Tasks

- [x] A committed config file names the model repo:quant, context, KV type, flash-attn, parallelism and port; a `serve` script takes a config path and starts `llama-server` from it and nothing else
- [x] `make serve` runs that script against the baseline config, and a second config file proves a variant launches without editing the first
- [x] `make serve` answers an OpenAI-compatible `/v1/chat/completions` request with a correct response, proven by a committed smoke script
- [x] The smoke script also asserts a tool-call round-trip returns valid JSON matching the requested schema
- [x] `docs/TECH.md` records the baseline config, the observed load time, and the arch-support check to repeat after `brew upgrade`
- [x] `make check` exists and is green — the gate every feature ships through, and 0001 is the first to need it

## Log

- 2026-08-17 — box 1 rewritten. It assumed a `make models` download into a gitignored
  `models/`, but the weights were already cached by `llama-server -hf` and a second copy
  would have cost 17 GB and ~74 minutes for nothing. Replaced by naming the cached
  repo:quant in config. The premise in `## Problem` was wrong for the same reason — a
  server was already running — so the feature is now about reproducing a launch, not
  achieving one.

- 2026-08-17 — added a `make check` box: AGENTS.md gates shipping on it and no feature
  had created it. Appended rather than inserted — it blocks shipping, not the boxes above.
- 2026-08-17 — open question settled by what the machine had: the baseline is
  `bartowski/Qwen3.8-27B-GGUF:Q4_K_M`, already cached and serving, so no quant was chosen
  on the merits here at all. The uploader is now itself a variable — bartowski against
  unsloth's dynamic UD-Q4_K_XL at the same nominal quant — which belongs to 0004 alongside
  the quant ladder rather than being inherited silently from whoever downloaded first.
- 2026-08-17 — built and verified against a real restart cycle: stopped the hand-started
  server, relaunched from `config/baseline.env` to an identical process line, launched
  `config/ctx16k-f16.env` to prove a variant needs no edit to either, then restored the
  baseline. Load ~4.2 s warm-page-cache, ~18.8 GB resident at low occupancy. Smoke runs
  in ~6.5 s, which is cheap enough to gate on.
