---
id: 0006
title: A/B llama.cpp against MLX
status: Draft
created: 2026-08-17
shipped:
check:
checked:
review:
needs: 0002, 0012
related: 0007
---

## Problem

llama.cpp was chosen in 0001 because it was already installed, which is a reason to
start with it and not a reason to keep it. MLX is built for Apple Silicon and often wins
on prompt processing — the metric that dominates agentic latency, since every turn
re-reads a growing context. Whether that theoretical advantage survives contact with
this model at this quantisation on 32 GB is unknown, and it is cheap to settle.

## Non-goals

- **No quantisation sweep on MLX.** Compare at matched effective bit-width; a full second
  grid doubles 0004 for a runtime that may not win.
- **No model comparison.** 0007's axis.
- **Not a commitment to switch.** The output is a decision with numbers behind it; if
  llama.cpp wins, MLX is dropped and the reason recorded.

## Design

Run 0002's harness against both runtimes serving the same model at the closest achievable
matched quantisation, and report the same five metrics. MLX quantisation is not bit-for-bit
comparable to GGUF k-quants, so the honest comparison is **at matched memory footprint**,
with the mismatch stated rather than smoothed over.

MLX must be installed first — nothing MLX-related exists on this machine today, so the
feature carries its own setup, in a virtualenv that does not become a global dependency.
Its memory cost during a run counts against the envelope from 0003; a Python runtime
competing with the model for 32 GB is part of what is being measured.

Prompt-processing speed is weighted heavily in the decision and reported at realistic
agentic context depths, not at 512 tokens. A runtime that wins on an empty context and
loses at 16k has lost the case that matters.

## Tasks

- [x] MLX and mlx-lm install into an isolated environment, with the setup committed and repeatable
- [ ] The same model is served under MLX at a documented matched-footprint quantisation, answering the same OpenAI-compatible requests
- [ ] Both runtimes are scored on the harness at realistic context depths, not just short prompts
- [ ] Peak memory of each runtime is measured against 0003's envelope, Python overhead included
- [ ] A decision is recorded in `docs/TECH.md` with the numbers, including what would reverse it

## Open questions

- If MLX wins on prompt processing but loses on memory headroom, which takes precedence?
  Leaning **memory headroom**, because 0003 establishes it as the binding constraint —
  but this is close enough that the numbers should decide it in the open.

## Log
- 2026-08-18 — the matched pair is **MLX 4bit at 16.1 GB against llama.cpp Q4_K_M at 17 GB**,
  which is the closest available. The 8bit build is 29.5 GB and sits past the ~22.2 GB
  admissible wired budget 0014 measured, so this comparison has one quantisation on each side
  and not a ladder.
- 2026-08-18 — **context is not a matched variable across these runtimes.** `llama-server`
  reserves a KV cache at load and reports what it serves; `mlx_lm` sizes per request and has
  no equivalent number, so the two cannot be set to "the same context" and depth has to be
  measured by sending deep prompts instead. That is also why 0012's served-config guard had
  to become optional before this feature could run at all.
- 2026-08-17 — now needs 0012. The scorer is welded to Qwen's thinking toggle and to
  llama.cpp's `/props` and `timings`; this feature would otherwise have to build that seam
  itself, under time pressure, in a comparison whose numbers then rest on it.
