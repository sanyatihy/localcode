# Backlog

Ideas that are not features yet. Nothing here is scheduled or prioritised — the
default state is **stays frozen**.

- **Flip the chain's default harness to Pi** — 0040 measured Pi cheaper than the incumbent
  on every per-session column across four chains, and TECH says to drive a chain with
  `-harness pi`, but `harness.DefaultAgent` is still `claude-code`. Promote when: more
  chains agree, or a reading exists of what the default costs one-shot patch work, which
  0010 says is the job the incumbent wins.

- **Move `AppendJSON` out of the scoring package** — `cmd/localcode` and `cmd/handoff`
  import `internal/eval` for a durable JSONL append and nothing else, so the session
  supervisor depends on the scorer to write a line. Promote when: a third caller needs it,
  or something else forces the import graph open. Behaviour-neutral either way.

- **Declare `Agent` in the package that consumes it** — the interface sits in
  `internal/harness` beside its implementations; its only consumer is `cmd/localcode`.
  `Driver` gets this right, declared in `internal/eval`. The registry behind `NewAgent` is
  a real reason to keep the constructor where it is, and none to keep the interface there.
  Promote when: a second consumer appears, or the registry moves anyway.
