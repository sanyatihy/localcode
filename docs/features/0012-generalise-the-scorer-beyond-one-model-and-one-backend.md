---
id: 0012
title: Generalise the scorer beyond one model and one backend
status: Shipped
created: 2026-08-17
shipped: 2026-08-18
needs: 0002
related: 0006, 0007
---

## Problem

The scorer is quietly welded to Qwen3.8 on llama.cpp in four places, and two planned
features walk straight into all four. 0006 serves the same model under MLX, which has
neither llama.cpp's `/props` nor its `timings`. 0007 swaps in models that do not have
Qwen's thinking toggle at all. Discovering this inside either feature means building the
seam under time pressure, in a comparison whose numbers then depend on it.

## Non-goals

- **No new models or backends here.** This makes them possible; 0006 and 0007 are what
  actually run them. A generalisation validated against nothing is a guess.
- **No plugin system, no registry, no reflection.** Two seams with two implementations
  each. The vision's "explicit over implicit" and the go-checklist's "do not pre-abstract"
  both apply: an interface earns its place by having a second implementation in hand.
- **No rewrite of the fixtures.** They are already backend-agnostic — tool schemas, Go
  patch tasks and retrieval sentinels know nothing about Qwen — and touching them would
  invalidate every result already collected.

## Design

Exactly what is coupled, established by reading the code rather than by guessing:

| Coupling | Bound to | What breaks without it |
|---|---|---|
| `chat_template_kwargs.enable_thinking` | Qwen's toggle | Models using `/no_think`, a reasoning-effort parameter, or having no toggle |
| `reasoning_content` response field | how llama.cpp surfaces thinking | Backends that embed `<think>` in content instead |
| `/props` | llama.cpp only, not OpenAI | No served-config guard, so mislabelling becomes undetectable |
| `timings` object | llama.cpp only | No server-side tok/s at all |

Two seams close all four.

**A model profile**, loaded from a committed file rather than passed as flags: how this
model toggles thinking, its recommended sampling *per mode*, and how its reasoning
surfaces. The matrix script currently hardcodes Qwen's card values in shell, which is
both invisible to the scorer and wrong for any other model. A profile makes the vision's
"never sweep a toggle that moves two things" enforceable in code rather than in a comment.

**A backend capability layer** answering two questions — what is this server actually
serving, and how fast did it go — with honest degradation. A backend that cannot report
served config returns "unknown" rather than a fabricated value, and the mislabelling
guard reports itself unavailable rather than silently passing.

**Client-measured wall time becomes the primary cross-backend speed metric.** llama.cpp
and MLX do not measure tok/s the same way, so putting two vendors' self-reported rates in
one table would be a comparison of measurement conventions. Server-reported figures stay,
labelled as such, for comparing configs *within* one backend.

Memory pressure per run belongs here too: free memory and swap delta, recorded on every
row. The vision makes a swapping run void, and 0003 measured the baseline already paging
at idle — a rule that cannot be evaluated from the data is not enforceable.

## Tasks

- [x] A committed model profile carries the thinking mechanism and per-mode sampling defaults, and the Qwen3.8 profile reproduces the current matrix exactly — same requests on the wire, proven by comparing against a recorded baseline
- [x] The scorer reads sampling defaults from the profile instead of the matrix script's hardcoded flags, so a toggle cannot be swept at one fixed temperature by accident
- [x] Reasoning is extracted through the profile, covering both a separate response field and `<think>` tags inside content
- [x] A backend capability layer reports served config and timings where available and says "unavailable" where not, with the llama.cpp implementation behaving exactly as today
- [x] Client-measured wall time is recorded for every run and documented as the only cross-backend speed metric
- [x] Free memory and swap delta are recorded per row, and the reporter flags any run that swapped rather than averaging it in

## Open questions

- Should a run that swapped be excluded from summaries automatically, or only flagged?
  Leaning **flagged and excluded from headline numbers but kept in the file** — dropping
  rows loses the evidence that a config cannot be run on this machine, which is itself a
  result 0004 needs.
- Is one profile per model enough, or is it per model *and* backend? Leaning **per model**,
  with backend differences living in the capability layer — but 0006 is what will actually
  settle it, and guessing now risks a shape that fits neither.

## Log
- 2026-08-18 — the baseline the first box asks for turned out to already exist: the shipped
  result files record the sampling each run sent, so the profile is proved against them rather
  than against a golden file written for the purpose. A profile that disagrees with the matrix
  it claims to reproduce now fails a test.
- 2026-08-18 — a missing `/props` stops being fatal. The scorer refused to start without it,
  which would have blocked 0006 before MLX served a single request; an un-introspectable
  backend is now scored with the served-config guard off and the row saying so.
- 2026-08-18 — two pieces of generalisation landed early, under 0013, because 0013 could not
  finish without them. `reasoning_effort` is passed through verbatim instead of validated
  against Qwen's `low`/`medium`/`xhigh`, since the next model's vocabulary will differ and a
  harness that hardcodes one vendor's has to be edited before it can measure anything new;
  the level is recorded per row instead. And every task now carries a `timeout_seconds`
  budget with `fail_over_budget` as its own outcome, so a model that does not terminate costs
  a bounded amount of a sweep rather than the sweep. Both belong to this feature's remit and
  are noted here so they are not designed twice.
