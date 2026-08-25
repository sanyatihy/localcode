# Backlog

Ideas that are not features yet. Nothing here is scheduled or prioritised — the
default state is **stays frozen**.

- **Start Pi with `PI_OFFLINE=1` in a chain** — pi runs startup network operations before
  its first request. Twice while 0038 was measured a session took over a minute to reach
  that request and once over five, with nothing sent to the endpoint; every run since with
  `PI_OFFLINE=1` started in seconds. Promote when: a chain on Pi measures per-session
  startup, or a run stalls again.
