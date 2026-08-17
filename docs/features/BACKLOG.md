# Backlog

Ideas that are not features yet. Nothing here is scheduled or prioritised — the
default state is **stays frozen**.

Brainstorm output lands here. Promote one to its own `NNNN-*.md` with `kit new`
only on a real signal: someone asked, a metric moved, something else needs it.

Keep each entry to a few lines: what it is, and what would make it worth doing.

- **Speculative decoding with a draft model** — pair the 27B with a 0.6B/1.7B Qwen3
  draft to cut generation latency. Promote when: 0004 has settled the main config and
  generation speed is the thing still failing the interactive threshold. Deliberately
  not a feature yet — it spends memory that 0003 may show is not there.
- **Prompt-cache reuse across agent turns** — llama-server can keep KV across requests;
  agentic loops re-send a near-identical prefix every turn. Promote when: 0008 shows
  prompt processing dominating real session latency.
- **Qwen3-Coder-Next (80B-A3B)** — MoE built for agentic coding, 3B active. Promote when:
  someone confirms a quant fits 32 GB with usable context; at ~45 GB for Q4 it currently
  does not, and the vision rules out a model zoo.
- **Quantising the model ourselves** — build custom quants rather than taking unsloth's.
  Promote when: 0004 shows the available quant ladder has a gap worth filling.
- **docs/TECH.md as-built** — the vision expects durable facts to land there; it does not
  exist yet. Promote when: 0001 ships and there is a first fact to record.
