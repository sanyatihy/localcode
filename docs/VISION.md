# VISION

What this project is for. Feature docs in [features/](features/) say what is being
built now; this says what it is being built *towards*, and it is what an agent reads
before drafting one.

## What this is

localcode is a local coding-agent backend for one MacBook: a quantised Qwen3.8-27B
served over an OpenAI-compatible endpoint, plus the evidence used to choose its
configuration. It exists so agentic coding runs entirely on-device at a known quality
and speed, instead of at whatever a first-guess `llama-server` flag string produced.

## Who it is for

One developer on an M2 Max doing agentic coding — multi-turn, tool-calling, repo-scale
edits — who wants it offline: no code leaving the machine, no per-token cost, no
network dependency. Secondarily, anyone reproducing the same choice on comparable
Apple Silicon, which is why every number here is committed rather than remembered.

## What done looks like

- One command starts the endpoint, and a real coding agent completes real tasks against it.
- A committed harness reports, per config: **agentic task success rate**, tool-call
  validity rate, generation tok/s, prompt-processing tok/s, peak memory, and context
  headroom. Re-running it reproduces the numbers.
- The quantisation × context × sampling A/B is settled by that harness, and the winning
  config is the default, recorded in `docs/TECH.md` with the numbers that beat the rest.
- The runtime question (llama.cpp vs MLX) is answered by measurement on this hardware.
- Whether to fine-tune **at all** is answered by the same harness: a named deficit the
  config sweep could not close. If there is one, a LoRA that beats stock on that metric.

## What it is not

- **Not a coding agent.** It serves models to an agent that already exists; it does not
  implement tool loops, planning, or editing.
- **Not multi-user or remote.** One machine, one user, bound to loopback. Serving to the
  LAN, auth, and quotas are all out — they would change every design here.
- **Not a model zoo.** A model earns a place by scoring on the harness; "worth a look"
  is a BACKLOG line, not a download.
- **Not cloud fallback or routing.** A hybrid local/cloud router is a different project;
  admitting it would make offline-by-construction unverifiable.
- **Not a training project.** Fine-tuning is one contingent feature gated on evidence,
  not the point. Pre-training and full fine-tunes are impossible on this hardware anyway.

## Constraints

- **Hardware is fixed and is the design.** M2 Max, 32 GB unified memory, 30 GPU cores,
  ~249 GB free. No config may make the machine unusable for the editor and browser the
  developer is running while the agent works.
- **Memory is the binding constraint, and context is what it buys.** Qwen3.8-27B is
  dense: 64 layers, 4 KV heads, head_dim 256 — so KV cache costs ~256 KiB/token at f16,
  ~128 KiB/token at q8_0. With Q4_K_M weights at ~16.4 GB, 32k context lands near 22 GB
  and 64k does not fit without quantising KV further. **Context budget is a first-class
  design parameter in every feature, not a flag chosen at the end.**
- **Offline by construction.** No feature may send code, prompts, or telemetry off the
  machine. Model downloads are the sole exception and are explicit.
- **Reproducible over convenient.** Every server invocation, quant, and sampling setting
  lives in the repo. A number nobody can regenerate is not evidence.
- **Measurement before tuning.** No optimisation — quantisation, speculative decoding,
  fine-tuning — is adopted without a before/after on the committed harness.
