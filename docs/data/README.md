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
| `2026-08-17-m2max-32gb-tier1-matrix.jsonl` | 42 runs | Thinking on vs off at model-card sampling, baseline 32k/q8_0, three passes over seven tasks. |

## How these get here

`results/*.jsonl` in a worktree is **live scratch** and stays gitignored — it accumulates
across runs and gets deleted with the worktree. A snapshot is copied here when a feature
ships, which is the point at which numbers stop changing and start being cited.

## Reading them

Rows are JSON Lines. Both files carry `condition`, so never aggregate without grouping by
it. Ladder rows also carry an `apparatus` record naming what else was resident, since a
condition label is a claim and that row is the evidence for it.
