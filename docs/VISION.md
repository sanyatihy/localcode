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

- **Hardware is the current envelope, not a permanent one.** M2 Max, 32 GB unified memory,
  30 GPU cores. A 128 GB machine is planned, and it moves every ceiling at once: quants
  that do not fit today, models that cannot load, contexts that cost too much. So
  **measured numbers are always recorded with the machine they came from**, and anything
  that hardcodes this machine's limits — ladder rungs above all — is a defect to fix rather
  than a value to update. No config may make the machine unusable for the editor and
  browser the developer is running while the agent works.
- **The job is to find which constraint binds, not to assume one.** Memory, ingest time,
  quality, and whatever else emerges are candidates, and which one binds depends on the
  envelope — model, quant, context, and the machine. A constraint asserted in advance is
  what sends a sweep looking in the wrong place, which has already happened once here.
- **What has been measured so far, and its exact scope.** In the envelope
  *Qwen3.8-27B · Q4_K_M · contexts to 64k · 32 GB*, **time bound before memory did**: the
  full ladder ran with swap delta 0.0 MB at peak resident 17.91-20.27 GB, while ingest cost
  rose to 13.1 minutes at 64k as the prompt rate decayed from 109 to 75 tok/s.
  **This does not rule memory out.** Only one quant was tested. Q6_K weights are roughly
  6 GB heavier, larger models and longer contexts are untested, and each moves the envelope.
  The finding is that memory did not bind *here*, not that it does not bind.
- **Headroom exists at this quant and should be spent deliberately.** 64k/Q4_K_M left
  roughly a third of the machine unused, which is why Q5_K_M and Q6_K belong in 0004's
  sweep — and why that sweep is also where memory gets its next chance to bind.
- **The optimum is expected to differ by task, not just by profile.** A short tool-calling
  turn, a long refactor and a repo-wide search have different context and latency needs, so
  "one winning config" is an assumption the results have to earn rather than a goal.
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
