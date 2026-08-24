# Backlog

Ideas that are not features yet. Nothing here is scheduled or prioritised — the
default state is **stays frozen**.

Brainstorm output lands here. Promote one to its own `NNNN-*.md` with `kit new`
only on a real signal: someone asked, a metric moved, something else needs it.

Keep each entry to a few lines: what it is, and what would make it worth doing.

An entry earns its place by being consulted when its trigger fires. A decision already
taken belongs in `docs/TECH.md`, which is where anyone would look for it; an idea whose
trigger nobody watches is not an idea, it is a hedge.

- **The attended desktop verdict for the MTP config** — 0017 adopted it for the grind
  profile and measured it `unattended` only. It peaks at 22.10 GB where 0014's desktop died
  at 22.29, so the margin is 0.19 GB and the answer is not obvious. One 90-second screen with
  somebody driving the machine settles it. Promote when: anyone wants to run the fast config
  and use the Mac at the same time — which is the likeliest trigger in this file, since the
  developer owns the machine.
- **The host-RAM prompt cache, against the memory ceiling** — llama-server keeps prefixes it
  has evicted from a slot in host RAM and restores them, which is what makes reuse survive
  traffic beside a conversation (0018). The budget is `--cache-ram`, **8192 MiB by default and
  unset in every config here**, bought from the same 32 GB the weights and the KV reservation
  sit in. 0014's attended ceiling was walked without accounting for it, so the admissible
  budget may be smaller than measured. Promote when: the ceiling is re-walked, or a session at
  depth is seen swapping with the model's own footprint unchanged.
- **Restricting the editor's tool set through the `agent` setting** — the extension spends
  36,309 tokens of preamble before anything is typed and has no equivalent of `--tools`, which
  costs 462 s a turn against 45 s in a terminal (0008). The `agent` setting is documented to
  run the main thread as a named subagent and apply *its* tool restrictions, which would be
  `--tools` by another route. Untested, and cheap to test. Promote when: the editor flow is
  something somebody wants to use daily rather than prove works.
- **`--tools` is variadic, so the instruction can be swallowed** — `internal/harness/claudecode.go`
  and `cmd/handoff` both pass `--tools <list> --permission-mode <mode> <instruction>`, and both
  work only because `--permission-mode` terminates the tool list. Drop that flag from either and
  the instruction joins the tools, the run dies asking for input, and nothing says why. Promote
  when: either is edited for any other reason — a `--` separator or the instruction on stdin
  costs one line each. **Its real home is a comment at both call sites**, since that is where it
  fires; it sits here until someone puts it there.
- **Retire the `## Tasks` parser, which is the one place this repo re-implements `kit`** —
  `internal/handoff` reads kit's doc format and `cmd/handoff` drives "the topmost unticked
  box" from it, which is what `kit next --json` already publishes. VISION now says this is
  the wrong side of the boundary. Two things have to land before it can go, and both are
  cheaper than reconciling the parser: 0026 gives a chain an external success test that is a
  command and an exit code rather than a ticked box, and a real chain has already shown the
  model driving `kit next`, `kit claim` and `kit ship` itself, unaided, better than the
  parser did. Promote when: 0026 ships, or the parser drifts from kit's format — whichever
  comes first. The unresolved part is whether `cmd/handoff` survives at all once its only
  unique property, checking the doc rather than the model's word, is general.
- **Pi under `localcode`** — measured against Claude Code on one profiling task at a 12,288
  wall: Pi compacted five times and finished; Claude Code died. At a 32,768 window Pi
  finished without compacting at all, on 22,845 ingested tokens against Claude Code's
  25,582. It also ships the session model 0023 is building — `--continue`, `--resume`,
  `--fork` — and caps tool output at 50 KB or 2,000 lines. The sandbox and server lifecycle
  in `localcode` are harness-agnostic, so driving Pi is a wiring question rather than a
  rewrite. Against it: 0010 measured 12/15 against 14/15 over five fixtures at three passes,
  and one task does not overturn that. Promote when: 0023 ships and the handoff chain is
  still dearer than Pi's compaction, or when a second harness is wanted for any other reason.
- **`serverUp`'s error text reaches nobody** — it composes a message naming the command that
  fixes a dead endpoint, and both callers in `cmd/localcode/main.go` test it with `== nil` and
  throw it away. What a user actually sees is `no server at <url>, and -no-serve was given`, or
  a twenty-minute wait. Two call sites, one line each. Promote when: someone hits a dead
  endpoint and cannot tell from the output what to start — or when that file is open anyway.
- **A controlled chain on real source at both ceilings** — 0034 settled the driver's default
  on two readings that disagree, and neither is the measurement wanted. The fixture pair is
  controlled but runs work whose calls cost a third of real ones, so the small ceiling never
  binds in it; the real-source pair binds every session but interleaved the two
  configurations across phases of one instruction, so its arms did different work. What
  would settle it is one instruction on a real repository run twice, once per ceiling, from
  the same starting commit — scored by what landed rather than by wall clock, since the two
  are within 3% per tool call. Promote when: the default is questioned again, or a repository
  and an instruction worth spending two full chains on are in hand.
- **The output reservation is subtracted twice** — `declaredFromServer` sets
  `CLAUDE_CODE_MAX_CONTEXT_TOKENS` to what is served less the reservation, and `NewLimits`
  then takes the reservation off that again to get the window. The second is Claude Code's
  own behaviour, measured; the first applies the same reasoning a second time, so about
  4,096 tokens of every served context go unused. At 49,152 that costs 3,072 of ceiling and
  nine tool calls — 22,528 and 51 against 25,600 and 60. It is not obviously wrong: the
  server may account for template or BOS tokens the arithmetic here cannot see, and getting
  it wrong means sessions dying on `Prompt is too long` after a cold ingest, which is the
  failure the whole reservation chain exists to prevent. Settle it the way the 4,096 floor
  was settled — pad a prompt to an exact token count against a 49,152 server with
  `CLAUDE_CODE_MAX_CONTEXT_TOKENS` set to the full 49,152, and bisect where the refusal
  lands. Promote when: the ceiling is the binding constraint on a chain worth the
  measurement, which it now is for every session on real source.
- **Spill a tool result to a file instead of dropping it** — Pi caps tool output at 50 KB or
  2,000 lines; `localcode` sets `BASH_MAX_OUTPUT_LENGTH` and the excess is simply gone, so a
  session that needed it re-runs the command through a filter and pays a turn plus the
  command's own time. 0023 considered a spill and dropped it, reasoning that a `PostToolUse`
  hook runs once the result is already in the conversation and so can only add context. That
  is true of `PostToolUse` and not of `PreToolUse`, which is the hook the gate already uses
  to rewrite an input — `ClampRead` narrows a `Read` there today, and the `Verdict.Input`
  channel it returns through is not specific to `Read`. A `Bash` command rewritten to
  redirect into the session directory and return only its tail would leave the whole output
  where the session can grep it, and name the path so the loss is visible rather than
  silent. Against it: a targeted re-run costs one call and so does a grep, so the win is on
  slow commands and on the session knowing what it lost, not on tokens. Promote when: a
  chain is measured losing work to a truncated result, or `go test` output at a real
  repository's size is shown to exceed the cap that a session then cannot recover.
