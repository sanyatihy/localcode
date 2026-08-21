---
id: 0025
title: Fail the gate on a mis-parsed machine reading
status: Draft        # Draft | Shipped | Dropped — kit ship and kit drop write it
created: 2026-08-21
shipped:             # written by kit ship, never by hand
needs:               # feature ids that must ship first, e.g. 0002, 0003. Empty = can start now
---

## Problem

Two of `docs/TECH.md`'s gotchas are mis-parsed machine readings that each cost a wrong
number, and neither could be caught by a test today. `internal/eval/mem.go` parses `vm_stat`
and `vm.swapusage` inline, and its only test calls the real machine and skips whenever the
platform does not answer — which is every CI run, since CI is Linux; on macOS it asserts
only that three numbers are positive, which a shifted field or a mis-scaled page satisfies.
`scripts/memprobe.sh` parses the same two outputs by field position, the approach `mem.go`'s
own comments call the one that "records a plausible number that is not the one the verdict
rests on", and it is the probe that already produced the page-size erratum.

## Non-goals

- **Not making CI run on macOS.** Runner cost is a separate question, and it is not needed:
  a parser fed captured output is platform-independent, which is the point.
- **Not a new memory metric.** Wired, anonymous, free and swap are what the verdicts rest
  on and they stay as they are.
- **Not moving the shell probe into Go.** `memprobe.sh` is called from shell sweeps and
  stays shell; what changes is that it reads the same way.
- **Not raising coverage as a number.** 0021 already settled that, and the target here is
  two parsers and one gate.

## Design

**The parser is a pure function, as the thermal probe already is.** `thermal.go` splits
`sampleThermal()` from `parseThermal(string)` and `thermal_test.go` table-tests the second;
`mem.go` does the same job inline and cannot be tested that way. Splitting it costs nothing
and is the difference between a test that can fail and one that skips.

**The tests are captured output, including the failures that happened.** A `vm_stat` block
at a 16 KB page, one at 4 KB, one with a counter absent, and a `vm.swapusage` line where the
wanted number is not the first of the three. Those are the two errata this repo has already
paid for, and a fixture is the only form in which they stay caught.

**The two probes read the same way.** `memprobe.sh` keys every counter off its label instead
of its column, matching `mem.go`. Field positions differ by macOS version, so the shell probe
is correct today by coincidence rather than by construction.

**Python enters the gate.** `scripts/deskverdict.py` holds the desktop rule "in one place
because two instruments now read it", and a syntax error there fails inside `ladder.sh` or
`screen.sh` partway through a sweep. `python3 -m py_compile` over the tracked Python is one
line and finishes the job 0021 started on the shell.

## Tasks

- [ ] `vm_stat` and `vm.swapusage` parsing is a pure function with a table test that runs on every platform, covering a 16 KB page and a shifted field
- [ ] `scripts/memprobe.sh` keys every counter off its label, so the two probes cannot disagree about the same output
- [ ] `make check` fails on a tracked Python file that does not compile

## Open questions

None.

## Log
