---
id: 0002
title: Build the agentic coding eval harness
status: Draft
created: 2026-08-17
shipped:
check:
checked:
review:
needs:
related: 0010, 0001
---

## Problem

Every A/B here — quant against context, sampling, llama.cpp against MLX, model against
model, Pi against Hermes — needs one instrument that says which side won. Without it
they are all opinion, and the vision's "measurement before tuning" constraint is
unenforceable. What is missing is not an agent: Pi and Hermes already are agents. What
is missing is the thing that runs fixed tasks through one of them and records numbers.

## Non-goals

- **Not a coding agent, and not a custom harness.** The single most expensive mistake
  available here. Pi (`earendil-works/pi`) and Hermes (`NousResearch/hermes-agent`) are
  model-agnostic and already speak OpenAI-compatible endpoints. This feature drives one;
  it does not reimplement one. If neither can be driven headlessly, that is a finding for
  `## Open questions`, not a licence to start writing an agent loop.
- **Not a general LLM benchmark.** No MMLU, no leaderboard reproduction.
- **No comparison runs.** Building the instrument is this feature. Running sweeps with it
  is 0004 onward; shipping this means it *can* score a config, not that anything is scored.
- **No reimplementation of throughput benchmarking.** `llama-bench` and the server's own
  timings already measure tok/s. Wrapping beats rewriting.
- **No cloud baseline.** Scoring against a hosted model would send code off the machine.

## Design

Two things the original plan conflated, kept separate here because they change at
different rates:

- **The harness** is the agent loop — Claude Code, Pi, Hermes, OpenCode. Chosen, not written (0010).
- **The scorer** is this feature: a Go binary that runs fixed tasks through a harness
  against an endpoint and records what happened.

The scorer drives a harness through a **thin adapter**: how to invoke it non-interactively
with a task prompt in a given directory, and how to tell when it has stopped. Adapters for the
candidates 0010 compares, written together because an interface with one implementation is
an abstraction inventing itself — and Claude Code's is the one that must exist first, since
it is 0010's baseline and 0008's flow. Everything else — task
definitions, scoring, metrics, results — is shared.

**The suite is two tiers, because they cost different amounts and answer different
questions.** Tier 1 talks straight to the endpoint over HTTP: no harness, no agent loop,
seconds per task. It measures single-turn quality — does it call the right tool with valid
arguments, does its patch compile and pass, does it still retrieve correctly at depth — and
that is enough to sweep a toggle matrix. Tier 2 drives a real harness through a multi-turn
task, which is the only way to see coherence and recovery, and costs minutes per run.

Tier 1 comes first because it is what the serving sweeps actually need, and because it is
independent of the harness question. Tier 2 tasks run in a **scratch git checkout**, and
passing means the repo's own tests pass Deterministic checks are what make this an instrument; an LLM judge would add
a second model's noise to every number this project rests on.

Metrics come from the cheapest honest source rather than from new code:

| Metric | Source |
|---|---|
| Task success | The repo's tests, in the scratch checkout |
| Tool-call validity | Harness logs plus the server's request log |
| Generation / prompt-processing tok/s | `llama-server` timings; `llama-bench` for isolated throughput |
| Peak memory, context high-water | Sampled during the run (0003 owns the envelope) |

Results append as one row per (config, harness, task, run) to a committed file, so sweeps
accumulate and nothing is recomputed to be compared. Runs repeat **3×** — sampling makes a
single run unfalsifiable — and the scorer reports spread, not just the mean.

Six to ten tasks, fixed and small. A suite too slow to re-run stops being re-run, and every
sweep multiplies it by the config count.

## Tasks

**Tier 1 — direct to the endpoint, no harness involved:**

- [x] A Go binary sends one fixed task straight to the endpoint, applies a deterministic check, and exits non-zero on failure
- [x] Six to ten tier-1 tasks exist as committed fixtures: tool-call correctness against an expected call, patch tasks checked by compiling and running the result, and retrieval probes at increasing context depth
- [x] Metrics are recorded per run — success, tool-call validity, tok/s generated, tok/s prompt, cached-token share — and appended to a results file in a stable schema
- [x] `make eval LABEL=<label>` runs the suite N× against a named config and prints a per-metric summary with spread
- [x] The request-level toggle matrix runs end to end: thinking on and off, each at its own model-card sampling defaults, reported per profile

**Tier 2 — through a real harness, for multi-turn behaviour tier 1 cannot see:**

- [x] Each candidate harness is confirmed to run non-interactively against the local endpoint, with the exact provider configuration recorded — or the blocker is written up
- [ ] An adapter interface drives at least two harnesses through the same tier-2 task in a scratch checkout, pass checked by the repo's own tests

## Open questions

- Does every candidate expose a scriptable non-interactive mode? This is the assumption
  the whole design rests on, hence task one. If one cannot be driven headlessly it leaves
  0010 by default, and that should be recorded as the reason rather than presented as a
  quality result.
- Is a fixed task suite representative of real agentic work? Leaning: **no, not fully** —
  which is why 0008 exists and why divergence between live use and this suite is a finding
  the suite has to answer for.

