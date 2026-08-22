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
- **The handoff, reconciled with `kit`** — 0016 puts `HANDOFF.md`, three hooks and a box parser
  in localcode, but `kit` owns the work protocol: it writes AGENTS.md, and `kit next --json`
  already publishes the topmost box the driver re-derives. Handing off inside a box is the same
  category of rule as doing the topmost one. Promote when: a second project wants it, or the box
  parser drifts from kit's format. The unresolved part is whether kit should ship one vendor's
  shell hooks at all, or only the rule.
- **Bounding what a tool result may put in the context** — one real session reached its
  limit with 74.6% of the window spent on tool results and 23.6% on the calls themselves,
  against 1.7% of the model's own text and two user messages. Nothing caps a command's
  output, so `seq 1 100000` costs a session. 0023 makes a full context survivable rather
  than fatal, and does not slow down how fast one fills. Promote when: a session hands over
  more than once on work that should have fitted, or when the cap is cheap enough to try —
  head-limiting Bash output is a hook, not a feature.
- **Pi under `localcode`** — measured against Claude Code on one profiling task at a 12,288
  wall: Pi compacted five times and finished; Claude Code died. At a 32,768 window Pi
  finished without compacting at all, on 22,845 ingested tokens against Claude Code's
  25,582. It also ships the session model 0023 is building — `--continue`, `--resume`,
  `--fork` — and caps tool output at 50 KB or 2,000 lines. The sandbox and server lifecycle
  in `localcode` are harness-agnostic, so driving Pi is a wiring question rather than a
  rewrite. Against it: 0010 measured 12/15 against 14/15 over five fixtures at three passes,
  and one task does not overturn that. Promote when: 0023 ships and the handoff chain is
  still dearer than Pi's compaction, or when a second harness is wanted for any other reason.

