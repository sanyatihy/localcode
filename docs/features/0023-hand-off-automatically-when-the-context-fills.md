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
tool calls, reads its context out of the transcript, and once either bound is gone denies
every call except writing the one handoff the session was given. The denial names the state
and stops there — the remedy is in the appended system prompt, because an instruction
arriving through a tool result is refused as injection. Measured: the third call of a
two-call budget was refused and the session peaked at **5,941 tokens of 12,288 — 48%**,
where every earlier design reached compaction.

**A turn is bounded as well as a session.** The harness issues a turn's tool calls together
and the transcript does not change while they run, so one reading otherwise decides any
number of them: a five-call turn carried the context **1,960 tokens past a ceiling it had
been under**. Four calls to a turn is what makes the ceiling's reserve a bound rather than
a hope, and the reserve is a quarter of the window because a measured four-call turn cost
530 tokens a call. What that bound holds back costs the session nothing: a call deferred to
be measured again is not a call the session chose to spend, and the appended system prompt
says so, since a session that did not know would lose most of a batch to it.

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

**The supervisor chains sessions and owns everything outside the session.** It is also the
only clock: a session is stopped if it runs past `-session-timeout`, because a denied call
costs a turn like any other and nothing else here bounds how many of them a session may
spend. Each session gets a directory of its own, so the handoff it inherits and the handoff it writes are never
the same file and `session-end.sh` always finds a fresh one to fill. It re-issues the
original instruction verbatim so the goal cannot drift through a chain, and stops when two
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
- **A denied call still costs a turn, so a session needs a clock as well as a budget.** At
  this depth a turn is minutes, and nothing in the design bounded how many of them a
  session could spend being refused: a session that answers a spent budget by trying
  another tool would run until its thirty calls were gone. `-session-timeout` is the bound,
  and a session that reaches it ends the chain rather than handing on a guess.
- **The ceiling is the headroom, and the fraction was an arbitrary number that made the
  feature useless.** At half the window a session's room was one turn wide: it spent it on
  the reads `Edit` requires and was refused before it could change anything, twice over,
  so the chain wrote handoffs and never touched a line. The reserve is what makes a ceiling
  safe, so a fraction below it buys nothing and costs a handoff — about 4,265 tokens. The
  default is now as high as the arithmetic allows and `-ceiling` only lowers it.
- **A handoff cannot carry the right to edit, only the diagnosis.** `Edit` fails on a file
  the session has not `Read` — the harness says so in as many words — so every session pays
  the read for every file it changes, however well the handoff describes it. That sets the
  floor on what a session can do: at this wall a read and an edit cost about 350 tokens a
  file, so a session's working room buys two or three of them and the chain needs a session
  per two or three files.
- **Over-reading is the part that was fixable, and the fix is prose.** Sessions read every
  file in the fixture before changing any, three chains running, and spent their whole
  ceiling doing it: twelve calls, nine calls, eleven calls, not a line edited. The appended
  system prompt now tells a session two things it could not otherwise know — that a handoff
  is established rather than a claim to check, and that `Edit` refuses a file this session
  has not read, so a read not followed by a change is room spent for nothing. Neither is a
  bound, and neither has to be: the mechanism keeps the session safe whether it is followed
  or not. A rule that forbade re-reading would forbid what `Edit` requires.
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
  what it can.** A whole turn of results and the turn that asked for them land after the
  last call the gate permits, so a session held at half the window peaks well above half —
  measured, between 6,434 and 8,558 of an 11,264 window. The 37–48% recorded above is what
  a two-call budget produced, not what a 50% ceiling guarantees. What a ceiling must
  reserve is what lands after it, which is what it is now derived from.
- **The gate is the launcher run as a hook, not a fourth shell script.** It reads a
  transcript and counts against a budget, which is a decision rather than a record, and
  0021 put decisions where `make check` covers them. The three hooks 0016 ships stay shell
  because they extract and record.
- **Three numbers bound any test of this feature.** Decode is 5–10 tok/s, so a session needs
  ~200 s to generate before it can write a handoff and 600 s is the practical floor;
  Claude Code's prompt budget is the declared window minus the output reservation, so
  declaring less than about 8 k leaves no room for the preamble; and a fixture whose work
  collapses into one command never exercises a chain at all.

