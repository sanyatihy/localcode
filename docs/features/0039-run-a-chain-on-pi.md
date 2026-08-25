---
id: 0039
title: Run a chain on Pi
status: Draft
created: 2026-08-25
shipped:
needs: 0038
---

## Problem

`localcode` starts one harness. The launch is `cmd/localcode/chain.go` assembling Claude
Code's flags, its settings file, its transcript path and its event stream, so a chain is not
something this project can run on anything else — and the harness question VISION leaves open
cannot be answered by a project that can only run the incumbent.

## Non-goals

- No third harness. Hermes and OpenCode already have scorer adapters and neither is being
  driven here — but what they need is known, and the interface is judged against it.
- No change to what a session may spend. `internal/chain` decides that and 0038 taught Pi to
  obey it; this is about starting one and reading it back.
- No comparison. Which harness a chain should use is settled by a run that needs both of
  them working, so it follows this rather than riding on it.

## Design

**The interface is extracted by the second implementation, and judged against the fourth.**
Two is what justifies writing one at all; but `internal/harness` already carries adapters for
four agents, so what varies across them is measured rather than guessed, and an interface
that only just fits Claude Code and Pi is one that will be rewritten by whoever adds the
third. The test is not that it compiles with two — it is that adding a third touches only
the new adapter.

**So the interface is the driver's needs, not Claude Code's flags renamed.** Six things, each
of which at least two of the four candidates answer differently:

| what the driver needs | Claude Code | Pi |
|---|---|---|
| start one non-interactive session on an instruction | `-p`, flags | `--print`, flags |
| restrict the tool set | `--tools` | `--tools`, and names differ in case |
| append to the system prompt | `--append-system-prompt` | `--append-system-prompt` |
| carry the inherited handoff in | `SessionStart` hook | `session_start`, or the same flag |
| refuse a tool call at the ceiling | `PreToolUse` hook, subprocess | `tool_call` event, in-process |
| account for what the session cost | transcript under `CLAUDE_CONFIG_DIR` | session file under `--session-dir` |

Hermes is configured by a YAML file and OpenCode by JSON, and neither has anything like a
pre-tool-call veto — so the interface must let an adapter say **what it cannot do**, and the
driver must degrade rather than fail. A harness with no veto is one whose sessions are bounded
by the call budget and the clock alone, which is worse and is still a chain.

**The property that makes a third harness additive is checkable.** No package outside the
adapters may name a harness: no `claude` in `cmd/localcode`, no transcript path, no flag
string, no event shape. That is a grep, so it is a box rather than an aspiration.

**Pi's session file is under `--session-dir`, which the driver already owns.** Claude Code
files its transcript under `CLAUDE_CONFIG_DIR` keyed by working directory, which is why
`newestTranscript` has to search for it. Pi is told where to write, so the driver knows the
path before the session starts and `localcode account` reads it without globbing.

**The accounting is the same shape.** Pi's session JSONL carries per-message `usage`, so
peak context, preamble and the fitted rates come out the way `chain.Cost` and
`handoff.Requests` produce them for Claude Code — a second reader, not a second model of what
a session cost.

**The supervisor keeps owning every line printed.** Pi's `--mode json` is the equivalent of
`--output-format stream-json`; `render` learns its event shapes rather than the driver
handing the terminal over, because a chain at 8 tok/s that prints nothing is
indistinguishable from a wedged one.

**One flag chooses.** `-harness pi` against a default of `claude-code`, so a comparison can
run the same instruction twice with one token changed and the arms differ by that and nothing
else.

## Tasks

- [x] what the driver needs of a harness is an interface, with Claude Code behind it and
      nothing about a chain changed
- [x] nothing outside the adapters names a harness — no flag, no path, no event shape —
      and a check holds it that way
- [ ] `-harness pi` starts a session against the local endpoint, in the sandbox, with the
      tool set and the appended briefing the Claude Code arm gets
- [ ] a Pi session inherits the handoff the session before it wrote
- [ ] the supervisor prints a Pi session's work as it happens, not after it
- [ ] `localcode account` reports a Pi chain: peak, preamble, ingest, generation, clock
- [ ] a Pi chain resumes, and refuses a resume on a different budget the way 0033 requires

## Open questions

- **What an adapter does when the harness cannot veto a tool call.** Hermes and OpenCode
  have no equivalent of `PreToolUse`, so a chain on either is bounded by the call budget and
  the wall clock and not by its context. Leaning towards the adapter declaring it and the
  driver saying so at launch, rather than refusing to run — a bound nobody can enforce is
  still worth recording against.

- **Whether the sandbox profile fits Pi unchanged.** It is written for what Claude Code
  touches; Pi reads `~/.pi` for settings and packages, and a profile that denies it may break
  a session in a way that looks like a model failure. Leaning towards finding out with one
  session rather than reasoning about the profile.
- **What a Pi session costs before the first word.** Claude Code spends 3,508 tokens of
  preamble for four tools, and 0008 measured Pi at a fifth the ingest per task. If the
  preamble is much smaller, the ceiling buys proportionally more work and 0040's comparison
  is partly a preamble comparison — worth measuring in this feature so that one is not.

## Log
