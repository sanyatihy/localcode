---
id: 0032
title: Let a session's call budget follow the window it was given
status: Draft
created: 2026-08-24
shipped:
needs:
---

## Problem

A session may spend 30 tool calls whatever window it was given, and the number fits neither
end of the range this repository serves. At a 10,240-token ceiling, sessions died on context
having spent 6 to 11 calls, so the budget never bound. At 22,528, eight of the last ten
sessions ended on the call budget with 15,000 tokens of context unused — each one handing off
a question it had the room to answer.

## Non-goals

- No removal of `-calls`. An explicit number is what makes a measurement repeatable, and
  the flag stays as the override.
- No per-tool weighting. What a call costs varies by tool, which is what the result cap
  already bounds; a budget that modelled it would be guessing at the harness's formatting.

## Design

The default is derived from the window the same way every other bound here is, rather than
being a constant beside them. A session's working room is its ceiling less the preamble it
starts with, and a call costs what a call costs — so the budget is that room divided by the
measured cost of a call, and `NewLimits` is where it belongs.

The cost of a call is the number to establish before the formula is written; the chain
accounted in 0027 has it per session and this repository has several chains to read it from.

A floor stays, because a window too small for a useful number of calls is a configuration
mistake rather than a session to run — the same argument `NewLimits` already makes for
refusing a window smaller than the preamble.

## Tasks

- [ ] the cost of a tool call is measured from the chains this repository has already run
- [ ] a session's call budget is derived from its window, with `-calls` still overriding it

## Log
