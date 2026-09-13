---
id: 0056
title: Serve the model from a headless 24 GB Mac as a dedicated node
status: Draft
created: 2026-09-13
submitted:
needs: 0053
---

## Problem

The second Mac VISION names is assumed to have a desk, and a 24 GB M5 Pro bought to do
nothing but serve has none. Q4_K_M of this model wires about 19.9 GB at 32,768 with the
projector off, the cap Metal derives on 24 GB is below the 17.4 GB weights file, and
`gpuraise.sh` refuses the raise that would admit it because its 8 GiB reserve was written
for a laptop with an editor and a browser. Nothing here can say whether the node carries
the model, at which context, or what it costs against the laptop.

## Non-goals

- The client side: endpoint, sandbox, refusing to start a remote server. 0053, whose Mac
  to Mac chain is the row this node's first session lands on.
- More than one slot. The node has one user; sharing is measured on the GB10 in 0054.
- A lighter quant. The envelope is Qwen3.8-27B at Q4_K_M, the quant every quality number
  here is for. A node that cannot carry it at 16,384 with every lever is recorded as not
  carrying this model, and a different quant is a new feature citing this one.
- A team's uptime: drain, announce, per-user slots. 0055, and for one user a restart on
  failure is the whole requirement.
- A faster machine. The M5 Pro's bandwidth is 307 GB/s against the laptop's 400, so
  decode parity is the hypothesis and prefill is where any gain is; the tier-1 rows say.

## Design

The reserve is a reading, not a judgement. `config/machine-m5pro-24gb.json` carries
`reserve_gb`, and `config/machine.json` gains the same field at 8 so the laptop's number
keeps one home. `scripts/gpuraise.sh` and `scripts/rungs.sh` read it from the machine
file named by `MACHINE`, defaulting to `config/machine.json`, and `RESERVE_GB` in the
environment still wins for a one-off. The node's value is set from the anonymous memory
`memprobe.sh` reads on the idle node at the login window plus a margin, so the cap it
licenses leaves the system what it was measured to need rather than what a laptop needs.
The `Machine` struct in `internal/eval/machine.go` does not read it: the sweep floor and
the ceiling are its concern, and the reserve is the raise's.

The cap is the node's standing state, applied at boot. `iogpu.wired_limit_mb` does not
survive a reboot, so a LaunchDaemon under `scripts/node/` applies the raise and then runs
`serve.sh` on the committed node config, with `KeepAlive` on failure. A daemon rather than
a login agent because the node is not logged in; a daemon rather than a login-time shell
because an SSH session's death is how a foreground `make serve` ends. `stop.sh` boots the
daemon out when it is loaded and falls back to the process otherwise, which is 0054's
systemd decision on the platform it has.

The projector is the first lever, and it is a config setting. `serve.sh` gains
`NO_MMPROJ`, passed as `--no-mmproj` only when set: TECH measured the projector at 1.02 GB
and no text-only session loads it. `config/node-32k.env` is `driver-mtp-32k-stock.env`
with the projector off, because the draft head is the served default for a chain and the
node config should be the laptop's with one line moved. Whether the head fits at 24 GB is
the screen's first question; the same config without the head is the fallback and the
second screen. KV at `q4_0` and 16,384 are the remaining levers, in that order, each a
screen rather than a guess: 0003 measured `q4_0` 0.47 GB under `q8_0` at 32,768.

The node has one profile and it is headless. The machine file's single profile is
`headless`, its ceiling from the ladder. Every run on the node uses condition `headless`,
which `deskverdict.py` already reads as `not_applicable`, so the screen and ladder need
no change for a machine with no desk; `min_headroom_gb` is the node's own, measured.
Memory is expected to bind before time on 24 GB for the first time in this repo, and the
ladder is what says so.

The node is prepared once, and the preparation is checked by a number. Logged out at the
login window, Spotlight indexing off, sleep off on lid and power, automatic updates off.
README lists the commands; what makes it a checklist rather than folklore is the idle
`anonymous_gb` reading recorded in TECH beside the reserve derived from it, which a
teammate re-takes to know their node matches.

The link is a Thunderbolt bridge with fixed addresses. Both ends get a manual address on
the bridge interface, recorded in README, because a self-assigned address changes and
0053's sandbox admits the resolved address and nothing else. The bind is `HOST` from the
environment as 0053 settles; the node config's own `HOST` stays loopback.

Rows from the node carry the machine in the file name, `<date>-m5pro-24gb-<what>.jsonl`,
which is the convention `docs/data/README.md` already fixes. 0054 puts the machine on
the row itself; until it merges the file name is the record, as it is for the laptop.

## Tasks

- [ ] `reserve_gb` lives in each machine file, `gpuraise.sh` and `rungs.sh` read it from the file `MACHINE` names with `RESERVE_GB` still winning, and the laptop's 8 has one home
- [ ] `serve.sh` passes `--no-mmproj` when `NO_MMPROJ` is set, and `config/node-32k.env` is the stock draft-head driver config with the projector off
- [ ] A LaunchDaemon under `scripts/node/` applies the cap and serves the node config at boot with restart on failure, and `stop.sh` boots it out when it is loaded
- [ ] `config/machine-m5pro-24gb.json` holds the node's measured reserve, floor and one `headless` profile, and TECH records the idle anonymous reading the reserve came from
- [ ] The screen runs the node config with the head and without at 32,768 under the raised cap, and TECH records wired, headroom and which levers Q4_K_M needed to serve on 24 GB
- [ ] The ladder runs on the node and TECH records which of memory or time binds on 24 GB, with the ceiling written into the machine file
- [ ] The tier-1 suite runs on the node under the laptop's settings and TECH records prefill, decode and peak wired beside the laptop's, head on and off
- [ ] README documents the node: preparation with its check, the bridge addresses, the daemon, and a `smoke` from the laptop over the bridge

## Log
