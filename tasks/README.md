# The ranking suite

Nine fixtures, about 95 seconds a pass. This is what a serving sweep is scored on:

    go run ./cmd/eval -tasks tasks -n 3 -config <label> -results results/tier1.jsonl

[`depth/`](depth/) is a separate suite and is deliberately not picked up by that command —
it is a floor check against damage at depth, costs most of the clock, and ranks nothing.

## What is here

| fixture | kind | asks for |
|---|---|---|
| `patch-nil-check` | patch | a guard on a nil argument, leaving other behaviour alone |
| `patch-off-by-one` | patch | a boundary condition, without over-correcting the other way |
| `patch-sibling-merge` | patch | a merge that does not alias the caller's map |
| `patch-sibling-splitpath` | patch | a split that drops empty segments, not just the outer slashes |
| `patch-contradiction-rounding` | patch | a rule, where a doc comment and a passing test disagree |
| `toolcall-read-file` | toolcall | read the named file rather than editing it blind |
| `toolcall-edit-file` | toolcall | edit, when the file is already in the prompt and there is nothing to read |
| `toolcall-constraint-unknown-path` | toolcall | search, when no path was given and guessing one is wrong |
| `toolcall-constraint-readonly` | toolcall | propose rather than write, under a stated read-only constraint |

## How a fixture earns its place

- **Exactly one defensible action.** A task that scores a style preference — reading a
  file before editing it — fails every config identically and ranks nothing.
- **Proved to discriminate before any model time is spent on it.**
  `TestPatchFixturesDiscriminate` runs a correct answer and the tempting wrong one through
  the real patch runner and requires the first to pass and the second to fail.
- **Proved against answers that are merely different.** The same test asserts that
  equally-defensible answers pass — returning `nil` rather than an empty slice, say. A
  fixture whose unseen test rejects a correct fix costs a config marks for being right,
  which is the more expensive failure.
- **Bounded.** Every fixture carries `timeout_seconds`, and over budget is scored as
  `fail_over_budget` rather than as a wrong answer. A single task once spent 15 minutes
  reasoning and produced nothing.

Two of the nine actually discriminate at present:
`patch-contradiction-rounding` and `toolcall-constraint-readonly`. The rest are 3/3 at
every setting ever measured — they are the floor check that catches a config broken
outright. See [which tier-1 tasks carry
signal](../docs/TECH.md#which-tier-1-tasks-carry-signal).

## How a fixture is written

A patch fixture is a directory: `task.json`, the `broken.go.txt` the model is shown, and
the `verify_test.go.txt` it never sees. A tool-call fixture is a single `.json`.

**Fixture Go files carry a `.txt` suffix.** Named `.go` they sit inside this module, so
`go test ./...` would compile the deliberately-broken sources and the gate would be
permanently red for the wrong reason.

A `tier2` block adds the one thing an agent loop needs and a single request does not: the
name the broken file takes in a scratch checkout. What the bug *is* stays in `messages`,
so a fixture cannot pose one problem to a harness and a different one to the model.
