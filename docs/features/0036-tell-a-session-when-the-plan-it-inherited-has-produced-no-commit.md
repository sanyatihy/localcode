---
id: 0036
title: Tell a session when the plan it inherited has produced no commit
status: Draft
created: 2026-08-24
shipped:
needs:
---

## Problem

A session sees the handoff it inherited and nothing else, so a plan that has already failed
twice arrives reading exactly like one that has not. Measured on a live chain: a session
concluded the sandbox was corrupting its regexes, and the four sessions after it inherited
that as a premise and spent 2h44m without a commit, each restating it more confidently.
Only the supervisor can see that pattern, and it says nothing.

## Non-goals

- No judgement of whether the work was good. This reports that the repository gained no
  commit, which is a count; whether the sessions were worth their time is a review's
  question and 0030 already refuses it.
- No change to what stops a chain. 0035's three still sessions still end it, and this fires
  earlier and on a different signal, so a chain is told before it is stopped.
- No second channel. What a session is told arrives in the appended system prompt beside
  the handoff briefing, because a hook that told the model what to do was refused as
  injection — correctly, since it came through a tool result.

## Design

The signal is commits, not movement. 0030 counts the object database and every worktree's
status together, which is right for deciding whether a session did anything at all — but a
session writing throwaway probes moves a worktree, and two such sessions read as working.
The object database growing means a commit landed somewhere, which is the durable unit and
the one a stalled chain stops producing. On the chain this feature comes from, commits stop
five sessions before movement does.

Two sessions without one, then the next is told. Two rather than three, so the chain is
told before 0035's counter can stop it — a warning that arrives with the ending is not one.

What is said is a fact the session cannot otherwise have, not a request for restraint.
Instruction was measured not to work on this model where it asked a session to want less:
one told in prose to spend three commands reached compaction anyway. This is the other
kind — the supervisor is the only thing that can see across sessions, and what it reports
is the count and the inference the count licenses.

When it fired is recorded beside the chain's ending, because a nudge nobody can read back
cannot be judged against the session that followed it.

## Open questions

- Whether it works at all. This is prose against a model that has ignored prose before, and
  the generated fixture cannot reproduce a chain talking itself into a false premise, so
  there is no clean A/B. The record in `chain.json` is what a later reading would rest on.

## Tasks

- [x] a session's row says whether the repository gained a commit while it ran
- [x] a session inheriting a plan from two sessions that committed nothing is told so, and
      the chain records that it was
- 2026-08-24 — the count is seeded from the rows rather than from this run's own sessions.
  A resumed chain inherits the plan that was not landing, and counting only the current
  invocation is what let three sessions of a real chain run free after a resume.
