# VISION

What this project is for. Feature docs in [features/](features/) say what is being
built now; this says what it is being built *towards*, and it is what an agent reads
before drafting one.

## What this is

localcode makes one MacBook a viable place to run agentic coding: a quantised
Qwen3.8-27B served locally, tuned by measurement rather than guesswork, driven by a
harness chosen on evidence. The work is **how the model is launched, served and
driven** — not the model's weights, and not a new agent.

## Who it is for

One developer on an M2 Max doing agentic coding — multi-turn, tool-calling, repo-scale
edits — who wants the grind to happen on-device: no per-token cost, no network
dependency for the build loop. Secondarily, anyone reproducing the same choices on
comparable Apple Silicon, which is why every number here is committed rather than
remembered.

## What done looks like

- One command starts the endpoint, and a real harness completes real tasks against it.
- A committed scorer reports, per config: **task success rate**, tool-call validity
  rate, generation tok/s, prompt-processing tok/s, peak memory, context headroom.
  Re-running it reproduces the numbers.
- **Two profiles are settled, not one.** *Attended* is the interactive loop with a human
  watching, where per-turn latency is the binding cost and a person catches mistakes
  cheaply. *Unattended* is the long autonomous grind, where latency barely matters and
  errors compound because nobody is watching. They may want opposite settings — thinking
  above all — so every sweep reports per profile and the project may ship two configs.
- **The profiles differ mainly in what a minute is worth.** Attended pays ingest time out
  of a human's attention, so a 13-minute cold context is disqualifying there and merely
  slow unattended. They differ in memory too — attended has an editor and browser resident
  by definition — but 0003 measured that difference as not binding at any context this
  model serves, so latency is the axis that actually separates them.
- The serving A/Bs are settled by that scorer and the winners recorded in `docs/TECH.md`
  with the numbers that beat the alternatives: **quantisation × context**, **sampling
  and tool-call format**, **llama.cpp vs MLX**, **model vs model** — each per profile.
- The harness question is answered the same way: **Pi**, **Hermes Agent** and
  **OpenCode** run the same tasks against the same endpoint, scored against the
  incumbent — Claude Code — and one is the default.
- **The existing editor flow keeps working, with inference local.** Claude Code driven
  from the Cursor/VSCode extension against the local endpoint, with the traffic it still
  sends to Anthropic characterised rather than assumed away.
- **At least one path is genuinely offline.** A harness that needs no vendor
  reachability completes a real task with the network disabled.
- Optionally, a flow exists where a frontier model plans and coordinates while local
  models grind features — the roles `kit` already defines, split across two tiers.

## What it is not

- **Not a coding agent, and not a custom harness by default.** Pi and Hermes exist and
  are model-agnostic. Writing a third is a last resort that must be justified against
  both, not a starting assumption.
- **No fine-tuning.** Not LoRA, not QLoRA, not continued pre-training. The model is
  taken as it ships; everything here is launch, serve and drive. An idea that requires
  touching weights is out of scope, not a backlog item.
- **Not multi-user.** One machine, one developer. Auth, quotas and LAN serving would
  change every design here.
- **Not a model zoo.** A model earns a place by scoring on the scorer; "worth a look"
  is a BACKLOG line, not a download.
- **No per-request cloud fallback.** A router that silently retries against a hosted
  model would make the local numbers meaningless. This is distinct from the
  coordination flow above, where the tiers are separate roles a human assigned.

## Constraints

- **Hardware is fixed and is the design.** M2 Max, 32 GB unified memory, 30 GPU cores,
  ~249 GB free. No config may make the machine unusable for the editor and browser the
  developer is running while the agent works.
- **Time is the binding constraint, not memory.** This replaces the opposite claim, which
  0003 measured and refuted. The full ladder — 8k, 16k, 32k, 48k, 64k at q8_0 — ran with
  swap delta 0.0 MB, peak resident 17.91 to 20.27 GB of 32 GB, and marginal cost settling
  at 31-37 KB/token. What binds is ingest: the prompt rate decays with depth, 109 tok/s at
  8k down to 75 at 64k, so a cold 32k context costs 5.4 minutes and a cold 64k costs 13.1.
  **Context budget is still a first-class design parameter, but it is spent in seconds
  rather than gigabytes**, which is why it falls almost entirely on the attended profile.
- **There is memory headroom, and it should be spent deliberately.** 64k/q8_0 leaves
  roughly a third of the machine unused. Weight quality is the obvious buyer — Q5_K_M and
  Q6_K were excluded on a memory argument that measurement does not support — and choosing
  between more context and better weights is a real trade rather than a foregone one.
- **Two properties, never conflated.** *Inference is local* — no prompt or file content
  reaches a model this machine does not run. *The harness is offline* — it needs no vendor
  reachability at all. The first is required everywhere. The second is stronger, and
  Claude Code does not have it: it still contacts Anthropic for OAuth refresh and feature
  flags whatever `ANTHROPIC_BASE_URL` points at. So every setup states which of the two it
  has. What is ruled out entirely is an editor assistant that proxies prompts through its
  vendor, which fails even the first. The one deliberate exception is the coordination
  flow, which sends planning context to a frontier model and may never be silent.
- **Go for anything built.** One static binary, no runtime competing with the model for
  the 32 GB it needs. Where a tool is Python-only — MLX above all — it is confined to
  its own environment and called, never merged into the Go code.
- **Reproducible over convenient.** Every server invocation, quant and sampling setting
  lives in the repo. A number nobody can regenerate is not evidence.
- **Measurement before tuning, and measurement over reasoning.** No optimisation is
  adopted without a before/after on the committed scorer. Arithmetic about this machine has
  now been wrong three times — predicted KV cost was ~4x the measured figure, free memory
  was read as saturation when macOS keeps it near zero regardless, and the memory ceiling
  did not exist at all. **A number that was derived rather than observed is a hypothesis,
  and it is labelled as one until a run confirms it.**
- **A swapping machine is not a slow machine, it is an invalid measurement.** On 32 GB
  the model plus normal desktop apps can exhaust memory before the context is full, and
  every timing taken in that state measures paging rather than inference. Runs record
  free memory and swap so contamination is detected rather than assumed, and a run that
  swapped is reported as void, not as a slow pass.
- **One toggle at a time, and never one that moves two things.** Thinking mode carries its
  own recommended sampling — `temp 1.0 / top_p 0.95 / top_k 20` with thinking, `temp 0.7 /
  top_p 0.80 / top_k 20 / presence_penalty 1.5` without. Comparing thinking on against
  thinking off at a single fixed temperature measures the pair, not the toggle, and any
  result that does so is void.
