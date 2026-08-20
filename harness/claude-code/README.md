# Claude Code — working, and unlike the other three

Claude Code is the only candidate that speaks the **Anthropic Messages** API rather than
OpenAI chat-completions, and the only one configured entirely through environment
variables: [`claude-code.env`](claude-code.env) is its provider block, and every variable
in it comes from Claude Code's own documentation read against 2.1.233.

    set -a; . harness/claude-code/claude-code.env; set +a
    claude -p "…" --tools Bash,Edit,Read,Write

## It needs the server configured for it

llama-server serves `/v1/messages` and `/v1/messages/count_tokens` and converts to its
chat-completions path internally, so there is nothing in the request path. Two things on
the server side are not optional:

- **`config/agent.env`, not `config/tuned.env`.** Sampling and the thinking toggle are
  per-request for the scorer and Claude Code sends neither, so they have to be served as
  defaults. Without that this model thinks at `xhigh`.
- **A one-line chat-template override.** Claude Code sends a `role: "system"` message
  *after* the user turn — the `mid-conversation-system-2026-04-07` capability — on every
  request, with 21 tools defined or with none, and no documented variable stops it.
  Qwen3.8's own template raises on a non-leading system message; llama.cpp returns that as
  a 500 and Claude Code retries ten times and dies.
  `config/templates/qwen3.8-system-anywhere.jinja` renders it as its own ChatML block
  instead.

Anthropic documents an automatic retry that disables the capability after such a
rejection, but it matches on the upstream's error wording — a Jinja exception carries
none, so the recovery path never fires.

## `--tools` is the whole story

Restricted to four coding tools it is the cheapest harness measured; on its defaults it is
the most expensive. The `CLAUDE_CODE_DISABLE_*` variables move the total by 7% and remove
no tool from the request. See the table in [../README.md](../README.md).

## Driving it from the editor

The extension does not read the env file — it spawns its own process. What it does read is
`env` in the project-scoped `.claude/settings.local.json` of the directory it was opened
on, so that is where the configuration goes:

    scripts/claude-code-settings.sh          # writes .claude/settings.local.json here

Generated from [`claude-code.env`](claude-code.env) rather than copied, because two forms
of one configuration drift. The file stays untracked: committed, it would route every
session anybody opens on this repository at a 27B model on loopback.

**Scope is the whole point.** These variables must not go in user settings or
`~/.claude/settings.json` — from there they would redirect every Claude Code session on the
machine, including whatever else the editor is open on. Project scope is keyed by the
directory, so only sessions started in this checkout are affected.

VS Code's own `claudeCode.environmentVariables` is what the general instructions name, and
in a workspace `.vscode/settings.json` it does **not** take: an extension session opened on
this checkout with it set kept using the hosted backend and the local server saw no
request. Only the user-settings form of it works, which is the scope this must not use.

Confirmed by running the CLI in a checkout with **no environment set at all** — the session
answered and the request arrived at the local server, so the settings file supplied both
the endpoint and the credential.

## The session handoff

`HANDOFF.md` at the root of the checkout carries working state **inside** one task box:
which files matter, what was tried, what is next. It is untracked — the branch's commits
and the feature doc's boxes are what carry state between features, and a reviewer should
read those rather than a diary — and it is written in this shape:

    # Handoff
    **Box:** the task box being worked, copied from the feature doc
    **Files:** each path that matters, and why it does
    **Tried:** what was done, and what came of it
    **Next:** the one thing to do next

Under 40 lines, because every line is read again at every session start.

[`hooks/session-start.sh`](hooks/session-start.sh) prints it, and Claude Code adds a
`SessionStart` hook's plain-text stdout to the session's context — so printing is what
injects it. With no handoff to print it prints the shape above instead; either way it
prints the standing instruction to keep the file current.

[`hooks/pre-compact.sh`](hooks/pre-compact.sh) refuses every compaction and appends what
fired to `results/precompact.jsonl`. Exit 2 is the only code that blocks one; neither
stream reaches the model, so the record is a file. Both triggers are refused, because a
manual `/compact` re-ingests the conversation exactly as an automatic one does.

**A refused compaction does not end the session**, measured at 2.1.233. The turn
completes normally and `PreCompact` fires again on the next one, once per turn for as
long as the conversation stays over the threshold. What ends a session here is the
declared window: `CLAUDE_CODE_MAX_CONTEXT_TOKENS` refuses a send that would exceed it,
and refusal is what stops the client spending generation on a summary in the meantime,
not what bounds the session.

[`hooks/session-end.sh`](hooks/session-end.sh) writes a handoff when the session wrote
none, and leaves one that exists alone. It calls no model: it reads the transcript for what
is recorded mechanically — the files the session edited and read, its last few tool calls,
and the last thing it said — because spending generation on a summary after the session has
ended buys even less than compaction does. Everything it extracts is cut to one line, since
a shell command in a transcript can be a whole heredoc.

[`hooks.json`](hooks.json) is a settings document rather than a fragment, so one committed
file has two readers: `claude --settings harness/claude-code/hooks.json` takes it directly,
and `scripts/claude-code-settings.sh` merges it into `.claude/settings.local.json` for the
extension, which reads only that. The command is written against `$CLAUDE_PROJECT_DIR`,
which is the checkout — a feature is always worked in a worktree of its own, so an absolute
path would run in only one of them.

`cmd/handoff` is what runs the box across those sessions:

    go run ./cmd/handoff -doc docs/features/0016-hand-off-between-sessions-instead-of-compacting.md \
      -sessions 5 -budget 30m -results results/handoff.jsonl

It reads the doc's topmost unticked box, starts a session, and asks the doc again — the box
is the record, not what the session says about itself. It stops when that box is ticked
(exit 0) or when the bound is reached (exit 1), and clears `HANDOFF.md` on the tick, since
working state inside a finished box is a stale instruction to the next one.

Each session gets a `CLAUDE_CONFIG_DIR` of its own, so nothing one learned reaches the next
except through the handoff, and its transcript is the only one in that directory. That is
also where the credential would have been: a session started this way has the one
`claude-code.env` sets and no other. Every row carries the session's peak context — the
largest single turn, since a session refused a compaction keeps growing — its turns, its
wall clock, and how many compactions were refused during it.

Verified in both readers at 2.1.233: a marker written into `HANDOFF.md` came back out of a
fresh `claude -p` session, through `--settings` and through the project-scoped file. The
refusal was verified on both triggers — `/compact` in a print session, and an automatic one
forced by `--autocompact 100000` against a conversation deliberately grown past it. All
three events fire in a print session, which is the form the driver runs.

## It cannot be offline

`ANTHROPIC_BASE_URL` routes every model call, so no prompt reaches a hosted model. It does
not route the rest: OAuth refresh, feature-flag fetches, the fast-mode availability check
and WebFetch's domain-safety preflight go to Anthropic hosts regardless.
`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` covers some of that and the observed hosts
are measured rather than assumed — see
[0008](../../docs/features/0008-wire-the-winning-config-into-the-coding-agent.md).
