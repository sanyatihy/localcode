# Backlog

Ideas that are not features yet. Nothing here is scheduled or prioritised — the
default state is **stays frozen**.

Brainstorm output lands here. Promote one to its own `NNNN-*.md` with `kit new`
only on a real signal: someone asked, a metric moved, something else needs it.

Keep each entry to a few lines: what it is, and what would make it worth doing.

- **Constrained decoding, when something actually emits malformed calls** — 0005 dropped GBNF
  because 150 tool-call runs produced zero unparseable and zero schema-invalid calls, leaving
  a grammar nothing to fix. Promote when: a model or harness appears whose failures are format
  rather than choice. Note the template asks for an XML call form, so a JSON-schema constraint
  is the wrong tool even then.
- **Record memory per eval run** — the vision requires runs to record free memory and swap so
  contamination is detected rather than assumed; `scripts/ladder.sh` does, `cmd/eval` does not,
  so a sweep's cleanliness is argued from stable tok/s rather than measured. Promote when: a
  sweep runs close enough to the memory ceiling that the argument stops being convincing.
- **Speculative decoding with a draft model** — pair the 27B with a 0.6B/1.7B Qwen3
  draft to cut generation latency. Promote when: 0004 has settled the main config and
  generation speed is the thing still failing the interactive threshold. Deliberately
  not a feature yet — it spends memory that 0003 may show is not there.
- **Qwen3-Coder-Next (80B-A3B)** — MoE built for agentic coding, 3B active. Promote when:
  someone confirms a quant fits 32 GB with usable context; at ~45 GB for Q4 it currently
  does not, and the vision rules out a model zoo.
- **The quant ladder, on a bigger machine** — 0004 was dropped because the admissible wired
  budget on 32 GB is ~22.2 GB and Q4_K_M already sits at 21.75, so Q5_K_M and Q6_K are
  excluded *by projection rather than by measurement*. Promote when: the 128 GB machine
  arrives, which moves that budget and makes the exclusion testable instead of assumed.
- **Quantising the model ourselves** — build custom quants rather than taking unsloth's.
  Promote when: 0004 shows the available quant ladder has a gap worth filling.
- **Hermes' persistent memory over a real week** — 0010 scores both harnesses cold,
  which is the only reproducible comparison but deliberately blind to Hermes' main bet.
  Promote when: a harness is in daily use and the question is whether it improved on
  this repo specifically.
- **Warm prompt-cache reuse across agent turns** — llama-server can hold KV across
  requests; agent loops resend a near-identical prefix every turn. Promote when: 0008
  or 0010 shows prompt processing dominating real session latency.
- **Cursor's built-in assistant via an HTTPS tunnel** — rejected in 0008 because it
  routes through Cursor's backend, so code leaves the machine. Promote when: Cursor
  supports calling a base URL from the client, which would remove the entire objection.
- **docs/TECH.md as-built** — the vision expects durable facts to land there; it does not
  exist yet. Promote when: 0001 ships and there is a first fact to record.
- **Harder tier-1 tasks that this model actually fails** — 0013 added seven traps and
  Qwen3.8-27B took only one of them: eleven of fourteen tasks score 3/3 at every setting
  measured. The traps are proven to trap (fixture self-tests fail the tempting answers), so
  this is about the model being good at these shapes rather than the fixtures being soft.
  Promote when: a comparison actually needs tier-1 to *rank* rather than to floor-check —
  0004 finding two quant configs it cannot separate, or 0007 comparing models, would both do
  it. Deliberately not scheduled: chasing traps a capable model will fail is open-ended, and
  0010 ranks harnesses on tokens and turns instead, which needs no ranking from tier-1.
- **One pass for floor-check tasks, three for the ones that move** — the twelve flat tasks are
  3/3 at every setting, so their repeats measure nothing and cost most of a sweep's runtime.
  Promote when: sweep runtime is the thing blocking a feature, which 0004's grid is the first
  candidate for.
