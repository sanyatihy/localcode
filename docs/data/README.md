# Measurement data

Raw rows behind the tables in [../TECH.md](../TECH.md). Committed because they are the
evidence every conclusion in this repo rests on, and because some of them stop being
reproducible: a 128 GB machine is planned, and the 32 GB baseline cannot be re-measured
once the hardware is gone.

## Naming

    <date>-<machine>-<what>.jsonl

The machine is in the filename because a measurement without it is not reusable. Nothing
here should be compared across machines without saying so.

## What is here

| file | rows | what |
|---|---|---|
| `2026-08-17-m2max-32gb-ceiling-ladder.jsonl` | 12 cells + apparatus | Context ladder 8k–64k, two conditions: `unattended` (clean boot, swap zero) and `attended-worked-in` (4 h uptime, 13.7 GB swap already allocated). The worked-in rows carry `instrument: pre-timing` and have no `fill_seconds` — they predate that metric and cannot be compared on speed. |
| `2026-08-18-m2max-32gb-desktop-band.jsonl` | 5 cells + apparatus | The 32k–64k band re-walked `attended-worked-in` against the desktop rather than the model, with wired memory and WindowServer sampled through each fill. Carries `desktop_verdict`, `wired_peak_gb` and the WindowServer series summary. The apparatus row records the desktop baseline the verdicts were judged against. |
| `2026-08-18-m2max-32gb-0013-calibration-r1.jsonl` | 84 runs | First calibration of the 14-task suite, both thinking modes, three passes, at 32k/q8_0. **Failed its own criterion** — one genuine failure in 78 clean runs, so the suite still cannot rank. Six rows are `fail_truncated_at_cap` and are cap artifacts, not quality: read them with the caps of that run (2048 patch, 1024 tool-call), which have since been raised. |
| `2026-08-18-m2max-32gb-0013-reasoning-effort.jsonl` | 18 runs | The three tasks whose caps bound at `xhigh`, re-run at `low` and `medium` with `reasoning_effort` set explicitly. The first file to carry the level; rows in every earlier file are `xhigh` whether they say so or not. |
| `2026-08-18-m2max-32gb-0013-readonly-fixed.jsonl` | 12 runs | `toolcall-constraint-readonly` at all four reasoning levels after its fixture was fixed. Supersedes that task's rows in the two files above, which scored the model for declining to patch a file it had not been shown. |
| `2026-08-18-m2max-32gb-0005-toggle.jsonl` | 81 runs | Thinking off, on/low and on/medium, **each at its own model-card sampling** — the first clean comparison across the toggle. Supersedes every earlier thinking-vs-off figure. |
| `2026-08-18-m2max-32gb-0005-sampling.jsonl` | 162 runs | Temperature and top_p swept around each mode's default, plus greedy, one value moved per cell. |
| `2026-08-19-m2max-32gb-0006-runtime.jsonl` | 84 runs | llama.cpp against MLX, 42 rows each, same model matched by footprint, 0005's settled config, only `-endpoint` differing. MLX at 2 cache slots — the memory-safe configuration. |
| `2026-08-19-m2max-32gb-0006-mlx-16-slots.jsonl` | 42 runs | The same MLX run at 16 cache slots. Every depth row fails with free memory at 0.00 and swap flat: evidence that `mlx_lm` allocates cache capacity per slot at startup, not as it fills. |
| `2026-08-19-m2max-32gb-0008-toolcall-messages.jsonl` | 12 runs | The four tool-call fixtures through the **Anthropic Messages path** llama-server converts internally, three passes, served by `config/agent.env`. Sampling and the thinking toggle are the server's on this path and are not on the row; `prompt_per_second` and `gen_per_second` are zero because that endpoint reports no timings. Compare with the `thinking: off` tool-call rows of `2026-08-18-m2max-32gb-0005-toggle.jsonl`. |
| `2026-08-19-m2max-32gb-0008-residual-traffic.jsonl` | 3 conditions + apparatus | Which hosts a `claude -p` session contacts while doing one real task, measured through a CONNECT proxy that records the host and tunnels TLS untouched. Records hosts, never traffic. The apparatus row carries what the instrument cannot see. |
| `2026-08-19-m2max-32gb-0008-harness-overhead.jsonl` | 6 harnesses + apparatus | What each harness spends before the user's first word: system prompt and tool definitions, counted by the served tokeniser. No model ran — every harness was pointed at an endpoint that records the request and answers 400, so `model_calls_before_giving_up` counts attempts, not a turn. |
| `2026-08-19-m2max-32gb-0010-harness-sweep.jsonl` | 60 runs | Four harnesses over five patch fixtures, three passes, at 65,536 — the one context all four accept, since Hermes refuses anything under 64,000. Every run cold (each harness pointed at a state directory of its own) and bounded at ten minutes. `prompt_tokens`, `cached_tokens` and `completion_tokens` are the server's counters read either side of each run, not the harness's own accounting; `turns` counts the tasks its slot was busy with. Timings on rows whose `swap_delta_mb` grew measured the pager: at this context with an editor resident the machine pages, so read wall only from the clean rows. |
| `2026-08-19-m2max-32gb-0010-offline.jsonl` | 4 runs | The same fixture with the network denied by a sandbox profile, loopback left open. `offline: true` on every row. Under the same profile at the same moment `api.anthropic.com` and `registry.npmjs.org` both failed to connect while the local endpoint answered 200. |
| `2026-08-17-m2max-32gb-tier1-matrix.jsonl` | 42 runs | Thinking on vs off at model-card sampling, baseline 32k/q8_0, three passes over seven tasks. |

## Erratum: page size, 2026-08-18

`2026-08-17-m2max-32gb-ceiling-ladder.jsonl` was written by a `memprobe.sh` that assumed a
4 KB page. This machine pages at 16 KB, so in every row **`free_gb` and `compressed_gb` —
inside `before`, `loaded` and `filled` — and `free_at_peak_gb` are understated by exactly
4×.** Multiply, or re-derive from the raw counters.

`swap_used_mb` comes from `sysctl` and `llama_rss_gb` from `ps`; neither is affected, and
those are the fields the file's conclusions rest on. The rows are left as written rather
than rewritten in place: they are what the instrument recorded, and a results file that is
quietly edited after the fact is no longer evidence. The instrument is fixed going forward,
and rows carrying `wired_gb` are the ones taken after the fix.

## How these get here

`results/*.jsonl` in a worktree is **live scratch** and stays gitignored — it accumulates
across runs and gets deleted with the worktree. A snapshot is copied here when a feature
ships, which is the point at which numbers stop changing and start being cited.

## Reading them

Rows are JSON Lines. Both files carry `condition`, so never aggregate without grouping by
it. Ladder rows also carry an `apparatus` record naming what else was resident, since a
condition label is a claim and that row is the evidence for it.
