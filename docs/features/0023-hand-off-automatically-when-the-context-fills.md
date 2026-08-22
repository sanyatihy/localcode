---
id: 0023
title: Hand off automatically when the context fills
status: Draft        # Draft | Shipped | Dropped — kit ship and kit drop write it
created: 2026-08-21
shipped:             # written by kit ship, never by hand
needs:
---

## Problem

A `localcode` session that fills its context dies rather than handing off, and the handoff it
leaves is worthless. Measured on one real session in a work repository: **20 compaction
refusals over 17 minutes**, ending at `Prompt is too long` with the work abandoned.

Reproduced and taken apart since, at a 12,288-token wall. **Every mechanism that reacts to a
full context loses the work.** A chain of eight sessions produced eight handoffs of which
seven carried `Prompt is too long` as their `Next` step, because `session-end.sh` extracts
the last thing a session said and a dying session says an error; the chain spent 49,671
tokens and never began the task. Claude Code's own compaction fares no better here — its
summary recorded that *"the original task from before compaction is not fully preserved"*.
A hook that warns the model at 45% of the window was seen, acknowledged and ignored.

**The context has to be kept away from the wall instead, and neither property may rest on
the model agreeing.** A session told to spend at most three commands reached compaction
anyway.

## Non-goals

- **Not a change to what a handoff contains.** 0016's three hooks and their format stand;
  this decides when a session ends and what starts the next one.
- **Not compaction.** Measured against this session: resuming from a handoff cost **4,265
  tokens and 54 s**, where compacting would have re-read 37,837 tokens — 452 s of ingest
  before generating a summary. Allowing compaction would trade a dead end for a recurring
  twelve-minute pause.
- **Not a bound on what fills the context.** 74.6% of that session was tool results and
  another 23.6% tool calls, so the fastest way to spend a window is unbounded command output.
  Capping it is a different decision and a `BACKLOG.md` line.
- **Not a driver for unattended work.** `cmd/handoff` already runs a task box across sessions
  on a budget. This is the interactive case, and it continues one instruction rather than
  working a list.
- **Not `claude --resume`.** Resuming re-ingests the conversation that just failed to fit,
  which is the cost being avoided. A session starts clean and inherits a handoff.

## Design

**Two hooks enforce what instructions could not.** Both were measured: a session given a
budget in prose ignored it, and a session warned about its context ignored that too. Neither
mechanism below asks the model for anything.

**`PreToolUse` spends a budget and then permits only the way out.** It counts a session's
tool calls and, once the budget is gone, denies every call except writing the handoff. The
denial names the state and the remedy, and the model receives it verbatim. Measured: the
third call of a two-call budget was refused and the session peaked at **5,941 tokens of
12,288 — 48%**, where every earlier design reached compaction.

**`Stop` refuses to let a session end without a usable handoff.** It checks the file exists,
clears a size floor and carries a `Next`. Measured: the model tried to stop **twice** without
one and was refused both times before complying. It is bounded by `stop_hook_active` and a
try counter, so an autonomous run cannot be wedged by the hook that is supposed to protect
it; after two refusals it relents and the mechanical extractor takes over as the floor.

**A handoff is written while the session still has its context, and it carries results.**
The one produced under enforcement recorded the counts themselves — `Production 140, Preprod
133, Other 127` — and the caveat that words in a free-text field were excluded. That is what
a fresh session needs and what the extracted handoffs never had.

**The model writes it outside the repository being visited.** The file tools are confined
to the workspace, and `--add-dir` is what puts the session's own state directory in it — so
nothing is relocated afterwards and a visited repository still ends a session with exactly
the files the work changed. `Write` must be in `--tools`, which 0022's four already provide.

**Tool output is capped, because a budget on calls is not a budget on tokens.** One
unbounded `cat` fills a window inside a single permitted call, and the gate decides on the
context as it stood before that result arrived — so the reserve it holds back and the cap
on a result are one number. `BASH_MAX_OUTPUT_LENGTH` is where it goes: the harness
truncates at the tool, which is the last place a result can still be shortened.

**The supervisor chains sessions and owns everything outside the session.** It archives the
inherited handoff so `session-end.sh` always writes a fresh one, re-issues the original
instruction verbatim so the goal cannot drift through a chain, and stops when two
consecutive handoffs carry the same `Next`. A chain is one invocation; `-continue`,
`-resume <id>` and `-fork <id>` choose between chains, and starting clean is the default
because that is Claude Code's.

**Compaction stays refused.** It is not merely slower: on this machine it re-reads the whole
conversation before generating, and it was measured losing the goal it was summarising.

## Tasks

- [x] a `PreToolUse` budget denies further work once spent, permitting only the handoff, and
      what it permits leaves the session room to write it
