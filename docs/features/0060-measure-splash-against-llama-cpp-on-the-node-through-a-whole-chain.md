---
id: 0060
title: Measure Splash against llama.cpp on the node, through a whole chain
status: Draft
created: 2026-09-19
submitted:
needs:
---

## Problem

0058 measured DFlash2 on the node accepting 6.7 to 6.95 tokens a step and returning only
1.94x to 2.09x, so most of what the drafter proposes is spent on how llama.cpp verifies
it. Splash (github.com/incoai/splash, Apache-2.0) is a runtime built around Qwen3.8-27B
with that drafter as its only decode path, and it serves `/v1/messages`, which is the
first of the two things TECH holds against MLX. Its vendor reads 74 tok/s short and 54 at
32k on an M5 Pro with two thirds of the node's bandwidth, against the node's 35 to 37
served today. None of that is a reading from this machine, and no runtime but llama.cpp
has ever been driven through a chain here.

## Non-goals

- Changing what the node serves. `com.localcode.serve`, `stop_server` and `localcode
  status` all read llama.cpp; adoption is a feature of its own citing this one's rows.
- The laptop. Splash needs an M3 or newer and 36 GB, so the M2 Max is excluded by its
  requirements rather than by measurement.
- Concurrency. The vendor's four-request figure is 0054's question, on the GB10.
- Splash's second model, and any model other than Qwen3.8-27B.
- A lossless claim. Splash serves its own 4-bit package, so the fidelity hash cannot match
  Q4_K_M and every ratio here is a runtime comparison like 0006's, void as a speculative one.

## Design

Everything runs on the node in 0058's machine state: nobody logged in, the daemon's
server stopped, one toggle at a time, rows under `docs/data/` with the machine in the
file name.

`runtimes/splash/setup.sh` installs the vendor's Homebrew formula and prints the version
it got, idempotently; nothing is installed on the node by hand. The weights are staged
into the HuggingFace cache over the 0053 link, as 0058's were, because the node's own
route to the CDN cannot deliver them. `runtimes/splash/serve.sh` reads
`runtimes/splash/config/splash-27b.env` and adds no flags of its own, as
`runtimes/mlx/serve.sh` does. The config pins `--max-context` to 49,152 and
`--max-memory` to the cap `scripts/node/cap.sh` applies, so both runtimes are held to
one window and one ceiling.

What Splash reports is probed before anything is measured, because every later box
depends on it and its README documents none of it: whether it answers `/props`,
`/metrics`, `/v1/models` or `/v1/messages/count_tokens`; whether responses carry usage
and timings; whether thinking can be turned off and sampling sent per request; whether
it refuses a prompt past `--max-context` or truncates it; and whether Claude Code's
system messages survive its template, read through `/apply-template`. TECH records the
answers. A chain is driven only if thinking can be off and an over-long prompt is
refused; otherwise the feature ends at the pair and says why.

Two baselines, not one. `config/node.env` is what the node serves, and answers whether
Splash beats the default. `config/dflash2-node-49k.env` is the same drafter on
llama.cpp, and answers how much of the gap is the runtime rather than the drafter. The
vendor compared against neither.

The measurements, in the order that lets each stop the next: `scripts/screen.sh` filled
at 49,152 for admissibility, with minimum free memory read beside peak wired because
0058 found a per-request allocator spends the free pool rather than the cap;
`scripts/pair.sh` with `CANDIDATE_SERVE` on the ranking suite for client-side decode,
pass rate and tool-call validity; `runtimes/mlx/compare.sh` over the depth suite for
prefill and for the repeated 32k fixture, which is where prefix reuse shows. That script
gains a label prefix in place of its hardcoded `0006`; moving it out of `runtimes/mlx/`
is not worth a rename.

The chain is the verdict, per VISION: three cold chains a side on the twenty-bug
fixture through `scripts/chainrun.sh`, Splash against `config/node.env`, driven on the
node. `chainrun.sh` takes the serve command the way `pair.sh` does. The launcher budgets
a session from the context `/props` reports and refuses an endpoint that reports none.
If Splash reports none, `cmd/localcode/main.go` gains `-context n`, honoured only when
`/props` does not answer and refused when it does, so a typed number can never override
a reading. A shim answering `/props` in front of Splash was rejected: it would present
a claim as a reading on every row. Rows from such a chain carry `served_n_ctx` 0, which
is the truth.

