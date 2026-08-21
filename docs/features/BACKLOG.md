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

## What the 128 GB machine unlocks

One trigger, so one entry. Six separate lines used to wait on this machine, which made the
file look like six ideas rather than one purchase. Promote whichever of these the arriving
hardware actually makes answerable:

- **The quant ladder.** 0004 was dropped because the admissible wired budget on 32 GB is
  ~22.2 GB and Q4_K_M already sits at 21.75, so Q5_K_M and Q6_K are excluded *by projection
  rather than by measurement*. More memory makes the exclusion testable.
- **Quantising the model ourselves**, if that ladder shows a gap in the published quants
  worth filling.
- **Harder tier-1 tasks.** 0013 added seven traps and this model took only one: twelve of the
  fourteen tasks show no quality failure at any setting. The traps are proven to trap, so this
  is the model being good rather than the fixtures being soft. Needed only when a comparison
  requires tier-1 to *rank* rather than floor-check, which a quant grid would be the first to.
- **One pass for the floor-check tasks, three for the ones that move.** Their repeats measure
  nothing and cost most of a sweep's runtime. Worth doing only when sweep runtime is what
  blocks a feature; no sweep so far has been large enough.
- **Qwen3-Coder-Next (80B-A3B)** — MoE built for agentic coding, 3B active. At ~45 GB for Q4 it
  does not fit 32 GB at all, and the vision rules out a model zoo, so it needs both the memory
  and a reason.
- **Hermes' persistent memory over a real week.** 0010 scores all four harnesses cold, which is
  the only reproducible comparison but deliberately blind to Hermes' main bet — and Hermes is
  unattended-only here, its 64,000-token floor sitting above the attended ceiling. More memory
  is what would let it be used attended long enough to answer.
