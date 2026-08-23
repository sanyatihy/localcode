# Serving configs

One file per way the server has actually been launched. **The config is the record of how a
measurement was produced** — [`scripts/serve.sh`](../scripts/serve.sh) reads one and adds no
flags of its own, so a row in `docs/data/` traces back to a file here.

A variant is a new file, never an edit to an existing one.

| file | context | what it is for |
|---|---|---|
| [`tuned.env`](tuned.env) | 32 768 | **the settled config**, and what `make serve` starts. Sampling is per-request, so the scorer sets it |
| [`agent.env`](agent.env) | 49 152 | the editor flow: sampling and the thinking toggle *served*, plus the chat-template override Claude Code needs |
| [`harness.env`](harness.env) | 65 536 | the harness comparison — the one context all four candidates accept, and **unattended only** |
| [`mtp-32k.env`](mtp-32k.env) | 32 768 | the target's own multi-token-prediction head. **Adopted for the grind profile** at 1.26–1.57× |
| [`mtp-49k.env`](mtp-49k.env) | 49 152 | the same at the editor profile, **refused** — the allocator fails on the first prefill batch |
| [`driver-mtp-32k.env`](driver-mtp-32k.env) | 32 768 | **what `localcode` serves**: `agent.env`'s served defaults at the context the MTP head is admissible at. A chain finished the same instruction 20% sooner here |
| [`dflash2-32k.env`](dflash2-32k.env) | 32 768 | an external drafter, kept as the record of a candidate that never generated a token here |
| [`dflash2-49k.env`](dflash2-49k.env) | 49 152 | the same, at the editor profile |

The five speculative configs name a `SERVER_BIN` that is **not** the binary on `PATH`: the
mechanism exists only in llama.cpp PR #27342, and the config says which build served it.

## What is deliberately not a file here

**Ladder rungs.** [`scripts/rungs.sh`](../scripts/rungs.sh) derives them from what the
machine reports and says which of memory or time binds;
[`scripts/ladder.sh`](../scripts/ladder.sh) generates each cell from `tuned.env` with the
context and KV type moved. Nine rungs used to sit here as files, which made them facts about
one laptop — the defect [`docs/VISION.md`](../docs/VISION.md) names first.

**A "baseline".** It was settings-identical to `tuned.env` under a second name, so it could
only ever disagree with it.

## Not a serving config

- [`machine.json`](machine.json) — **every limit this machine imposes**: the headroom a sweep
  needs before it may start, and the context ceiling each desk profile caps a run at. Read by
  `cmd/eval` and `cmd/tier2`, which refuse below it. Nothing in Go carries one of these
  numbers. The code reads `min_headroom_gb`, `desk_profiles[].name` and
  `desk_profiles[].ceiling_tokens`; `machine`, `note` and `why` are prose, since JSON has no
  comments.
- [`profiles/qwen3.8.json`](profiles/qwen3.8.json) — the *model's* properties, not the
  server's: how thinking is switched, each mode's recommended sampling, where reasoning comes
  back. All three are read; a profile naming a mechanism the scorer cannot perform is refused
  at load.
- [`templates/`](templates/) — the one-line chat-template override that lets a
  mid-conversation system message render instead of raising.

Why each number is what it is: [`docs/TECH.md`](../docs/TECH.md).
