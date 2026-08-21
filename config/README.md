# Serving configs

One file per way the server has been launched. **The config is the record of how a
measurement was produced** — [`scripts/serve.sh`](../scripts/serve.sh) reads one and adds
no flags of its own, so a row in `docs/data/` can always be traced back to a file here.

A variant is a new file, never an edit to an existing one. Editing one would silently
change what an already-recorded number was measured on.

## The four you would actually serve

| file | context | what it is for |
|---|---|---|
| [`tuned.env`](tuned.env) | 32 768 | **the settled config.** Sampling is per-request, so the scorer sets it |
| [`agent.env`](agent.env) | 49 152 | the editor flow: sampling and the thinking toggle *served*, plus the chat-template override Claude Code needs |
| [`harness.env`](harness.env) | 65 536 | the harness comparison — the one context all four candidates accept, and **unattended only** |
| [`baseline.env`](baseline.env) | 32 768 | the first config, kept as the anchor everything else is a diff against |

## The rest

- `ladder-*.env` — one cell each of the context ladder, walked by
  [`scripts/ladder.sh`](../scripts/ladder.sh). Committed so the ladder is mechanical
  rather than hand-run.
- `ctx16k-f16.env` — the variant that proved a config change needs no edit to
  `baseline.env` or to `serve.sh`.
- `mtp-32k.env`, `mtp-49k.env` — the target's own multi-token-prediction head. **`mtp-32k`
  is adopted for the grind profile**; `mtp-49k` is refused, the allocator failing on the
  first prefill batch.
- `dflash2-32k.env`, `dflash2-49k.env` — an external drafter, kept as the record of a
  candidate that never generated a token here.

The four speculative configs name a `SERVER_BIN` that is **not** the binary on `PATH`: the
mechanism exists only in llama.cpp PR #27342, and the config says which build served it.

## Not a serving config

- [`machine.json`](machine.json) — what the machine must have spare before a sweep may
  start. Read by `cmd/eval` and `cmd/tier2`, which refuse below it.
- [`profiles/qwen3.8.json`](profiles/qwen3.8.json) — the *model's* properties, not the
  server's: how thinking is switched, each mode's recommended sampling, where reasoning
  comes back. It lives here so a toggle cannot be swept at one fixed temperature by
  accident.
- [`templates/`](templates/) — the one-line chat-template override that lets a
  mid-conversation system message render instead of raising.

Why each number is what it is: [`docs/TECH.md`](../docs/TECH.md).
