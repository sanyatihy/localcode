---
id: 0017
title: Measure a lossless decode speedup that survives this machine
status: Draft
created: 2026-08-20
shipped:
check:
checked:
review:
needs:
related: 0005, 0006, 0014
---

## Problem

Decode is **89–96% of wall** on the grind profile — per `2026-08-18-m2max-32gb-0005-toggle.jsonl`,
a tier-1 pass spends 267 s of 299 in generation with thinking off, 1,671 s of 1,739 with it on —
and nothing in this project has ever been aimed at it. Lossless speculative decoding is now
shipping for **this exact model** in two forms, both claiming 1.8–3×. Neither has a published
number on pre-M5 Apple Silicon at this repo's context, and one of them may not fit the machine
at all. The decision is which, if either, survives contact with a 32 GB M2 Max.

## Non-goals

- **Not adopting anything.** This produces numbers and a per-profile verdict against a rule
  written first; adoption is what the rule decides.
- **No change to the target model or quant**, and no fine-tuning of any head — out of scope
  project-wide.
- **Not a runtime migration.** If the MLX candidate wins, that reopens 0006 as its own
  feature; this measures, it does not switch what the project serves.
- **No custom kernels or trained heads.** `mlx.fast`'s challenge is exactly that, and it is a
  research programme rather than a config change.

## Design

**Two candidates, one rule, one set of harness changes.** They differ in where the draft comes
from, and that difference decides admissibility before it decides speed.

| | **A — MTPLX** (MLX, native MTP) | **B — DFlash2** (llama.cpp, external drafter) |
|---|---|---|
| draft source | the model's own MTP heads | `z-lab/Qwen3.8-27B-DFlash2-GGUF`, 1.1 GB |
| extra memory | **none** | drafter + its own KV |
| lossless by | Leviathan & Chen rejection sampling with residual correction | rejection sampling during path selection |
| Apple Silicon evidence | 1.6× on a 16 GB M4 mini, 2.24× on M5 Max; a third party got 10.5 → 18.3 tok/s on an M4 Pro against llama.cpp | **1.81×** on an M5 Pro, this model and quant |
| pre-M5 support | **M1+, explicit M1/M2 builds** | all Apple numbers are M5 |
| measured at | coding temperatures | GSM8K at `temp 1.0` |
| licence / status | Apache-2.0, released | llama.cpp PR **#27342, open not merged** |

**A is favoured going in, and the reason is memory rather than speed.** 0014 measured Q4_K_M at
32k reaching **21.75 GB wired** against a desktop that fails at **22.29 GB**. B's drafter lands
in that 0.54 GB gap or past it. A adds no drafter — but its shipped `Optimized-Speed` checkpoint
is documented to peak at **23.6 GB**, which is *also* past the line. **Neither candidate is known
to be attended-admissible, and that is the first thing measured, not the last.** 0014 established
the failure happens at load, so each verdict costs a load plus 90 seconds.

**The premise is unmeasured here in three ways, and the design is built around which falsifies
it first.** The hardware is pre-M5, so verification runs on "steel simdgroup-MMA fallbacks"
rather than the Metal 4 tensor path every published Apple figure uses. The context is 32,768,
where the one published curve falls from 2.37× at 1k to **1.34× at 8k**. And acceptance was
measured at each vendor's sampling, not at 0005's settled `temp 0.7 / top_p 0.80 / top_k 20 /
presence_penalty 1.5` with thinking off — a mismatch this repo has already been burned by once.

**The measurement method is imported, because the obvious way to run this is wrong.** From
`mlx.fast`'s ranked harness and from published MLX benchmarking practice:

- **Pair the baseline against the candidate on the same machine in the same session**, and
  report the ratio. An unpaired before/after measures host drift; their unmodified tree scores
  0.994 rather than 1.000, which is the size of the noise being cancelled.
- **Isolate decode.** The metric is `mean(baseline s/token) / mean(candidate s/token)`, not
  wall. Wall conflates a prefill this lever cannot move.
- **Gate on thermals.** Timed measurement happens inside a fixed thermal envelope, a run
  exceeding 4× median latency is rejected as a stall, and at least three accepted pairs are
  required. This repo runs multi-minute GPU-saturating fills on a laptop and has no such guard;
  a throttled run is the same category of void as a swapped one.
- **Gate on token fidelity.** Lossless is a property of the algorithm, not of a build: a greedy
  run must match token for token with the mechanism on and off, and a mismatch fails the run
  rather than scoring it.
- **Record acceptance length τ, not acceptance rate.** Published data has a draft-4 run
  accepting 47% at 34.5 tok/s losing to a draft-15 run accepting 18% at 47.2 tok/s.
- **Trust no server's self-reported rate.** Practitioners report wrong stats, OOM, and caches
  mixing prompts across sessions; and a benchmark can end up measuring the engine's KV
  allocation policy rather than the hardware — which is precisely what cost 0006 three
  misconfigurations. `docs/TECH.md`'s client-measured-wall rule already says this and it holds
  across both candidates here.

