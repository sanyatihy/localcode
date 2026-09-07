---
id: 0051
title: Say which stack produced a measurement, and re-take the envelope on the current one
status: Draft
created: 2026-09-04
shipped:
needs:
---

## Problem

A row names the model file and the `localcode` commit that wrote it, and nothing about the
stack that served it: llama.cpp's build, the Metal backend behind it, the snapshot `-hf`
resolved, the template the server rendered. The build moved 10450 to 10621 to 10809 in three
days and no row says which of them served it. Those upgrades also put three recorded
conclusions back in question.

## Non-goals

- Harness and Python-runtime versions on rows. Only tier-2 and runtime rows carry them, and
  each needs a reader of its own.
- Re-taking the sampling, reasoning-effort, tool-call and harness tables. They stand on the
  stack they were measured on until something contradicts them.
- Backfilling the 533 existing rows. A row cannot learn what served it, so `docs/TECH.md`
  says what the absence means instead.
- Adopting `draft-mtp-adaptive`. llama.cpp PR #27210 is unmerged.
- Pinning Homebrew. A source build appears here only where the formula cannot reach.

## Design

The stack is read from `/props` rather than from the config that started the server.
`eval.ServerProps` already parses that endpoint and every row already records what came back.
A config says what was asked for; `/props` says what answered.

The stack is two versions and `/props` reports one. It carries `build_info`, `chat_template`,
`model_path`, `n_ctx`, `total_slots`, `modalities` and `bos_token`, and nothing about the
backend: Metal kernels come from the separately versioned `ggml` formula, loaded at runtime
from its `libexec/libggml-metal.so`. Sparse flash attention arrived in ggml 0.22.0 to 0.23.0
while llama.cpp's own version moved 0.3.0 to 0.4.0 for unrelated reasons, so a row carrying
only `build_info` cannot tell the sparse path from its absence. The backend is resolved
host-side from the library the process loaded, and recorded as unknown when it cannot be.

Weights are named by resolved snapshot. `served_model` is `Qwen3.8-27B-Q4_K_M.gguf` on every
row and would survive a re-upload unchanged, which `-hf` cannot pin.

The chat template is named by the content hash of `/props.chat_template`, which is what the
server holds rather than what a config asked for. Six configs point `CHAT_TEMPLATE_FILE` at
this repo's patched template, and a config that failed to take renders a template no path
records.

Unknown is recorded as unknown, as `ServerProps.Available` and 0049's build stamp already do.
A backend that serves no `/props` is every MLX server 0006 measured.

`SERVER_BIN` in `config/mtp-32k.env` and `config/driver-mtp-32k.env` names a fork checkout
(`z-lab/llama.cpp-fork`, `5ecbe1ac`, 2026-08-18) on the premise that stock llama.cpp drops the
MTP tensors. Stock 10809 lists `draft-mtp` under `--spec-type`, MTP reached master in PR
#22673, and bartowski's quants carry the head at Q4_0. Whether stock loads those tensors for
this GGUF decides whether the fork stays: load log, greedy hash at temperature zero,
acceptance. The fork is about 400 builds behind stock, so a disagreement between them is not
evidence about MTP tensors on its own.

Draft depth is swept rather than fixed. Every MTP config sets `SPEC_DRAFT_N_MAX="3"`, the
ratio falls 1.57x to 1.26x with depth, the paired benchmarks for this model run 2, and
llama.cpp issue #23230 reports 3 losing where 2 recovers. Acceptance here is 3.94 to 3.97
flat, which is higher than either report.

Sparse flash attention is measured by switching the backend under one server, not by building
one. PR #28098 merged 2026-09-03, engages with no flag, and reports prefill rising from 107 to
324 t/s at 65,536 KV on an M2 Ultra. The installed ggml 0.23.0 carries
`flash_attn_ext_vec_idx` and 0.22.0 does not; both remain in the Cellar, and `libggml.dylib`
honours `GGML_BACKEND_PATH`. So the off arm is that variable pointed at 0.22.0's `libexec`,
which holds llama.cpp fixed at one build. A 0.22.0 backend under 0.23.0's base may refuse to
load; the box screens that first and falls back to a source build at a pinned commit if it
does. Ingest is what the ladder's rungs, both desk ceilings and the profile split derive from.
Whether Qwen3.8's attention presents the mask that activates the sparse path is unmeasured.

The stamp ships before the runs below it, so every number this feature takes names its binary.

## Tasks

- [x] Every row names the llama.cpp build and the ggml backend that served it, or records that
      either could not be read
- [x] Every row names the weights by resolved snapshot and the chat template by the content
      hash of what `/props` reports, with unknown recorded as unknown rather than as a blank
- [x] `docs/TECH.md` says what a row written before this feature does not carry, and which
      stack those rows were taken on
- [ ] The serving configs name a binary whose MTP support is demonstrated: stock 10809 against
      the fork checkout on loaded tensors, greedy hash and acceptance
- [ ] The MTP draft depth is swept at 2, 3 and 4 at the depth where the ratio fell to 1.26x,
      and the long-prompt verdict is re-stated on the result
- [ ] The ingest ladder is re-walked on the current build, and `docs/TECH.md` says whether
      10450's rungs survived the upgrade
- [ ] Sparse flash attention is measured against that same ladder with the backend switched
      between ggml 0.22.0 and 0.23.0, and `docs/TECH.md` records what it did to ingest

## Open questions

- Do the desk ceilings have to be re-derived if ingest moves? 0014's bounds are the
  compositor's and 0042's is the GPU-wired cap, neither obviously a function of prefill.
  Leaning: re-derive the rungs when the prompt rate moves more than 10%, and leave the
  ceilings until a fill is measured against a screen again.

## Log

- 2026-09-07 — the stamp box now names the ggml backend as well as the llama.cpp build.
  `/props` reports `build_info` only, and the Metal kernels ship in a separately versioned
  formula, so a build number alone cannot attribute a sparse-path number.
- 2026-09-07 — the template hash is taken from `/props.chat_template` rather than from the
  file on disk, so a config that did not take is visible on the row.
- 2026-09-07 — sparse flash attention no longer needs a source build. Homebrew carries it at
  ggml 0.23.0, 0.22.0 is still installed, and `GGML_BACKEND_PATH` switches between them under
  one llama.cpp build; the source build survives as the fallback if that backend refuses to
  load. This deleted the open question about whether a source build becomes the served binary.
