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

Superseded in part by the 2026-09-14 Log entries: the node is a 36 GB M5 Max on a USB
link, serves `config/node.env` at 49,152, and runs three daemons.

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

- [x] `scripts/node/bootstrap.sh` takes the node from a clean macOS to an installed checkout and a passing `qwen35` check, idempotently, and a second run changes nothing
- [x] `scripts/node/prepare.sh` applies every OS lever, prints the idle anonymous and wired readings against the machine file's record, and exits nonzero when the node reads higher
- [x] The OS levers are measured as a set by the idle reading at the login window, TECH records the saving, and a lever leaves the script only when it breaks something
- [x] The node answers SSH by key as the serving user: a full TCP scan from the laptop shows port 22, the server's 8081 while the daemon serves and one Apple built-in listener on a high random port, and a password login is refused
- [ ] The node serves with its own lid closed on power and comes back serving after a power cut and a login, each verified by a `smoke` over the link
- [x] `reserve_gb` lives in each machine file, `gpuraise.sh` and `rungs.sh` read it from the file `MACHINE` names with `RESERVE_GB` still winning, and the laptop's 8 has one home
- [x] `serve.sh` passes `--no-mmproj` when `NO_MMPROJ` is set and `--mlock` when `MLOCK` is set, covered by shellcheck and the gate
- [x] `gpuraise.sh` treats a raise to the value already set as a no-op, covered by a test
- [x] Three LaunchDaemons under `scripts/node/` hold the link address, apply the cap as root, and serve the node config as the serving user at boot with restart on failure, and `stop_server` boots the serving daemon out when it is loaded
- [x] Each server lever is screened one at a time at 49,152 filled with a saved prefix, then the context is raised rung by rung until the ladder's pass rule fails; TECH records peak wired, headroom and decode per lever and rung
- [x] `config/node.env` carries the levers and the context the screen settled
- [x] `config/machine-m5max-36gb.json` holds the node's measured reserve, floor and one `headless` profile, and TECH records the idle reading the reserve came from
- [x] The ladder runs on the node from explicit `CELLS` and TECH records which of memory or time binds on 36 GB, with the ceiling written into the machine file
- [x] The tier-1 suite runs on the node under the laptop's settings and TECH records prefill, decode and peak wired beside the laptop's, head on and off
- [x] The node is driven from itself and from the laptop, three cold repetitions each, and TECH records chain wall, per-call latency and server rates side by side, per 0053's Design
- [x] README is the node's runbook in order: the manual steps macOS forces and why, `bootstrap.sh`, `prepare.sh` and its check, `install.sh` and the daemons, the SSH-only access, the link and Wi-Fi addresses, and a `smoke` from the laptop over the link

## Log
- 2026-09-14 — took 0053's Mac-to-Mac measurement box: the second Mac is this node.
- 2026-09-14 — the idle record prepare.sh checks itself against lives in the machine file, as `idle_anonymous_gb` and `idle_wired_gb`, rather than in TECH: a script cannot read a number out of prose, and every other limit a machine imposes is already in `config/machine*.json`. TECH still records the reading and the levers it came from. Box 1 says the machine file.
- 2026-09-14 — paused: the remaining boxes need the node
- 2026-09-14 — paused: the remaining boxes need the node
- 2026-09-14 — the node is not the machine this plan assumed. It is a Mac17,6: Apple M5 Max,
  32 GPU cores, 36 GB, macOS 26.5.2, 1.8 TB free. Problem's 24 GB arithmetic is void — the
  node has more room than the 32 GB laptop and carries `config/agent.env` at 49,152 — so the
  screen no longer asks whether Q4_K_M fits at 32,768 but how far above 49,152 the node
  reaches, and the node config starts from `agent.env`. The memory levers stand unchanged:
  every gigabyte macOS keeps is context.
- 2026-09-14 — the owner requires the node as infrastructure as code. Every step from a clean
  macOS to a serving node is a script here, idempotent and re-runnable, with only what macOS
  makes unavoidable left manual and each of those named in README. The node has nothing
  installed on it and is the clean-state test of `scripts/node/bootstrap.sh`, which is a new
  box. Steps needing root are run by the owner over SSH with `sudo`; no sudoers or
  passwordless-sudo mechanism is added.