## Log

- 2026-08-17 — stack settled as Go, per `docs/INBOX.md`; question answered and cleared.
- 2026-08-17 — rescoped: the harness is chosen (Pi/Hermes), not written. Only the scorer
  is built here, which is a fraction of the original scope.
- 2026-08-17 — tier-1 scorer runs. Verified against the live baseline both ways and,
  more importantly, against two negative controls: expecting the wrong tool and an
  unsatisfiable argument each fail with a distinct outcome and exit 1. Outcomes are
  classified rather than boolean (no call / wrong tool / invalid JSON / wrong args /
  server error) because 0005 needs to know *how* a config fails, and because a server
  error must never be scored as model quality. Checker failure paths are covered by an
  offline test so the instrument does not depend on a server behaving.
- 2026-08-17 — seven tier-1 fixtures land and all run end to end. Both patch fixtures were
  confirmed to fail their own unseen tests before any model saw them; a fixture that passes
  out of the box would score every config correct and mean nothing.
- 2026-08-17 — the first suite run caught a defect in the instrument rather than the model.
  toolcall-edit-file failed with the model calling read_file when told to make an edit — but
  reading before editing is defensible agent behaviour, so the fixture was scoring a style
  preference as a wrong answer, and would have failed every config identically. Rewritten to
  include the file content inline, making a read provably unnecessary; the model then calls
  edit_file correctly. The general rule this teaches: a tier-1 task must have exactly one
  defensible action, or it is measuring the fixture author, not the model.
- 2026-08-17 — retrieval passes at 2k, 8k and 16k depth on the q8_0 KV baseline, so the
  quantised cache is not visibly costing recall at the context this config serves. That is a
  single sentinel per depth, not a rate — it establishes the probe works, and 0004 is what
  turns it into evidence about KV types.
- 2026-08-17 — fixture Go files renamed to .go.txt. Left as .go they sit inside this
  module, so `go test ./...` compiled and ran the deliberately-broken fixtures and the
  repo gate was red for exactly the reason the fixtures are correct. The scratch module
  writes them back under real .go names.
- 2026-08-17 — results are JSONL, one row per (config, task, repeat). Each row records
  what the server reported serving — n_ctx and model file — not only the config label a
  human typed. When the sweep starts varying server-level flags, a label is a claim, and
  a row carrying the served n_ctx is what stops one config's numbers being attributed to
  another after a restart that did not take.
- 2026-08-17 — suite mode, `make eval CONFIG=… N=…`, and a reporter grouping by config
  and thinking mode. Spread is min-max rather than a standard deviation: three passes is
  already a multi-hour job here, and a deviation over three samples claims precision the
  data does not have. Runs are sequential because the server has one slot — concurrent
  requests would queue and every timing would measure the queue. A transport failure
  records and continues rather than aborting, since losing hours of sweep to one dropped
  connection is worse than a gap in the data.
- 2026-08-17 — merging 0001 collided on the Makefile in two ways that mattered more than
  the textual conflict. Both branches defined `CONFIG`: 0001 as the path to the env file
  scripts/serve.sh consumes, documented in docs/TECH.md, and 0002 as the label written
  into every results row. Either silently taking the other's value would have mislabelled
  results or launched the wrong server, so the eval knob is renamed `LABEL` and 0001's
  shipped meaning stands. Both also defined `check`, one as unit tests and one as the
  live smoke; `check` is the ship gate in AGENTS.md, so it now runs both, with `test`
  and `smoke` available separately.
- 2026-08-17 — audited the Go against `kit help go-checklist`, which I had not read before
  writing it. Real defects, not style: runPatch used a hand-rolled timer that killed the
  process but left its reader goroutine blocked and ignored the caller's context, now
  `exec.CommandContext`; both commands did their work in `main` with scattered
  `os.Exit`, bypassing deferred cleanup, now `run() error` with one exit point and
  documented codes; `time.Now()` was uninjected, now a package-level `Now` a test can
  set; `go.mod` had no toolchain pin. `make check` gated on tests alone — it now runs
  fmt, vet and `-race`, and the live smoke moved to `make smoke` because
  `kit init --stack go` wired CI to `make check`, and CI has no server or weights.
  Tests moved to the public boundary (Client.Run against an httptest double), which also
  reaches failure paths a real model cannot be asked to produce on demand.
- 2026-08-17 — golangci-lint found 12 issues the earlier gate missed, because `make check`
  did not run it. Two were substantive: `AppendRow` deferred-and-dropped `Close` on a
  *write* path, so a row that never reached disk would have silently shortened a
  multi-hour sweep — Close is now checked and returned; and the haystack determinism test
  compared one expression to itself, which proves almost nothing, so it now builds from
  two independently constructed equal inputs and additionally asserts the sentinel appears
  exactly once and that a deeper haystack is larger. The rest were unchecked writes to
  stdout, now explicitly discarded at the call site. `lint` is wired into `make check`
  so this cannot regress silently, `.golangci.yml` is pinned, and the CI action is pinned
  to the same version rather than `latest`, which would fail a push for a lint that did
  not exist when the code was written.