TECH gets one table: runtime, admissible, peak wired, minimum free, decode short and at
32k, cold prefill, warm 32k repeat, pass, tool-call validity, chain wall and sessions.

## Tasks

- [x] `runtimes/splash/setup.sh` installs Splash on the node idempotently and prints its version, the weights are staged over the link, and `runtimes/splash/serve.sh` serves `runtimes/splash/config/splash-27b.env`, covered by shellcheck and the gate
- [x] The external review's findings on the first box are closed: `setup.sh` tells an absent Splash from one brew cannot list and never installs over it, a Splash that cannot print its version is not reported ready, `serve.sh` reads a relative config from the checkout only, and the offline claim is held to one snapshot
- [x] Splash's endpoints, usage reporting, thinking switch, per-request sampling, behaviour past `--max-context` and template handling are probed on the node, and TECH records each answer and whether a chain may be driven
- [ ] Splash is screened filled at 49,152 and TECH records admissibility, peak wired, minimum free memory and swap delta beside 0056's baseline row
- [ ] Splash is paired on the ranking suite against `config/node.env` and against `config/dflash2-node-49k.env`, three passes a side, and TECH records decode, pass rate and tool-call validity with the ratios marked as runtime comparisons
- [ ] `runtimes/mlx/compare.sh` takes its label prefix from the environment, and the depth suite is scored on both runtimes with TECH recording cold prefill by depth and the repeated 32k fixture
- [ ] `scripts/chainrun.sh` takes the serve command, and if Splash reports no context the launcher gains `-context`, refused whenever `/props` answers, covered by a test on both branches
- [ ] Three cold chains a side run on the node, Splash against `config/node.env`, and TECH carries the one table and says whether an adoption feature is warranted, with the rows that decide it cited

## Log

- 2026-09-19: the config records the address Splash binds instead of setting it. The plan
  has `splash-27b.env` pin host and port the way every other runtime's config does, and
  `splash serve` takes neither flag: its launcher binds `127.0.0.1:8000` and refuses to
  start when anything else owns that address. They stay in the file because a row has to
  say where it was driven, and `serve.sh` refuses a config naming anything else rather
  than let a row carry an address nothing bound.
- 2026-09-19: `serve.sh` exports `HF_HUB_OFFLINE=1` and passes no flag for it. The plan has
  it add no flags of its own, and Splash checks the Hub for the repository's `main` before
  every load, which is the stall 0058 measured on this node's route. Offline it falls back
  to the staged snapshot and verifies it against the package manifest either way, so this
  is 0058's `LLAMA_ARG_OFFLINE=1` on the other side of the pair. Overridable, because the
  first install on a machine has to reach the Hub once.
- 2026-09-19 — paused: box 2 needs the node's daemon server stopped, which needs the owner's sudo until 0061 is installed there
- 2026-09-19: a chain may be driven. Thinking can be turned off and a prompt past the window
  is refused, which are the two conditions Design set. The served context is on `/status`, so
  the launcher reads it from there when `/props` answers 404 and no `-context` flag is added:
  a reading beats a typed number, which is Design's own reason for refusing a shim.
- 2026-09-19: every pair runs at `presence_penalty` 0 on both sides. Splash refuses the
  penalty with HTTP 400, so 0005's settled sampling cannot be sent to it, and a pair whose
  sides sample differently measures the sampling. The baseline is sent an explicit 0 over the
  1.5 its config serves. The chains are not held to this: they compare what each runtime
  serves, and a runtime that cannot take the penalty 0005 found necessary is what is judged.
- 2026-09-19: the node has a console session logged in for every row, recorded as condition
  `logged-in`. 0058's state is nobody logged in, but the USB link needs a session on the node
  after a reboot and the owner is away, so logging out risks the link for the whole run. Both
  sides of every pair share the state; memory headroom reads about 3 GB worse than 0056's.
