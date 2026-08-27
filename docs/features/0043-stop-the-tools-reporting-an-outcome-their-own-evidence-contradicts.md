---
id: 0043
title: Stop the tools reporting an outcome their own evidence contradicts
status: Draft
created: 2026-08-27
shipped:
needs:
---

## Problem

A chain can report success on work that failed. `Done` reads `**Next:** No, the tests
still fail` as finished, and an event line above the renderer's buffer ends the render
silently while leaving `cmd.Wait` blocked, so a truncated stream is recorded as a session
that ran past its clock. A reading of every Go file found both, and eleven smaller
defects of one shape: a bound that is not checked when it is hit.

## Non-goals

- **No refactor.** `eval.AppendJSON` sits in the scoring package and the session
  supervisor imports it for that alone; the `Agent` interface sits beside its
  implementations rather than in `cmd/localcode`, which consumes it. Both are real and
  neither changes behaviour, so both go to `BACKLOG.md`.
- **No general sandbox for the grader.** `go test` on a model's answer is denied the
  network and nothing else. Confining its writes is a different question from the one
  `harness/offline.sb` answers, and the grader already runs in a temp directory it
  removes.
- **No new completion signal.** `Done` keeps reading prose. Ending a chain on commits
  instead is a feature, and this one is a repair.
- **Nothing about the model.** Every defect here is in the driver, so no number this
  repo has published moves. The one exception is named in the design.

## Design

**Shipped as one round because it is an audit.** Six of these are three lines each and
none is separately reviewable: split, the reviewer reads the same audit six times to
judge six one-line diffs. The boxes below are the review units.

**A bare `no` stops being a completion word.** `Done` cuts the step at the first
`.,;:—–` and tests what is left, so `No, the tests still fail`, `No. The build is
broken.` and `No — still investigating` all reduce to `no` and read as finished. The cut
is load-bearing and stays: it is what makes `none — 0003 is done. Remaining: …` read as
finished, which was measured. What goes is `no` as a whole clause. `none`, `nothing` and
`done` keep it — they are the contract word the briefing asks for and cannot open a
negation — and `no` still counts with a qualifier after it, which is where `no further
work` lives. A step whose head clause is the single word `no` is then unfinished, which
is the conservative reading and the right default: a false finish reports success on
failure, and a false stall costs one session.

**Both renderers check the scanner and drain what they cannot read.** They raise the
buffer to 16 MB and never test `scan.Err()`, so past the cap the loop exits as if the
stream had ended — verified: a 17 MB line drops every event after it, the server's own
overrun refusal included. `launch.session` reads this off `cmd.StdoutPipe()` and then
calls `cmd.Wait()`, and a reader that stops leaves the child blocked in `write(2)` —
verified: `Wait` does not return. So the session hangs to `-session-timeout` and the row
says `timed_out`, which is the one diagnosis the evidence rules out. The error becomes a
rendered line and the rest of the pipe is copied to `io.Discard`, so the session ends on
its own exit code and the operator is told why the narration stopped.

**Whether a session may end without a handoff moves to the spec.** `Agent.Prepare` takes
`oneShot` so that an agent refusing to stop cannot refuse a conversation; `piAgent`
ignores the parameter, and its extension asks `localcode hook stop` on every `agent_end`.
An interactive `-harness pi` session is therefore told twice that it handed nothing on.
`chain.Spec` gains `OneShot`, written by the supervisor, which is the only thing that
knows; `stop` stands aside when it is false. Two adapters stop having to remember, and
the parameter `piAgent` ignores stops existing.

**The git snapshot gets a deadline of its own.** `chain.Repo` runs four git commands twice
a session, one of them per worktree, and `git` neither takes a context nor sets a timeout.
Every other blocking call here is bounded — the request, the grader, the health check, the
session. An index lock or a stalled filesystem hangs the chain outside the session clock,
so no bound in the system applies. The context is internal rather than a parameter: the
caller cannot usefully cancel this, and the function already reads every error as "no
repository here", so a deadline degrades the way the design already handles.

**The grader is denied the network.** `runPatch` and `RunTier2` compile and run code the
model wrote, bounded by a clock and nothing else, in a repo that refuses to run an agent
at all when `sandbox-exec` is missing. `GOPROXY=off` stops the build reaching the module
proxy for an import the model invented, and where `sandbox-exec` is present the test runs
under a generated profile that denies everything but loopback. Generated rather than
`harness/offline.sb`: that file is the scored offline condition and is addressed by path
from the repo, which the grader has no handle on. Absent — CI is Linux — `GOPROXY=off`
stands alone, and the grader is not the path the machine is served on.

**The Messages path counts the whole prompt.** `toResponse` decodes
`cache_creation_input_tokens` and adds it to nothing, so `prompt_tokens` is short by
whatever the server wrote to cache. The chat path counts the whole prompt and
`handoff.Requests` already sums both, and this is the path `cmd/prefixprobe` uses
exclusively because it is the one an editor session takes. No published number moves:
nothing under `docs/data/` carries a non-zero cache-creation count.

**One swap threshold, not two.** The summary allows 20 MB of slack because less would
flag every clean sweep; the paired section voids any growth at all. The same run reads
clean in one half of one report and void in the other. 20 MB wins, because it is the one
with a measurement behind it.

**The rest are the same defect in smaller places.** `-sessions 0` skips the loop and
records `{"reason":"bound","session":-3}`, where `cmd/handoff` already refuses a bound
below one. `handoff.Refusals` keeps the 64 KB scanner default and drops `scan.Err()`,
which the two readers beside it in the same file both fixed. `truncate`, `oneLine` and
`tail` cut UTF-8 by byte. `chain.Chains` sorts ids as strings, so the eleventh chain in a
second sorts before the second. `renderPiJSON`'s `Args` is the one field matched by
accident rather than by tag. `chain.Cost` answers `-1` and `piRecorder.Cost` answers `0`
against one interface that documents zero.

**The tests that were missing are the ones at the bounds.**
`TestProgressSurvivesAResultBiggerThanAScannerBuffer` asserts at 300 KB against a 16 MB
cap, so it passes while the cap is unguarded; the new one asserts past it. `Window`,
`declared` and `piDeclares` turn a served context into the number the whole gate is
derived from and have no test at all. `cmd/prefixprobe` is at 0.0%.

## Tasks

- [x] A chain's ending names what happened: no negation reads as finished, and a bound
      below one is refused rather than recorded as a chain that stopped at session -3
- [x] The event stream reports what it could not read instead of going quiet, and a
      session it cannot read to the end still exits on its own code rather than its clock
- [x] One place decides whether a session may end without a handoff, so an interactive
      session is never refused one
- [x] Nothing in a session's loop blocks without a bound, and the grader cannot reach the
      network to build or to run
- [x] Every number a row carries is the number its instrument reported, and one swap
      threshold decides void across the whole report
- [x] The remaining readers are correct at their bounds, and the three functions that
      size a session's budget have tests

## Log
- 2026-08-27 — `GOPROXY=off` gave `isBuildFailure` a message it did not know. An
  unresolvable import used to be resolved over the network and now fails against the
  module cache, which reports itself as `finding module for package` and was scored
  `fail_test_failed`. It is invalid Go rather than Go that is wrong, so the two markers
  join the list. Caused by this box, so fixed in it.