- 2026-09-14 — the link is USB, not Thunderbolt: the cable negotiated USB 2 at 480 Mb/s and
  Thunderbolt Bridge is inactive. The laptop is `en14` (AppleUSBNCMData, which `networksetup`
  cannot manage, having no hardware port) at 10.99.0.1/24 and the node is `anpi2` at
  10.99.0.2/24, both set with `ifconfig` and neither surviving a reboot — so the node's
  address is a LaunchDaemon and the laptop's is a manual step in README. The node's Wi-Fi is
  <its Wi-Fi address> with internet. The boxes that said bridge say link, and the ladder box says
  36 GB.
- 2026-09-14 — paused: the remaining boxes need the node
- 2026-09-14 — paused: the remaining boxes need the node
- 2026-09-14 — the reserve is idle anonymous plus wired plus 1 GiB, not anonymous plus 1 GiB: the cap is bought from the same 36 GB the kernel's wired pages sit in, and a reserve that ignored them would let the raise plus the kernel exceed the machine. Measured idle 2.60 GB anonymous, 1.93 GB wired; reserve 6.
- 2026-09-14 — the per-lever table is dropped: attributing the saving lever by lever costs fourteen root toggles and readings at the login window, and on 36 GB no lever is worth removing for what it saves. The set is measured instead: 3.98 GB to 2.60 GB anonymous and 2.05 GB to 1.93 GB wired, the first reading still carrying Setup Assistant's session and an iCloud login.
- 2026-09-14 — the SSH box said the scan would show sshd and the server and nothing else. It
  shows a third port, 63198, held by a root-owned Apple daemon the firewall admits as
  built-in software and `prepare.sh` does not remove; the owner decided not to chase it, so
  the box now says what a prepared node exposes.
- 2026-09-14 — the ladder's pass rule never failed. Memory bound no rung the node could be
  waited for: marginal KV held at 39 KB/token from 8,192 to 163,840, where 5.81 GB of the
  raised cap was still free and swap was flat. What ended the walk was the ingest budget
  `rungs.sh` applies — 1,033 s cold at 163,840, and 196,608 stopped at half its fill after
  35 minutes rather than finished. The ceiling is the largest rung walked to a completed
  fill, and the box says memory or time binds, which it now answers.
- 2026-09-14 — the screen half of the lever box is done and the box stays open: it also asks
  for decode per lever and per rung, and the only decode reading so far is one 256-token
  completion per screened cell, which two settings-identical cells put a 5.9% spread on. The
  decode-by-depth curve 0017 measured the laptop's table with is what closes it — node.env
  against the draft head at ~200, 8k, 16k and 32k — and it is not run.
- 2026-09-14 — the lid box says the node's own lid, and a login after a power cut: closing both lids showed the node stays awake but the USB link needs a login at its screen after the laptop sleeps or the node boots. TECH records it.
- 2026-09-14 — the three rungs the first walk left suspect were re-walked from a rested
  machine and the anomaly was the machine: 14, 31 and 73 s against 64, 117 and 224, with peak
  wired unchanged. The ceiling and the floor the machine file carries are unaffected — they
  come from the top rung, which was walked rested — and the node's ingest is now comparable
  with the laptop's at every matched rung.
- 2026-09-14 — decode is recorded as a curve over prompt depth rather than per rung, because
  0034 measured that decode falls with the depth a call works at and not with the context the
  server reserves: per-rung decode would re-measure that at nine contexts. The curve is
  0017's, on node.env with the draft head on and off at ~200, 8k, 16k and 32k. The screen's
  per-lever decode column stays one sample a cell, with the spread two settings-identical
  cells put on it.
- 2026-09-14 — node.env's settings do not move: the screen found one lever worth setting and
  it was already set. The projector stays off, CACHE_RAM, UBATCH_SIZE, the KV type and MLOCK
  are all left alone with the readings that say why, and the context stays 49,152 because the
  ladder measured what the machine allows and 0008 chose this for what a session needs. The
  draft head clears the adoption bar on decode here and is still not set: what it does to a
  chain on this node is unmeasured.
- 2026-09-14 — the first three chain attempts on the node stopped on the launcher's sandbox dropping `~/.claude` on a fresh machine (no transcript, so no budget); fixed in the launcher with a test, and the six chains recorded were run after the fix.
