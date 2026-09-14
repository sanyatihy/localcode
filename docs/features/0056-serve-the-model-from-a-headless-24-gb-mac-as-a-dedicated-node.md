---
id: 0056
title: Serve the model from a headless 24 GB Mac as a dedicated node
status: Draft
created: 2026-09-13
submitted:
needs: 0053
---

## Problem

A 24 GB M5 Pro bought to do nothing but serve has no desk, and every gigabyte macOS
keeps is a gigabyte of context the model does not get. Q4_K_M wires about 19.9 GB at
32,768 with the projector off, the cap Metal derives on 24 GB is below the 17.4 GB
weights file, and `gpuraise.sh` refuses the raise because its 8 GiB reserve was written
for a laptop with an editor and a browser. Nothing here says what macOS needs on a
node that only serves, which levers cut it, or what the model gets after them.

## Non-goals

- The client side: 0053.
- More than one slot. The node has one user; sharing is 0054.
- A lighter quant. The envelope is Qwen3.8-27B at Q4_K_M. A node that cannot carry it at
  16,384 with every lever is recorded as not carrying this model; another quant is a new
  feature citing this one.
- A team's uptime: 0055. For one user, restart on failure is the requirement.
- A faster machine. The M5 Pro's bandwidth is 307 GB/s against the laptop's 400, so
  decode parity is the hypothesis; the tier-1 rows say.
- Disabling swap, SIP or the memory compressor. A node that panics under pressure is
  worse than one that swaps once, and the ladder's zero-swap rule makes swap visible.

## Design

The node's budget is total memory less what macOS keeps idle, and both are readings.
`scripts/node/prepare.sh` applies every OS lever below, then prints the idle
`anonymous_gb` and `wired_gb` from `memprobe.sh` at the login window and compares them
with the numbers recorded in TECH; a node that reads higher than the record is not
prepared. The reserve in the machine file is that idle reading plus 1 GiB, and the
raise `gpuraise.sh` applies is total less the reserve.

The node is SSH-only, lid closed. Remote Login is the one service on, restricted to
the serving user with key authentication only; Screen Sharing, Remote Management, file
sharing and every other sharing service are off; the application firewall is on in
stealth mode admitting `sshd` and the server binary only. `pmset` sets `disablesleep 1`
so the closed lid does not sleep the node, `sleep 0` on power, and `autorestart 1` so
a power failure ends in a serving node. Wi-Fi stays on and joins from the system
network profile at the login window, so the node is reachable from anywhere on the
network; the Thunderbolt bridge is the link the client uses.

Each OS lever is measured on its own, before and after, by the idle reading, and TECH
records its saving in a table. Levers, in the order they are applied: logged out at
the login window; Apple Intelligence, Siri and Spotlight indexing off; iCloud, Handoff,
AirPlay and every sharing service but Remote Login off; Time Machine, automatic
updates and the update daemon off; Bluetooth off; the screen saver off, and the lid
closed with no display attached. A lever whose saving reads under 50 MB is dropped
from the script and the table says so.

Each server lever is measured by the screen, one at a time, from the node config:
`NO_MMPROJ`, which `serve.sh` gains and passes as `--no-mmproj` (1.02 GB in TECH);
`CACHE_RAM`, bounded rather than left at the 8192 MiB default, because that budget
fills lazily and a long session would fill it out of the model's memory; `UBATCH_SIZE`
at 256 and 512, since the compute buffer is sized by the physical batch; KV at `q4_0`,
0.47 GB under `q8_0` at 32,768 (0003); and the draft head on and off. `MLOCK`, passed
as `--mlock`, is screened last for whether pinning the weights changes peak wired or
decode; the finding is recorded either way.

Each machine file carries `reserve_gb`; `config/machine.json` gets 8. `gpuraise.sh` and
`rungs.sh` read it from the file `MACHINE` names, default `config/machine.json`, and
`RESERVE_GB` in the environment still wins. `internal/eval/machine.go` does not read it.

