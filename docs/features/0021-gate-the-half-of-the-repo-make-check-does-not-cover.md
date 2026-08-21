---
id: 0021
title: Gate the half of the repo make check does not cover
status: Draft        # Draft | Accepted | Shipped | Dropped | Superseded
created: 2026-08-21
shipped:             # fill the date when status flips to Shipped
check:               # optional — date to check whether this worked. Only for bets.
checked:             # written by kit check <id> "<outcome>", never by hand
review:              # optional — `human` means a person merges this one. kit accept --review
needs:
related: 0002, 0019, 0020
---

## Problem

`make check` is the gate, and it runs `gofmt`, `go vet`, `golangci-lint` and `go test`. Go is
**49.6%** of the repo. The other half — 1,184 lines of shell, 4,496 of markdown, 65 of Python
— is checked by nothing.

That matters more here than in most repos, because a shell bug does not crash: it produces a
wrong measurement. Two of `docs/TECH.md`'s gotchas are already shell bugs that cost wrong
numbers, and one of them made the gate itself report green while CI failed. Run for the first
time, `shellcheck` finds four warnings, one of them a live instance of exactly that class:
`runtimes/mlx/compare.sh` does an unchecked `cd` and is the only script without `set -e`, so a
failed `cd` runs the comparison from wherever it was invoked.

The Go half has a hole of its own. Every `cmd/` package is at **0%** coverage and the total is
51.6%. Each command documents its exit codes as "the contract, so a sweep can branch without
parsing output", and every one of those contracts is prose — including the refusal that exists
because a fixed-sampling toggle sweep voided 114 rows.

## Non-goals

- **Not stricter Go linting.** Measured: enabling `revive`, `gocritic`, `prealloc` and nine
  others yields 33 findings, 29 of which are revive demanding doc comments on exported types.
  Adding them would contradict this repo's own comment rule and spend the local tier's context
  on what the code already says. The three real ones are fixed by hand instead.
- **Not raising coverage as a number.** The target is the exit-code contract and the refusals,
  which are what other programs depend on. A percentage is not a goal.
- **No new dependency in the Go build.** `go.mod` has no third-party requires and no `go.sum`;
  that stays true.
- **Not a CI platform change.** CI runs on Linux and the product is macOS-only, so the
  platform probes degrade to zero there. Making CI run on macOS is a separate question about
  runner cost.

## Design

**The gate covers what the repo is made of.** `make check` gains `shell` and `docs` targets
beside `fmt`, `vet`, `lint` and `test`. Both follow `lint`'s existing shape: run the tool when
it is present, skip loudly when it is not, and never let a failure take the skip branch — the
mistake that made this gate green for two pushes CI rejected.

**`shellcheck` is configured, not argued with.** Three of its four findings are SC1090,
sourcing a path known only at runtime, which is the design of every config-driven script here.
A `.shellcheckrc` disables that one rule with the reason beside it; the rest are fixed.

**The docs check is a link check, and it is this repo's own.** 4,496 lines of markdown across
39 files cross-reference each other and `docs/TECH.md`'s anchors; nothing notices when one
rots. `kit audit` checks a doc's structure and not its links. The check is a short script with
no dependency, because a link checker that needs a network is a gate that fails offline.

**The command tests take the seam that already exists.** Every `main.go` splits `main()` from
`run(args, stdout, stderr) error`, which was written to be callable and never called. The
tests drive `run` with flags and assert the refusals and the error that maps to each exit
code — not the sweep behind them, which needs a server.

## Tasks

- [x] `make check` runs `shellcheck` over every tracked shell script, skipping loudly when it is absent, and the repo is clean under it
- [x] `runtimes/mlx/compare.sh` cannot run from the wrong directory, and every script sets the same shell options
- [ ] `make check` fails on a broken relative link or anchor in any tracked markdown file
- [ ] `cmd/eval` and `cmd/tier2` have tests for every refusal they document, including the sampling guard and the desk-profile ceiling
- [ ] `cmd/report`, `cmd/prefixlog` and `cmd/handoff` have tests for the flag errors their exit codes rest on
- [ ] The three real findings from the stricter-linter trial are fixed, including the test that panics instead of failing when a marker is absent
- [ ] CI carries a job timeout, so a hung gate fails rather than running until the runner is reclaimed

## Open questions

None.

## Log

- 2026-08-21 — the open question is settled as it leaned: `make check` skips `shellcheck`
  loudly, matching `lint`, and CI installs it so the gate is absolute where it has to be. One
  thing the leaning did not have — the skip is not total, since `bash -n` runs in its place
  and still catches a syntax error.
- 2026-08-21 — box 1 absorbed `runtimes/mlx/compare.sh`'s unchecked `cd` from box 2, because a
  gate added red is not added. What is left of box 2 is the shell-options consistency, which
  `shellcheck` does not flag.
- 2026-08-21 — the file mode turned out to be the rule box 2 needed. Every executable script
  must `set -euo pipefail`; `scripts/lib.sh` is the one non-executable, is sourced, and must
  set nothing, since options set there leak into the caller. `make check` holds both halves,
  so this is enforced rather than written down.
- 2026-08-21 — the gate runs at `-S style`, not the `warning` the design assumed. After the
  three findings were fixed the repo is clean at every severity, so there was no reason to
  configure the stricter tiers away. `.shellcheckrc` disables one rule: SC1090, sourcing a
  config path chosen at runtime, which is the design of every script here.
