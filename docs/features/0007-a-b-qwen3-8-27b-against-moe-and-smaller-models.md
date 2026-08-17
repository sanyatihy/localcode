---
id: 0007
title: A/B Qwen3.8-27B against MoE and smaller models
status: Draft
created: 2026-08-17
shipped:
check:
checked:
review:
needs: 0002, 0003
related: 0004
---

## Problem

Qwen3.8-27B is the project's premise, not a proven choice. It is dense, so every token
pays for all 27B parameters, and on 32 GB it forces a hard trade against context. A MoE
of similar total size activates a fraction of its weights per token and may be far faster
at comparable memory; a smaller dense model may free enough memory to double the context.
For agentic coding, more context and more speed can beat more parameters — and that is
testable rather than arguable.

## Non-goals

- **No fine-tuning of any candidate.** Out of scope project-wide; candidates run as they ship.
- **No quant sweep per model.** Each candidate runs at the quant class 0004 settled on;
  a full grid per model is a combinatorial trap.
- **No open-ended model survey.** Three named candidates. Others are BACKLOG lines, per
  the vision's "not a model zoo".

## Design

Three candidates, each representing a different bet about what agentic coding needs:

| Candidate | The bet | Known risk on 32 GB |
|---|---|---|
| `Qwen3.8-27B` (dense) | Per-token quality wins | ~16.4 GB at Q4_K_M leaves little for context |
| `Qwen3.6-35B-A3B` (MoE, ~3B active) | Speed and context win | Larger total footprint; MoE quality per param is lower |
| A smaller dense Qwen3 (14B class) | Context headroom wins | May simply be too weak for multi-step tool loops |

Each is measured through 0003's ladder first to find its own envelope, then scored on the
harness at the largest context that fits it. **Comparing them at equal context would be
the wrong test**: context headroom is exactly the advantage a smaller model is being
considered for, so each competes at its own best feasible setting, with that setting stated.

Existence and exact identifiers of the smaller and MoE candidates are verified before any
download — the Qwen line moves fast and the 3.8 release is recent enough that the sibling
line-up should be confirmed, not recalled.

The decision rule from 0004 applies unchanged, so results are comparable across features.

## Tasks

- [ ] The exact identifiers of all three candidates are confirmed to exist, with quant availability recorded
- [ ] Each candidate's memory envelope is measured with 0003's ladder, and its best feasible context recorded
- [ ] Each is scored 3× on the harness at that context, with truncation failures counted separately
- [ ] The comparison is written up with each model's best feasible setting stated alongside its score
- [ ] The winner and the margin are recorded in `docs/TECH.md`, along with what would change the answer

## Open questions

- If the MoE wins on speed but the dense model wins on success rate, which is the default?
  Leaning **success rate**, provided speed clears 0004's interactive threshold — a fast
  model that fails the task is not cheaper.

## Log
