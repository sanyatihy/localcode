---
id: 0054
title: Serve and measure on a GB10 for several users at once
status: Draft
created: 2026-09-13
submitted:
needs: 0053, 0056
---

## Problem

Everything here serves and measures on macOS: no CUDA build of the server exists, the
memory, thermal and backend probes shell out to `vm_stat`, `pmset` and `lsof`,
`rungs.sh` and `ladder.sh` read `sysctl`, `stat -f` and `memprobe.sh`, the machine file
is one laptop, and one slot serves one developer. A GB10 (128 GB unified, Blackwell GPU,
Arm Linux, headless) is arriving to serve four developers, and nothing here can run
its gate, build its server, walk its ladder, or record which machine a row came from.

## Non-goals

- A second runtime. llama.cpp with CUDA is the runtime; every tool reads its `/props`,
  `/metrics` and `/slots`. vLLM or SGLang is a BACKLOG A/B.
- A larger model or quant. The scorer decides, in its own feature.
- Authentication and quotas: VISION fixes the network as trusted.
- The desktop probes. The box has no compositor; `deskprobe.sh` and the attended
  verdict stay macOS-only.
- Uptime for a team: drain, alerting, reboot to ready. 0055.

## Design

`scripts/gb10/build-llama.sh` builds llama.cpp from a pinned commit with `GGML_CUDA=ON`
for the GB10's compute capability, into a path `SERVER_BIN` names. TECH records the
commit and CUDA toolkit version beside the Mac's Homebrew build. The `qwen35` check runs
`strings` over `libllama.so`. `smoke` passing is not proof of CUDA: the banner's backend
and `nvidia-smi` showing the process are.

The server is a systemd unit for a service user whose home holds the HuggingFace
cache. It runs `serve.sh` on the committed GB10 config with `HOST=0.0.0.0` in the
unit's environment and `Restart=on-failure`. `stop_server` in `lib.sh` stops the unit
through `STOP_CMD` when one is active, so every script that stops a server stops the
unit rather than a process the unit would restart. 0056 puts the same dispatch in for
launchd; this adds the systemd branch.

The memory sample reads `/proc/meminfo`: `MemTotal`, `MemAvailable`, swap as total less
free. Headroom is per platform: total less wired and anonymous on macOS, `MemAvailable`
on Linux. A sample missing any field its headroom needs is not OK and the preflight
refuses it. Whether CUDA allocations on unified memory appear in `/proc/meminfo` is
measured on arrival; if not, the sample also reads `nvidia-smi --query-gpu=memory.used`
and TECH records the rule. The thermal sample records unknown on Linux. The backend
probe already handles `libggml-cuda.so`.

`rungs.sh` and `ladder.sh` read total memory, the weights size and the apparatus
through one function per platform in `lib.sh`. On the GB10 the rungs are given
explicitly through `CELLS`; the derivation in `rungs.sh` carries the laptop's ingest
coefficients. The ladder fills every slot at once, not one, and reads `total_slots`
from `/props` to tell a per-slot `n_ctx` from a total.

`config/machine-gb10.json` holds the box's floor and one `shared` profile with a
ceiling from the ladder. The eval writes the machine file's `machine` name onto every
row; no row carries it today, and the report refuses to summarise two machines as one.

`config/gb10-agent.env` is `agent.env` with `PARALLEL="4"` and a context sized for four
slots; `HOST` stays loopback and `CACHE_RAM` stays at the default until a measurement
moves it. Whether `--ctx-size` is total or per slot is read from the server README at
the pinned commit and recorded in the config's comment.

The ladder runs 49,152, 65,536, 98,304 and 131,072 per slot, at one slot and at four,
recording peak memory, swap delta and cold ingest per rung. A rung passes when every
slot fills with zero swap delta and no request fails. The per-slot window is the
largest passing rung, and its cold ingest is recorded as its cost; if 131,072 passes
it is the window. Whether the model serves 131,072 without RoPE scaling is read off
its card at that rung; a rung that needs scaling is a different config.

The launcher's session budget is one slot's share: it reads `n_ctx` and `total_slots`
from `/props`, and refuses a response it cannot resolve to a per-slot number.

Four chains on the fixture run at once, one per client Mac, and the server log is read
as 0018 read it, with each request attributed to its client by matching the client's
transcript timestamps, tested on an interleaved log. Recorded per client: decode,
ingest, queue wait, failed requests, task outcome, and how often a prefix survived
another's traffic; recorded for the box: peak memory and swap.

GitHub's Arm Ubuntu runner runs `make check` beside the existing job.

The box is not reachable yet. The first four boxes need no box; the rest wait for SSH
access.

## Tasks

- [x] `make check` passes on Arm Linux in CI beside the existing job, and every macOS-only probe records unknown rather than a wrong number
- [x] The memory probe reads `/proc/meminfo`, headroom is defined per platform, a sample missing a needed field is refused, and a test covers each platform's parser
- [x] Every results row names its machine, read from the machine file, and the report refuses to summarise rows from two machines as one
- [x] `rungs.sh` and `ladder.sh` read the machine through per-platform functions in `lib.sh`, the ladder fills every slot at once and reads `total_slots`, and a test covers the per-slot and total shapes of `/props`
- [ ] `scripts/gb10/build-llama.sh` builds the pinned CUDA llama.cpp on the box, the `qwen35` check passes, `make smoke` passes over the LAN, and the banner and `nvidia-smi` show the CUDA backend serving
- [ ] The server runs as a systemd unit for a service user on the committed GB10 config, and `stop_server` stops the unit when one is active, from `stop.sh` and from the measurement scripts alike
- [ ] `config/machine-gb10.json` holds the box's measured floor and one profile, and the tier-1 suite runs on the GB10 under the laptop's settings with prefill, decode and peak memory in TECH beside the laptop's
- [ ] The launcher's session budget is one slot's share on a four-slot server, refusing a `/props` it cannot resolve, covered by a test on both shapes
- [ ] The ladder runs to 131,072 per slot at one slot and at four with the pass rule above, and TECH records what binds on 128 GB, the cold ingest per rung, and whether the top rung needed RoPE scaling
- [ ] Four chains run at once from four clients against `PARALLEL="4"`, requests are attributed per client, and TECH records per-client decode, ingest, queue wait, failures, outcome and prefix survival, with the box's peak memory and swap
- [ ] `config/gb10-agent.env` carries the per-slot window the ladder settled and `config/machine-gb10.json` its ceiling
- [ ] README documents the GB10 server setup and points the client side at 0053