**Both profiles are reported and expected to disagree.** The editor turn is 96.7% prefill —
434 s against 15 s — so no decode lever can move it beyond ~1.02×. That is written in advance
so a flat editor result reads as predicted, not as failure.

**The decision rule, per profile, written before the runs:** unattended, adopt at ≥ **1.5×**
decode ratio at 32k with pass rate unchanged and τ recorded; reject below **1.25×**; between
them record and do not adopt. Attended, the same bar *and* 0014's desktop verdict. Any run whose
swap grew is void per the vision, and 0015's preflight applies unchanged.

## Tasks

- [ ] Both candidates are screened against 0014's desktop verdict at 32k and 49k — a load plus 90 s of sampling — and the admissible profile for each is recorded before any suite runs
- [ ] The scorer reports a paired, decode-isolated ratio against a baseline measured in the same session, and records acceptance length τ, or records it unavailable rather than absent
- [ ] A run is voided on thermal throttling and on token-fidelity mismatch, on the same footing as a swapped run, with a test covering both
- [ ] Candidate A (MTPLX) is run over the tier-1 ranking tasks at 0005's settled sampling and 32k, 3 repeats, appended to `docs/data/`
- [ ] Candidate B (DFlash2) is built from PR #27342 into a scratch prefix with its commit SHA recorded, the Homebrew build left in place, and run identically — or recorded as inadmissible by the screen above, which is a result
- [ ] The context sweep 8k/16k/32k is run for whichever candidates cleared the screen, and reported as a curve rather than a point
- [ ] The per-profile verdict is decided by the rule above and recorded in `docs/TECH.md` with the number that beat the alternative, including a rejection
- [ ] `docs/TECH.md`'s "multi-token prediction is not reachable here" is corrected: it is reachable, it is a net loss in llama.cpp on Metal, and it is the better of the two mechanisms on MLX

## Open questions

- ~~**Which MTPLX checkpoint?**~~ **Answered: `Optimized-Speed-FP16`, and one screen rather
  than three.** The FP16 build is not a fourth option but this machine's build of the same
  weights — MTPLX routes M1 and M2 to the FP16 siblings and its own doctor resolves this id as
  the default here. `Optimized-Quality` is 29.95 GB of weights, which no screen is needed to
  reject; `Bare-Speed` at 16.29 GB is the footprint-matched build if the recommended one fails.
- **Does the MLX candidate reach the editor path?** MTPLX serves an Anthropic-compatible
  `/v1/messages` with tool calls, which is one of 0006's three reasons to stay on llama.cpp,
  and its dashboard reports acceptance and cache state, which is a second. Leaning **note it and
  do not chase it here** — that is 0006's reopening, and it needs its own comparison.
- **Draft depth?** MTPLX auto-tunes depth on the machine; DFlash2's card runs `n-max 7` while
  the MLX backend warns `block_size <= 5` for quantized targets. Leaning **let A auto-tune and
  record what it chose, and measure B at 5 and 7 only**.
- **Is running an unmerged PR as a serving path a human call?** The measurement is not;
  adoption would pin this project to a branch. Leaning **INBOX only if B actually wins**.

## Log

- 2026-08-20 — **candidate B is inadmissible here, measured rather than inferred, so box 5
  takes its second branch and no suite pass is owed to it.** At both contexts the fork loads,
  answers `/health` with 200, then fails every Metal command buffer with
  `kIOGPUCommandBufferCallbackErrorOutOfMemory` and returns 500 on the first token; quantising
  the drafter's KV, the only memory lever this build has, does not change that. The drafter
  costs 0.71 GB of wired memory over the same config without it. The design's headroom
  arithmetic is also no longer an assumption: the allocator refused at **22.16 GB wired**,
  against the 24 GB `iogpu.wired_limit_mb` implies and the 22.29 GB where 0014's desktop died.
- 2026-08-20 — **the screen asks for one token, because health is not admissibility.** Its
  first build scored both of B's cells admissible on a 200 from `/health` while the server
  could not generate a word. A config that cannot produce a token is now inadmissible whatever
  its memory says, and every row carries that request's status beside the verdict.

- 2026-08-20 — **rescoped from "measure DFlash2" after research on `mlx.fast`, MTPLX and
  practitioner benchmarks.** Naming one vendor in the title made the doc argue for a candidate
  rather than decide a question, and the candidate turned out to be second-best here: DFlash2
  needs a 1.1 GB drafter against 0.54 GB of admissible headroom, while native MTP needs none.
  Two other corrections. The earlier draft called MTP "a net throughput loss on Metal" — true of
  llama.cpp's implementation, and false on MLX, where loading the checkpoint's own MTP head is
  reported at +145% over a server that ignores it. And the research this doc first rested on
  quoted **3.4×** from an M4 Max MLX write-up of a *different model at 8-bit and bf16*, where a
  slower target flatters speculation; the matched row is **1.81×**.
- 2026-08-20 — the measurement method is imported rather than invented, from `mlx.fast`'s ranked
  harness: paired same-session baseline, decode-isolated ratio, thermal gate, token-fidelity
  gate. The harness itself cannot be reused — it targets an M5 Max runner and wants ~36 GiB —
  but its gates are the part that transfers, and two of them close holes this repo already has.
