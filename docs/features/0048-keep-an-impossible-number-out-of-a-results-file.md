---
id: 0048
title: Keep an impossible number out of a results file
status: Draft
created: 2026-08-28
shipped:
needs:
---

## Problem

`internal/prefix` derives the cache figure rather than reading it — `prompt = n_tokens +
Offset - generated` — and nothing checks that the answer is possible. A log where those
lines do not pair produces `prompt_tokens: -6, cached_tokens: -6`, which `cmd/prefixlog`
sums into its totals and appends to a results file as a measurement. Found by a fuzz target
in 0.46 seconds; the repo has eight hand-rolled parsers and no fuzzing.

## Non-goals

- **Not validating the log's shape.** A line the server prints in a form this does not know
  is already skipped rather than parsed into a zero, which is right. What is missing is a
  check on the arithmetic's own answer.
- **Not fuzzing everything.** Two of the three parsers tried came back clean — 16 million
  executions against the handoff parser found nothing. Targets go where a value is derived
  or an invariant exists to state, and nowhere for the sake of coverage.

## Design

**A derived quantity is a hypothesis, and this one is published as a measurement.** VISION
says a number derived rather than observed is labelled as one until a run confirms it.
`Offset` exists precisely because the derivation is known to be off by one against the
endpoint's own account, and `Check` exists to hold it to a run of the probe. Neither fires
unless someone passes `-check`, so the ordinary path emits whatever the arithmetic returns.

**A request that cannot have happened is dropped, not clamped.** Clamping to zero would put
a row in the file saying a request ingested nothing, which is the finding the tool exists to
report — the reading has to be absent rather than wrong. The count of what was dropped is
printed, because a log this cannot account for is a fact about the log and silently reading
fewer requests than it holds is how an instrument lies quietly.

**Fuzz targets are regression tests that write themselves.** `go test` runs a target's seed
corpus and everything under `testdata/fuzz` on every ordinary run, so a case the fuzzer
finds becomes a permanent test at no cost to the gate — the corpus entry from this finding
is committed with the fix. Targets go on the parsers that turn somebody else's bytes into a
number: the server log, the transcript readers, `parseMetrics` and `parseEnvFile`.

**What fuzzing here will not do is find a wrong answer.** The `Done` defect 0043 fixed —
`No, the tests still fail` reading as finished — is not expressible as a property, because
both the input and the output are well-formed and only a reader knows which is meant. Fuzzing
buys robustness and invariants; the semantic bugs still need a person.

## Tasks

- [x] A request whose derived account is impossible is dropped and counted, and the count is
      reported rather than left in the totals
- [x] The parsers that turn somebody else's bytes into a number have fuzz targets stating
      what must hold, and the case behind this finding is in the committed corpus

## Log
