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
