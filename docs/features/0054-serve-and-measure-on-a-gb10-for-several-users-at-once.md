---
id: 0054
title: Serve and measure on a GB10 for several users at once
status: Draft
created: 2026-09-13
submitted:
needs: 0053
---

## Problem

Everything here serves and measures on macOS: the server is Homebrew's Metal build, the
memory, thermal and backend probes shell out to `vm_stat`, `pmset` and `lsof`, the machine
file is one laptop, and one slot serves one developer. A GB10 desktop — 128 GB unified,
Blackwell GPU, Arm Linux, headless — is arriving to serve four developers at once, and
nothing in the repo can run its gate, start its server, or record a row that says which
machine it came from.

## Non-goals

- A second runtime. llama.cpp with CUDA is the first and only runtime here: every tool
  reads its `/props`, `/metrics` and `/slots`. vLLM or SGLang is a later A/B, as 0006 was
  for MLX, and goes to BACKLOG.
- A larger model or quant. 128 GB admits them; the scorer decides, in its own feature.
- Authentication and quotas — VISION fixes the network as trusted.
- The desktop probes. There is no compositor on a headless box, so `deskprobe.sh` and
  the attended verdict stay macOS-only and the GB10's machine file carries one profile.

## Design

**The server is a pinned from-source CUDA build.** No package ships llama.cpp with CUDA
for Arm Linux, so `scripts/gb10/build-llama.sh` clones a named commit and builds with
`GGML_CUDA=ON` for the GB10's compute capability, and TECH records the commit and the
CUDA toolkit version beside the Mac's Homebrew build. The `qwen35` check becomes a
`strings` over `libllama.so`. `serve.sh` is unchanged: it already takes `SERVER_BIN`.

**The server is a systemd unit, and `stop` knows it.** A foreground `make serve` dies
with the SSH session. The unit runs `serve.sh` on the committed GB10 config with
`Restart=on-failure`; `stop.sh` stops the unit when one is active and falls back to the
process otherwise, so the wait for memory has one home still.

**The probes answer for Linux or say they cannot.** The memory sample reads
`/proc/meminfo` — `MemTotal`, `MemAvailable`, and swap as total less free — and its
headroom is `MemAvailable`, because Linux does not keep free memory near zero the way
macOS does. Whether CUDA allocations on unified memory appear there is measured on
arrival; if they do not, the sample also reads `nvidia-smi --query-gpu=memory.used` and
the headroom rule in TECH says so. The preflight refuses on a sample that did not
answer, as it does today. The
thermal sample records unknown on Linux rather than a wrong 100; `nvidia-smi` throttle
reasons are a later box if a run ever needs them. The backend probe already handles
`libggml-cuda.so`.

**Every row names its machine.** `config/machine.json` gains nothing; the GB10 gets
`config/machine-gb10.json` with its own floor and one `shared` profile whose ceiling
the ladder measures. The eval reads the machine file's `machine` name onto each row,
which no row carries today. Two envelopes in one results file without that field is
the conflation VISION forbids.

**Slots are a config setting, four to begin with.** `config/gb10-agent.env` is
`agent.env` with `PARALLEL="4"`, a context sized for four slots, `HOST="0.0.0.0"` and a
prompt cache sized for 128 GB. Whether `--ctx-size` is the total across slots or per
slot changed during 2025, so it is read from the server README at the pinned commit and
recorded in the config's comment. The per-slot window is set from the ladder, not copied
from the laptop's 49,152: what binds at four slots on 128 GB is unknown, and a number
derived rather than observed is a hypothesis here as everywhere.

**Sharing is measured before it is offered.** 0018 found that a slot is chosen by prefix
similarity and a prefix the server has never seen ingests from zero; with four users
that rule decides whose conversation is evicted. Four chains on the fixture run at once
and the server's log is read the way 0018 read it: per-user decode, ingest, and how
often a user's prefix survived another's traffic.

**The gate runs on Arm Linux in CI so the port cannot rot.** GitHub's Arm Ubuntu runner
runs `make check` beside the existing job. No model, no CUDA: the gate needs neither.

**The box is not reachable yet.** The first three boxes need no box; the rest start
when SSH access to it exists, and a session that reaches them without it stops there.

## Tasks

- [ ] `make check` passes on Arm Linux in CI beside the existing job, and every macOS-only probe records unknown rather than a wrong number
- [ ] The memory probe reads `/proc/meminfo` so the preflight carries a verdict on Linux, with a test per platform parser and the headroom rule recorded in TECH
- [ ] Every results row names the machine it came from, read from the machine file, and the report refuses to summarise rows from two machines as one
- [ ] `scripts/gb10/build-llama.sh` builds the pinned CUDA llama.cpp on the box, the `qwen35` check passes, and `make smoke` passes against it over the LAN
- [ ] The server runs as a systemd unit on the committed GB10 config and `stop.sh` stops the unit when one is active
- [ ] `config/machine-gb10.json` holds the box's measured floor and its one profile, and the tier-1 suite runs on the GB10 under the laptop's settings with prefill, decode and peak memory recorded in TECH beside the laptop's
- [ ] Four chains run at once against `PARALLEL="4"` and TECH records per-user decode, ingest and prefix survival from the server's log
- [ ] The context ladder at four slots finds what binds on 128 GB and `config/gb10-agent.env` carries the per-slot window it settled
- [ ] README documents the GB10 server setup and points the client side at 0053