Two LaunchDaemons under `scripts/node/`: one runs as root and applies the cap at boot,
because `iogpu.wired_limit_mb` does not survive a reboot and the sysctl needs root; the
other runs as the serving user, whose home holds the HuggingFace cache, and runs
`serve.sh` on the node config with `KeepAlive` on failure. `gpuraise.sh` treats a raise
to the value already set as a no-op, so a restart after a crash does not fail on its
own cap. `stop_server` in `lib.sh` boots the serving daemon out through `STOP_CMD` when
it is loaded and stops the process otherwise.

`config/node-32k.env` is `driver-mtp-32k-stock.env` with the projector off and the
server levers the screen settled: 32,768 is the largest context the screen can start
from on 24 GB, and the draft head is admitted there (0025). `agent.env` stays the
default on machines that carry 49,152.

The screen runs the node config at 32,768 filled and with a saved prefix in the prompt
cache, since that cache costs nothing at load. A pass is a filled context with zero
swap delta and headroom at or above the node's floor. If 32,768 fails with every
lever, 16,384 is screened; every branch failing is recorded as the node not carrying
this model. The ladder then runs from explicit `CELLS`, because the derivation in
`rungs.sh` carries the laptop's ingest coefficients, and its top passing rung is the
machine file's ceiling.

The machine file's single profile is `headless`, ceiling from the ladder. Every node
run uses condition `headless`, which `deskverdict.py` reads as `not_applicable`.
`min_headroom_gb` is measured on the node.

The link is a Thunderbolt bridge with manual addresses at both ends, recorded in
README, because 0053's sandbox admits the resolved address only. The serving daemon
sets `HOST` per 0053; the node config keeps loopback.

Rows from the node carry the machine in the file name, `<date>-m5pro-24gb-<what>.jsonl`,
per `docs/data/README.md`, until 0054 puts it on the row.

## Tasks

- [ ] `scripts/node/prepare.sh` applies every OS lever, prints the idle anonymous and wired readings against TECH's record, and exits nonzero when the node reads higher
- [ ] Each OS lever is measured on its own by the idle reading, TECH records the table, and a lever under 50 MB is dropped from the script
- [ ] The node answers SSH by key as the serving user and nothing else: a port scan from the laptop shows `sshd` and the server only, and a password login is refused
- [ ] The node serves with the lid closed on power and comes back serving after a power cut, each verified by a `smoke` over the bridge
- [ ] `reserve_gb` lives in each machine file, `gpuraise.sh` and `rungs.sh` read it from the file `MACHINE` names with `RESERVE_GB` still winning, and the laptop's 8 has one home
- [ ] `serve.sh` passes `--no-mmproj` when `NO_MMPROJ` is set and `--mlock` when `MLOCK` is set, covered by shellcheck and the gate
- [ ] `gpuraise.sh` treats a raise to the value already set as a no-op, covered by a test
- [ ] Two LaunchDaemons under `scripts/node/` apply the cap as root and serve the node config as the serving user at boot with restart on failure, and `stop_server` boots the serving daemon out when it is loaded
- [ ] Each server lever is screened one at a time at 32,768 filled with a saved prefix, and TECH records peak wired, headroom and decode per lever
- [ ] `config/node-32k.env` carries the levers the screen settled, and TECH records whether Q4_K_M fits on 24 GB at 32,768 or 16,384, or does not fit
- [ ] `config/machine-m5pro-24gb.json` holds the node's measured reserve, floor and one `headless` profile, and TECH records the idle reading the reserve came from
- [ ] The ladder runs on the node from explicit `CELLS` and TECH records which of memory or time binds on 24 GB, with the ceiling written into the machine file
- [ ] The tier-1 suite runs on the node under the laptop's settings and TECH records prefill, decode and peak wired beside the laptop's, head on and off
- [ ] README documents the node: `prepare.sh` and its check, the SSH-only access, the bridge and Wi-Fi addresses, the daemons, and a `smoke` from the laptop over the bridge

## Log
