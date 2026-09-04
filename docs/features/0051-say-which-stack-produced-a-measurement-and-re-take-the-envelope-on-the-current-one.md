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
stack that served it: llama.cpp's build, the snapshot `-hf` resolved, the template the server
rendered. The build moved 10450 to 10621 in one session and no row says which side of it it
sits on. That upgrade also puts three recorded conclusions back in question.

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

Weights are named by resolved snapshot. `served_model` is `Qwen3.8-27B-Q4_K_M.gguf` on every
row and would survive a re-upload unchanged, which `-hf` cannot pin.

The chat template is named by content hash. Six configs point `CHAT_TEMPLATE_FILE` at this
repo's patched template, and a path records no edit to it.

Unknown is recorded as unknown, as `ServerProps.Available` and 0049's build stamp already do.
A backend that serves no `/props` is every MLX server 0006 measured.

`SERVER_BIN` in `config/mtp-32k.env` and `config/driver-mtp-32k.env` names a fork checkout
(`z-lab/llama.cpp-fork`, `5ecbe1ac`) on the premise that stock llama.cpp drops the MTP
tensors. Stock 10621 lists `draft-mtp` under `--spec-type`, MTP reached master in PR #22673,
and bartowski's quants carry the head at Q4_0. Whether stock loads those tensors for this
GGUF decides whether the fork stays: load log, greedy hash at temperature zero, acceptance.

Draft depth is swept rather than fixed. Every MTP config sets `SPEC_DRAFT_N_MAX="3"`, the
ratio falls 1.57x to 1.26x with depth, the paired benchmarks for this model run 2, and
llama.cpp issue #23230 reports 3 losing where 2 recovers. Acceptance here is 3.94 to 3.97
flat, which is higher than either report.

Sparse flash attention is measured from a source build at a pinned commit. PR #28098 merged
2026-09-03 at approximately b10785, engages with no flag, and reports prefill rising from 107
to 324 t/s at 65,536 KV on an M2 Ultra. Homebrew serves 10621 and `flash_attn_ext_vec_idx` is
absent from the installed `libggml.dylib`. Ingest is what the ladder's rungs, both desk
ceilings and the profile split derive from. Whether Qwen3.8's attention presents the mask that
activates the sparse path is unmeasured.

The stamp ships before the runs below it, so every number this feature takes names its binary.

## Tasks

- [ ] Every row names the llama.cpp build that served it, or records that the backend could
      not be asked
- [ ] Every row names the weights by resolved snapshot and the chat template by content hash,
      with unknown recorded as unknown rather than as a blank
- [ ] `docs/TECH.md` says what a row written before this feature does not carry, and which
      stack those rows were taken on
- [ ] The serving configs name a binary whose MTP support is demonstrated: stock 10621 against
      the fork checkout on loaded tensors, greedy hash and acceptance
- [ ] The MTP draft depth is swept at 2, 3 and 4 at the depth where the ratio fell to 1.26x,
      and the long-prompt verdict is re-stated on the result
- [ ] The ingest ladder is re-walked on the current build, and `docs/TECH.md` says whether
      10450's rungs survived the upgrade
- [ ] Sparse flash attention is measured against that same ladder from a source build at a
      pinned commit, and `docs/TECH.md` records what it did to ingest

## Open questions

- Does a source build with sparse attention become the served binary, or stay a measurement
  arm? Leaning: a measurement arm, with Homebrew the default until the formula carries #28098.
- Do the desk ceilings have to be re-derived if ingest moves? 0014's bounds are the
  compositor's and 0042's is the GPU-wired cap, neither obviously a function of prefill.
  Leaning: re-derive the rungs when the prompt rate moves more than 10%, and leave the
  ceilings until a fill is measured against a screen again.

## Log
