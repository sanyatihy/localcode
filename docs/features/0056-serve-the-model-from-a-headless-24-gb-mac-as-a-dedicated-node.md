---
id: 0056
title: Serve the model from a headless 24 GB Mac as a dedicated node
status: Draft
created: 2026-09-13
submitted:
needs: 0053
---

## Problem

A 24 GB M5 Pro bought to do nothing but serve has no desk. Q4_K_M wires about 19.9 GB
at 32,768 with the projector off, the cap Metal derives on 24 GB is below the 17.4 GB
weights file, and `gpuraise.sh` refuses the raise because its 8 GiB reserve was written
for a laptop with an editor and a browser. Nothing here can say whether the node
carries the model, at which context, or what it costs against the laptop.

## Non-goals

- The client side: 0053.
- More than one slot. The node has one user; sharing is 0054.
- A lighter quant. The envelope is Qwen3.8-27B at Q4_K_M. A node that cannot carry it at
  16,384 with every lever is recorded as not carrying this model; another quant is a new
  feature citing this one.
- A team's uptime: 0055. For one user, restart on failure is the requirement.
- A faster machine. The M5 Pro's bandwidth is 307 GB/s against the laptop's 400, so
  decode parity is the hypothesis; the tier-1 rows say.

## Design

Each machine file carries `reserve_gb`; `config/machine.json` gets 8. `gpuraise.sh` and
`rungs.sh` read it from the file `MACHINE` names, default `config/machine.json`, and
`RESERVE_GB` in the environment still wins. The node's reserve is the idle anonymous
memory `memprobe.sh` reads at the login window plus 1 GiB; the reading is recorded in
TECH and the margin is the decision. `internal/eval/machine.go` does not read it.

Two LaunchDaemons under `scripts/node/`: one runs as root and applies the cap at boot,
because `iogpu.wired_limit_mb` does not survive a reboot and the sysctl needs root; the
other runs as the serving user, whose home holds the HuggingFace cache, and runs
`serve.sh` on the node config with `KeepAlive` on failure. `gpuraise.sh` treats a raise
to the value already set as a no-op, so a restart after a crash does not fail on its
own cap. `stop_server` in `lib.sh` boots the serving daemon out through `STOP_CMD` when
it is loaded and stops the process otherwise.

`serve.sh` gains `NO_MMPROJ`, passed as `--no-mmproj` when set: `-hf` loads the
projector whether or not a session uses it, and TECH measured it at 1.02 GB.
`config/node-32k.env` is `driver-mtp-32k-stock.env` with the projector off: 32,768 is
the largest context the screen can start from on 24 GB, and the draft head is admitted
there (0025). `agent.env` stays the default on machines that carry 49,152.

The screen runs the node config with the head and without at 32,768, each filled and
with a saved prefix in the prompt cache, since that cache costs nothing at load. If
neither passes, KV at `q4_0` and then 16,384 are screened in that order; 0003 measured
`q4_0` 0.47 GB under `q8_0` at 32,768. A pass is a filled context with zero swap delta
and headroom at or above the node's floor. Every branch failing is recorded as the
node not carrying this model.

The machine file's single profile is `headless`, ceiling from the ladder. Every node
run uses condition `headless`, which `deskverdict.py` reads as `not_applicable`. The
rungs are given through `CELLS`; the derivation in `rungs.sh` carries the laptop's
ingest coefficients. `min_headroom_gb` is measured on the node.

Node preparation: logged out at the login window, Spotlight off, sleep off, automatic
updates off. README lists the commands; TECH records the idle `anonymous_gb` reading
and the reserve derived from it, which a teammate re-takes to check their node matches.

The link is a Thunderbolt bridge with manual addresses at both ends, recorded in
README, because 0053's sandbox admits the resolved address only. The serving daemon
sets `HOST` per 0053; the node config keeps loopback.

Rows from the node carry the machine in the file name, `<date>-m5pro-24gb-<what>.jsonl`,
per `docs/data/README.md`, until 0054 puts it on the row.

## Tasks

- [ ] `reserve_gb` lives in each machine file, `gpuraise.sh` and `rungs.sh` read it from the file `MACHINE` names with `RESERVE_GB` still winning, and the laptop's 8 has one home
- [ ] `serve.sh` passes `--no-mmproj` when `NO_MMPROJ` is set, and `config/node-32k.env` is the stock draft-head driver config with the projector off
- [ ] `gpuraise.sh` treats a raise to the value already set as a no-op, covered by a test
- [ ] Two LaunchDaemons under `scripts/node/` apply the cap as root and serve the node config as the serving user at boot with restart on failure, and `stop_server` boots the serving daemon out when it is loaded
- [ ] `config/machine-m5pro-24gb.json` holds the node's measured reserve, floor and one `headless` profile, and TECH records the idle anonymous reading the reserve came from
- [ ] The screen runs the node config with the head and without at 32,768, filled and with a saved prefix, then `q4_0` KV and 16,384 if needed, and TECH records wired, headroom and which levers Q4_K_M needed on 24 GB or that it does not fit
- [ ] The ladder runs on the node from explicit `CELLS` and TECH records which of memory or time binds on 24 GB, with the ceiling written into the machine file
- [ ] The tier-1 suite runs on the node under the laptop's settings and TECH records prefill, decode and peak wired beside the laptop's, head on and off
- [ ] README documents the node: preparation with its check, the bridge addresses, the daemons, and a `smoke` from the laptop over the bridge

## Log
