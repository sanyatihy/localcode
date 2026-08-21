---
id: 0017
title: Measure a lossless decode speedup that survives this machine
status: Shipped
created: 2026-08-20
shipped: 2026-08-20
check: 2026-09-03
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
  feature; this measures, it does not switch what the project serves. It would reopen it on
  more than speed: MTPLX serves the Anthropic `/v1/messages` 0008 needs and reports acceptance
  and cache state, which are two of the three reasons 0006 stayed on llama.cpp.
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

- [x] Both candidates are screened against 0014's desktop verdict at 32k and 49k — a load plus 90 s of sampling — and the admissible profile for each is recorded before any suite runs
- [x] Native MTP on the served GGUF — the head llama.cpp already ships past — is screened at 32k and 49k like the other two, and recorded as candidate C
- [x] The scorer reports a paired, decode-isolated ratio against a baseline measured in the same session, and records acceptance length τ, or records it unavailable rather than absent
- [x] A run is voided on thermal throttling and on token-fidelity mismatch, on the same footing as a swapped run, with a test covering both
- [x] The candidate that cleared the screen is run over the tier-1 ranking tasks at 0005's settled sampling and 32k, 3 repeats, appended to `docs/data/`
- [x] Candidate B (DFlash2) is built from PR #27342 into a scratch prefix with its commit SHA recorded, the Homebrew build left in place, and run identically — or recorded as inadmissible by the screen above, which is a result
- [x] The context sweep 8k/16k/32k is run for whichever candidates cleared the screen, and reported as a curve rather than a point
- [x] The per-profile verdict is decided by the rule above and recorded in `docs/TECH.md` with the number that beat the alternative, including a rejection
- [x] `docs/TECH.md`'s "multi-token prediction is not reachable here" is corrected: it is reachable, it is a net loss in llama.cpp on Metal, and it is the better of the two mechanisms on MLX

## Open questions

- ~~**Which MTPLX checkpoint?**~~ **Answered: `Optimized-Speed-FP16`** — not a fourth option
  but this machine's build of the recommended weights, which MTPLX's own doctor resolves as
  the default for an M2. `Optimized-Quality` is 29.95 GB and needs no screen to reject;
  `Bare-Speed` at 16.29 GB is the footprint-matched build, and it is now the open lead.
- ~~**Draft depth?**~~ **Moot:** neither candidate reached a depth comparison.
- ~~**Is running an unmerged PR as a serving path a human call?**~~ **Moot:** it was to be
  asked only if B won.

## Log

- 2026-08-20 — **rescoped from "measure DFlash2".** Naming one vendor made the doc argue for a
  candidate rather than decide a question, and that candidate turned out second-best: it needs
  a 1.1 GB drafter against 0.54 GB of admissible headroom, where native MTP needs none. Two
  numbers the draft rested on were wrong. "MTP is a net throughput loss on Metal" was true of
  llama.cpp's implementation and false on MLX. And the **3.4×** it quoted came from an M4 Max
  write-up of a different model at 8-bit and bf16, where a slower target flatters speculation;
  the matched row is **1.81×**.
- 2026-08-20 — the measurement method is imported rather than invented, from `mlx.fast`'s
  ranked harness: paired same-session baseline, decode-isolated ratio, thermal gate,
  token-fidelity gate. The harness itself wants an M5 Max runner and ~36 GiB, so only its
  gates transfer — and two of them close holes this repo already had.
- 2026-08-20 — the screen now asks for one token. Its first build scored two cells admissible
  on a 200 from `/health` while the server could not generate a word, so health is not
  admissibility and every row carries that request's status beside the verdict.
- 2026-08-20 — **box 5 takes its second branch: candidate B is inadmissible, so no suite pass
  is owed to it.** At both contexts the fork loads, answers `/health`, then fails every Metal
  command buffer out of memory; quantising the drafter's KV, its only memory lever, does not
  change that. The design's headroom arithmetic stops being an assumption — the allocator
  refused at 22.16 GB wired.
- 2026-08-20 — the editor-path question moves into the non-goal that already owned it. What
  MTPLX serves is the candidate's property, not this feature's to decide.
- 2026-08-20 — box 5 is ticked out of order, because its work was box 1's prerequisite. Kept
  over the 150-line alarm for the reason 0014 was: a rule fixed before the runs, and
  rejections that have to stay falsifiable.
- 2026-08-20 — **the screen's own method was nearly wrong, and box 1 changed because of it.**
  "A load plus 90 s" inherits 0014, which measured a runtime reserving its KV at load. MTPLX
  allocates per request, so a load-only screen scored both its cells admissible; under a
  prompt sized to the profile it reaches 23.31 GB and fails. The screen sends that prompt now,
  and the baseline survives the identical fill.
- 2026-08-20 — the feature waits on a release rather than a run, which is what `check:` is set
  for. Both candidates are out on memory, so boxes 2, 4 and 6 have nothing admissible to
  measure. A smaller build would change that, and the boxes stay as written rather than being
  rewritten around a constraint it would remove.
- 2026-08-20 — **a third candidate, found in the target's own weights, and a box added above
  the scorer for it.** The GGUF already served carries Qwen3.8's MTP head, which stock
  llama.cpp drops as unused. That is the no-extra-memory property the design credited to
  candidate A, on the runtime this project already runs.
- 2026-08-20 — **the first split this feature produced: candidate C is admissible at the grind
  profile and not at the editor one.** Boxes 2, 4 and 6 have a candidate at 32k and the editor
  profile has none. Two conditions on the 32k result, both found by getting them wrong — it
  needs `--parallel 1`, and `--n-gpu-layers 999` blocks the build's own memory fitter, a lever
  left unpulled rather than one that failed.
- 2026-08-20 — decode is isolated by streaming, because the client is the only side that sees
  where prefill ended: a non-streamed reply reports one clock covering both halves. Tasks
  carrying tools stay unstreamed and say so on the row, since streamed tool calls arrive as
  fragments and a speed number is not worth a scoring bug.
- 2026-08-20 — both voids are in, and the fidelity one is what earns the word lossless. Two
  thresholds beyond the box: a sample past four times its own side's median is a stall rather
  than a slow decode, and a side with fewer than three accepted samples reports no ratio.
- 2026-08-20 — box 4 no longer names candidate A. It is inadmissible, and the box's subject is
  whichever candidate cleared the screen.
- 2026-08-20 — the reporter was handing every config that ran a fidelity probe a free pass and
  moving the denominator the pass rate is read against. Instrument rows are skipped.
- 2026-08-20 — three depth fixtures were added because the existing ones could not carry the
  measurement: `retrieval-*.json` answers in 12 tokens, too few to divide into a per-token
  figure, where `decode-*.json` returns 384.
- 2026-08-20 — **the box asked for a curve, and the curve is why the verdict splits.** The
  ratio crosses the rule's own bands between the ranking suite's short prompts and 32k without
  the mechanism changing. A point at either end would have been a different verdict, and the
  point everyone quotes is the shallow one.
- 2026-08-20 — the correction box is corrected rather than applied: it had the two runtimes the
  wrong way round, expecting a net loss on llama.cpp and the better mechanism on MLX.
- 2026-08-20 — **the verdict splits four ways and one quarter is undecided.** Attended has no
  answer: every screen here ran unattended, and that half of 0014's rule needs a desktop
  verdict against a config peaking 0.19 GB under where that desktop died. The rejections are
  recorded with their numbers beside the winner, since a rule that only records winners is not
  a rule.