- [x] a `Stop` hook refuses to end a session without a handoff carrying a `Next`, and gives
      up after two refusals rather than wedging the run
- [x] a tool result cannot spend more of the window than the gate reserves for one
- [x] the handoff is written outside the repository being visited, and `session-end.sh`
      writes one for every session in a chain
- [ ] a chain of sessions finishes a task no single session could, with each handoff
      carrying results rather than commands
- [x] two consecutive handoffs with the same `Next` stop the chain and name the file
- [x] a repository carries several chains; `localcode` starts a new one, `-continue`,
      `-resume <id>` and `-fork <id>` choose one, and `localcode sessions` lists them
- [x] the README says what the developer sees when a session hands over

## Open questions

- Whether the digest should carry each session's `Next` or its `Tried`. `Next` is what the
  session meant to do and `Tried` is what it did, and only a chain long enough to repeat
  itself will show which one prevents that.

## Log

- **Starting clean beat replaying and beat `--resume`.** Both re-ingest a conversation that
  had just failed to fit, and the measurement that motivated this feature is that a handoff
  costs 4,265 tokens where the conversation cost 37,837.
- **The staleness of an inherited handoff was found while planning this, not while using it.**
  It makes a chain freeze at its first handoff, and automatic handover is what turns that
  from an edge case into every session after the first.
- **One handoff per repository was wrong, and the correction is Claude Code's own model.** It
  assumed a repository holds one line of work, so a second task in the same checkout would
  have resumed the first and then overwritten its handoff. Chains are per invocation, the
  default is clean, and continuing is a choice with an id — which is what `claude` does and
  therefore what needs no explaining.
- **The feature was re-planned against measurements rather than patched.** Its first design
  had a supervisor end a session on the first compaction refusal and let `session-end.sh`
  write the handoff. Both halves were wrong: the refusal loop is a symptom of a context
  already full, and the handoff written at that point says `Prompt is too long`. What
  replaced it keeps the session away from the wall and enforces the handoff before it.
- **A hypothesis about Pi was refuted and is gone.** Its compaction was predicted to thrash
  at the committed 32,768; measured, it completed the same task without compacting once, and
  at a 12,288 wall it compacted five times and still finished. Pi under `localcode` is worth
  its own feature and is a `BACKLOG.md` line.
- **The enforcement was measured across six independent sessions, not one.** Every session
  stopped at its budget with peak context between 4,512 and 5,941 of 12,288 — 37% to 48% —
  so "keep the session small" holds without the model's cooperation. The `Stop` hook was
  refused twice by one session before it complied, which is the reason it exists.
- **A hook may state a fact but must not give an instruction.** A hook that told the model
  what to do was refused as injection, correctly: the model will not follow instructions
  arriving through a tool result. The protocol therefore belongs in the appended system
  prompt, which 0022 already sends, and the hook reports only the state.
- **A count of refused compactions measures turns, not pressure.** `PreCompact` fired once
  before every turn of an enforced session — at 4,325 tokens of an 11,264 window as readily
  as at 6,183 — so it is consulted per turn rather than at a threshold, and the twenty
  refusals this feature opens with are twenty turns rather than twenty attempts to survive.
  `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE` was tried against it and changed nothing, so nothing
  here sets it.
- **The cap is a setting rather than a `PostToolUse` hook, and the spill went with it.**
  That hook runs once the result is already in the conversation, so it can add context and
  cannot remove any: what it would have capped is spent by the time it is asked. The
  harness truncates at the tool instead, sized from the window, and a session that needs
  what was dropped re-runs the command through a filter — which is cheaper here than
  reading a spill file back in.
- **A ceiling expressed as a fraction cannot bound a session's peak, so the box asks for
  what it can.** One tool result and one assistant turn land after the last call the gate
  permits, so a session held at half the window peaks above half; the 37–48% that was
  measured is what a two-call budget produced, not what a 50% ceiling guarantees. The
  ceiling is therefore derived from what has to fit after it — one capped result, the turn
  that asked for it, and the turn that writes the handoff — and the requested fraction
  binds only while it is the smaller of the two.
- **The gate is the launcher run as a hook, not a fourth shell script.** It reads a
  transcript and counts against a budget, which is a decision rather than a record, and
  0021 put decisions where `make check` covers them. The three hooks 0016 ships stay shell
  because they extract and record.
- **Three numbers bound any test of this feature.** Decode is 5–10 tok/s, so a session needs
  ~200 s to generate before it can write a handoff and 600 s is the practical floor;
  Claude Code's prompt budget is the declared window minus the output reservation, so
  declaring less than about 8 k leaves no room for the preamble; and a fixture whose work
  collapses into one command never exercises a chain at all.

