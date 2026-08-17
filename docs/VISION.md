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
- **The profiles have different memory budgets, not only different latency tolerances.**
  Attended means an editor and a browser are open *by definition*, so the model gets what
  is left; unattended can have the machine. That is the same split as 0003's working and
  hard ceilings, and binding them is what stops a config being recommended for attended
  use on numbers measured with nothing else running.
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
- **Memory is the binding constraint, and context is what it buys.** Qwen3.8-27B is
  dense: 64 layers, 4 KV heads, head_dim 256 — so KV cache costs ~256 KiB/token at f16,
  ~128 KiB/token at q8_0. Against ~16.4 GB of Q4_K_M weights, 32k context lands near
  22 GB and 64k does not fit. **Context budget is a first-class design parameter in
  every feature, not a flag chosen at the end.**
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
- **Measurement before tuning.** No optimisation is adopted without a before/after on
  the committed scorer.
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