- 2026-08-17 — the fixtures were starving thinking mode, and the aborted matrix proves it:
  **8 of 8 failures under thinking=on hit their token cap exactly**, and none was a quality
  failure. Retrieval allowed 64 tokens, which thinking spends on reasoning before it can
  answer. Left unfixed the matrix would have reported thinking on at 13/21 against thinking
  off at 21/21 and concluded thinking hurts quality, when the entire gap was a budget set
  while testing with thinking off. Caps raised to 512/1024/2048 — the same for both modes,
  so a mode spending more of it is a measured cost rather than a disqualification — and
  `fail_truncated_at_cap` now separates a capped answer from a wrong one, checked before
  any per-kind check.
- 2026-08-17 — **first clean matrix. 42 runs, zero truncations, every task 3/3 in both
  modes.** Thinking costs 3.06x the completion tokens (1089 to 3333) and 1.67x the wall
  time (384 s to 641 s), and buys nothing this suite can detect. The wall ratio is smaller
  than the token ratio because retrieval tasks are ingest-dominated, so extra generation is
  diluted by prompt processing.

  | task | off: pass / tok / s | on: pass / tok / s |
  |---|---|---|
  | patch-nil-check | 3/3 · 137 · 15.0 | 3/3 · 221 · 23.9 |
  | patch-off-by-one | 3/3 · 100 · 10.7 | 3/3 · 438 · 46.6 |
  | retrieval-2000 | 3/3 · 12 · 7.7 | 3/3 · 73 · 14.8 |
  | retrieval-8000 | 3/3 · 12 · 26.9 | 3/3 · 93 · 37.4 |
  | retrieval-16000 | 3/3 · 13 · 55.4 | 3/3 · 70 · 64.4 |
  | toolcall-read-file | 3/3 · 28 · 4.4 | 3/3 · 64 · 8.6 |
  | toolcall-edit-file | 3/3 · 61 · 8.0 | 3/3 · 152 · 18.0 |

- 2026-08-17 — **the result's real limit is a ceiling effect, and it must not be reported
  as a quality verdict.** Both modes scored 21/21, so the suite did not discriminate: it
  showed both configs are adequate for these tasks and said nothing about which is better
  where they are not. For the attended profile that is still decisive — thinking is 1.67x
  slower at no measured gain, so off wins on cost alone. For unattended it is no evidence
  at all: thinking's hypothesised benefit is long-horizon multi-step reasoning, and this
  suite contains none. Concluding "thinking does not help" from a suite where nothing fails
  would be the same error as the token-cap one, reached from the opposite direction.
- 2026-08-17 — the discriminating-tasks box is removed from here: 0013 was drafted for
  exactly that work after this box was written, and a box one feature owns should not sit
  in another's list. 0002 keeps the instrument; 0013 makes it able to rank.
- 2026-08-17 — harness configuration solved for two of three, and each needed its own
  mechanism: Pi an extension registering a provider, OpenCode a `provider` block using
  `@ai-sdk/openai-compatible`. Both are repo-local, so a run is reproducible from a
  checkout. Both were verified by completing patch-nil-check in a scratch module with the
  unseen test passing — Pi in 38.5 s, OpenCode in 3 m 06 s, the gap being OpenCode
  spending turns on `go build` and `go vet` where Pi went straight to the edit.
  Hermes is blocked and written up in `harness/hermes/README.md`: it resolves the
  provider — proven by control, since an unknown provider name fails differently — and
  then cannot reach loopback, while curl and both other harnesses reach the same server
  from the same shell. Egress firewall, proxy env, the /v1 suffix and api vs api_mode are
  all ruled out; a network-isolated sandbox is the leading hypothesis. It is excluded from
  0010 on transport, not on quality, and that distinction has to survive into the write-up.
- 2026-08-17 — Hermes works; the blocker write-up is replaced by a configuration. It was
  never a transport fault. The `providers.<name>` map I reverse-engineered from the
  source is real but is not how a local endpoint is configured, and taking that path
  produced `Connection error` — which an instrumented listener disproved, since Hermes
  never opened a connection at all. The documented shape is a top-level `model:` block
  with `provider: custom`, and it worked immediately.
  **Hermes refuses any context window under 64,000 tokens**, checked before any request,
  so the 32k baseline could never have satisfied it. That is a real constraint on 0010:
  Pi and OpenCode run at 32k and Hermes cannot, so a like-for-like comparison must put all
  three at 64k — where 0003 measured a cold ingest at 13.1 minutes against 5.4 at 32k.
  The harness comparison therefore inherits a context cost that is Hermes' requirement.
  Verified on patch-nil-check with the unseen test passing: Pi 38.5 s at 32k, OpenCode
  3 m 06 s at 32k, Hermes 4 m 45 s at 64k. Hermes also took 3 m 13 s to answer a trivial
  prompt, pointing at a large fixed system prompt ingested every turn.
