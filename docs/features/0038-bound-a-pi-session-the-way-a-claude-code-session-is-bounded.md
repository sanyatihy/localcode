---
id: 0038
title: Bound a Pi session the way a Claude Code session is bounded
status: Draft
created: 2026-08-25
shipped:
needs:
---

## Problem

Every bound this project enforces is written against Claude Code's hook protocol, so the
harness question VISION names as open is decided by which harness the enforcement happens to
speak. Pi is the alternative with the strongest prior — a fifth the ingest per task on the
one profiling comparison taken, and a context window this repo declares in a file it already
owns — and nothing here can hold it to a ceiling.

## Non-goals

- No chain. A session that is bounded is not a chain that hands off, and 0039 is where the
  driver learns to run one.
- No Pi compaction. It is the mechanism handing off replaces, and it is slower and less
  controlled than a handoff: the reserve is configurable, so the threshold is put where it
  cannot fire and `session_before_compact` cancels what still does.
- No second gate. What a session may spend is `internal/chain`'s answer and stays there;
  this is an adapter to it, not a reimplementation.

## Design

**Pi's extension API has every primitive the hooks have, and three are stronger.** A
`tool_call` handler runs before the tool and returns `{block, reason, terminate}`, so a
denial is a return value rather than a subprocess and an exit code. `event.input` is mutable
in place, which is what `ClampRead` needs and what Claude Code makes it fake through a hook's
JSON reply. `terminate` ends the run when every result in the batch terminates, which the
`Stop` hook has no equivalent of. `session_before_compact` returns `{cancel: true}` and says
whether the trigger was `manual`, `threshold` or `overflow`.

**The extension decides nothing.** `chain.Gate` is the decision and it takes a `Payload` and
a `State`; the extension's job is to fill those in from Pi's vocabulary and to act on the
`Verdict`. Tool names differ — `read`, `edit`, `write`, `bash` against `Read`, `Edit`,
`Write`, `Bash` — and the mapping is the adapter's, not the gate's.

**The context is declared in a file this repo owns.** `harness/pi/local-provider.js` carries
`contextWindow` and `maxTokens` per model, so there is no environment variable whose
enforcement has to be measured and turned off — the whole of what 0037 spent four boxes on
does not arise. That file currently says 32,768 against a server that serves 49,152, which is
wrong today and would be wrong in a way that matters here.

**Compaction is disabled twice.** Pi auto-compacts when `contextTokens > contextWindow -
reserveTokens`, with `reserveTokens` defaulting to 16,384 and configurable in settings. Put
the reserve where the threshold cannot fire below the ceiling, and cancel in
`session_before_compact` anyway: a threshold is a number that can drift and a cancel is a
refusal.

## Tasks

- [x] the provider file declares what the server actually serves
- [x] a Pi session is denied a tool call at the ceiling, by `chain.Gate` and not by a second
      answer to the same question
- [x] a `Read` past the result cap is narrowed rather than refused, as it is under the hooks
- [x] a Pi session cannot compact, by a reserve that cannot fire and a cancel that refuses
- [x] a Pi session cannot end without a handoff, within the same grace the `Stop` hook allows

## Log

- **Both open questions settled in the building.** The peak context is passed in by the
  extension, from `ctx.getContextUsage()` — Pi's own reading, which sums the four usage
  fields `internal/handoff` takes a Claude Code peak from. So no second reader of Pi's
  session JSONL is written here, and 0039's supervisor inherits none. The extension reaches
  the gate by running `localcode hook gate`, one process per call, as the hooks already do.
- **`chain.Payload` gained `peak_tokens`.** A harness that keeps its own context reports it
  there, and the transcript is read only when it is absent.
