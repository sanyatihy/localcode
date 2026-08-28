---
id: 0049
title: Say which build produced a measurement, and refuse a command that is not one
status: Draft
created: 2026-08-28
shipped:
needs: 0048
---

## Problem

A row records the model, the served context and the config label, and nothing about the
driver that wrote it — so 0043, which moved the budget arithmetic every session runs under,
left rows either side of it indistinguishable. No binary here answers `-version`. And the
subcommand switch falls through on anything it does not recognise, so `localcode hook` with
no hook name starts a server and asks the model to do "hook".

## Non-goals

- **Not a release process.** No tags, no changelog, no published artifacts. What is wanted
  is that a row names the commit it came from.
- **Not guessing what a mistyped subcommand meant.** `localcode statu` is refused, not
  corrected: an instruction is free text and a driver that autocorrects one word of it will
  eventually rewrite a real instruction.

## Design

**The provenance is already in the binary and nothing reads it.** `go build` stamps
`vcs.revision`, `vcs.time` and `vcs.modified` into the build info, with no `-ldflags` and no
Makefile change — verified: a clean tree reports `vcs.modified=false` and a tree with one
edited line reports `true`. That last field is the one that matters here. VISION says a
number nobody can regenerate is not evidence, and a row from a modified tree is exactly
that; it can now say so instead of being indistinguishable from one that is reproducible.

**`go run` stamps nothing, and the Makefile uses it.** `make eval` and `make report` run
`go run ./cmd/eval`, which produces no VCS settings at all — measured. So the row records
what it can and says the rest is unknown, rather than a build id that is silently absent.
Whether the Makefile should build first is a question the numbers answer: an unstamped row
is a row that cannot be attributed, and the sweeps are hours.

**Refusing an unknown subcommand is not the same as refusing an unknown instruction.** The
two are told apart by a list, because there is no other signal: `localcode fix the median
bug` and `localcode statu` differ only in that one word is a subcommand this binary nearly
has. A single word that is not a subcommand and is not a sentence is the case worth
refusing, and `hook` with no name is unambiguous — it names a subcommand and gives it
nothing.

**Where the stamp goes is where the row already is.** `eval.Row`, `prefix.Row` and
`prefix.LogRow` are the three shapes that reach a results file, and one field on each is
cheaper than a fourth record type nothing else reads.

## Tasks

- [ ] Every binary answers `-version` with its commit and whether the tree it was built
      from was modified
- [ ] Every row that reaches a results file names the build that produced it, or records
      that it could not be known
- [ ] A subcommand this binary does not have is refused with the list of the ones it does,
      and `hook` without a name is one of them

## Open questions

- **Whether `make eval` should build rather than `go run`.** Building costs a second and
  makes every row attributable; `go run` is what the target has always done and what the
  recorded numbers were taken with. Leaning: build, and say in the target why — the sweeps
  are hours and the second is not the cost being managed.

## Log
