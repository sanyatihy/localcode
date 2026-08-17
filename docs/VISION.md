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
- The serving A/Bs are settled by that scorer and the winners recorded in `docs/TECH.md`
  with the numbers that beat the alternatives: **quantisation × context**, **sampling
  and tool-call format**, **llama.cpp vs MLX**, **model vs model**.
- The harness question is answered the same way: **Pi** and **Hermes Agent** run the
  same tasks against the same endpoint, and one is the default.
- **Cursor** is tested against the local endpoint, with its cost to the on-device
  property measured and stated rather than discovered later.
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
- **On-device by default; every exception named and bounded.** The build loop runs
  locally. Two exceptions are deliberate and must always be labelled as such: Cursor
  routes requests through its own backend and cannot reach loopback, so any Cursor
  setup sends code off the machine; and the coordination flow, if built, sends planning
  context to a frontier model. Neither may be the silent default.
- **Go for anything built.** One static binary, no runtime competing with the model for
  the 32 GB it needs. Where a tool is Python-only — MLX above all — it is confined to
  its own environment and called, never merged into the Go code.
- **Reproducible over convenient.** Every server invocation, quant and sampling setting
  lives in the repo. A number nobody can regenerate is not evidence.
- **Measurement before tuning.** No optimisation is adopted without a before/after on
  the committed scorer.
