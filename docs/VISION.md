# VISION

What this project is for. Feature docs in [features/](features/) say what is being
built now; this says what it is being built *towards*, and it is what an agent reads
before drafting one.

## What this is

localcode makes a machine the developer controls a viable place to run agentic coding:
a quantised Qwen3.8-27B served on that machine, tuned by measurement rather than
guesswork, driven by a harness chosen on evidence. The first machine was one MacBook.
Since 2026-09-13 it may also be a second Mac on the same network, or a GB10 desktop
serving several developers at once. The work is **how the model is launched, served and
driven** — not the model's weights, and not a new agent.

## Who it is for

One developer on an M2 Max doing agentic coding — multi-turn, tool-calling, repo-scale
edits — who wants the grind to happen on-device: no per-token cost, no vendor
dependency for the build loop. The same developer with a second machine on a trusted
network — another Mac, or a GB10 — who wants the grind to happen there while the
laptop stays usable. A small team, four to begin with, sharing that GB10 as their
model endpoint. Secondarily, anyone reproducing the same choices on comparable
hardware, which is why every number here is committed rather than remembered.

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
- **The profiles differ in what a minute is worth, and in what the desktop can survive.**
  Attended pays ingest time out of a human's attention, so a 13-minute cold context is
  disqualifying there and merely slow unattended. Latency was expected to be the only axis
  separating them; 0014 found a second, and it binds harder. The GPU-wired budget the model
  competes for is invisible to swap and to resident size, and at 65,536 the compositor
  stops drawing while every request still succeeds — so the attended ceiling is **57,344**
  and the unattended one is 65,536.
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
- **A session on one machine drives a model served on another.** The launcher is pointed
  at a remote endpoint, every harness talks to that endpoint and nothing else, the sandbox
  admits that one host, and the client never starts or stops a server it does not own.
  Mac to Mac is the first case, because it needs no new hardware; the GB10 is the second.
- **The GB10 serves several developers at once, and the numbers say what that costs.**
  The slot count is a config setting, four at first. Per-user decode rate, ingest and
  prompt-cache reuse under shared traffic are measured on the committed scorer, with
  what binds at 128 GB found by the ladder rather than assumed from the laptop.
- Optionally, a flow exists where a frontier model plans and coordinates while local
  models grind features — the roles `kit` already defines, split across two tiers.

## What it is not

- **Not a coding agent, and not a custom harness by default.** Pi and Hermes exist and
  are model-agnostic. Writing a third is a last resort that must be justified against
  both, not a starting assumption.
- **No fine-tuning.** Not LoRA, not QLoRA, not continued pre-training. The model is
  taken as it ships; everything here is launch, serve and drive. An idea that requires
  touching weights is out of scope, not a backlog item.
- **Not a second `kit`.** `kit` owns the work protocol: what a feature is, which one is
  next, who has claimed it, and when it is shipped. It writes `AGENTS.md` and publishes
  `kit next --json`. Nothing here re-derives any of that. This project owns how the model is
  launched, served and driven — a session's budget, its handoff, the sandbox it runs in, and
  the numbers each of those was settled by. Where the two meet, the driver runs a command
  and reads an exit code; it does not read a doc format, and it does not decide what work
  exists. **0016's `## Tasks` parser is on the wrong side of this line and predates it**,
  which is why retiring it is a `BACKLOG.md` entry rather than a rule nobody wrote down.

- **Not a public service.** The GB10 serves a trusted network and nothing else: no TLS,
  no per-user auth, no quotas, no billing. A shared token at most. Anyone on the network
  is a trusted user, and exposing the endpoint beyond it is out of scope.
- **Not a model zoo.** A model earns a place by scoring on the scorer; "worth a look"
  is a BACKLOG line, not a download.
- **No per-request cloud fallback.** A router that silently retries against a hosted
  model would make the local numbers meaningless. This is distinct from the
  coordination flow above, where the tiers are separate roles a human assigned.

## Constraints

- **Two envelopes, each fixed, never conflated.** The laptop is an M2 Max, 32 GB unified
  memory, 30 GPU cores, and every answer measured before 2026-09-13 is for that envelope.
  **Its exclusions stay permanent there** — Q5_K_M and Q6_K stay ruled out *by projection*
  on 32 GB; **MTPLX is excluded by measurement rather than by assumption**, its filled peak
  sitting 1.5 GiB above what that machine can cap at while leaving the system its reserve;
  Hermes stays unattended-only. The second envelope is a GB10: 20 Arm cores, a Blackwell
  GPU, 128 GB of unified LPDDR5X, Linux, headless, CUDA. Its reported memory bandwidth is
  below the laptop's, so single-stream decode there is a hypothesis and not a promise; its
  wins, if they are wins, are prefill and room for several slots. A finding from one
  envelope is not evidence about the other — a projection the 32 GB machine could not check
  may be measured on 128 GB, and only then does the exclusion lift.
  **Measured numbers are recorded with the machine they came from**, and each machine's
  limits live in its own `config/machine*.json` rather than in code. No config may make a
  Mac unusable for the editor and browser its developer is running while the agent works;
  the GB10 and a Mac kept only to serve are headless, and the rule does not reach them.
- **The job is to find which constraint binds, not to assume one.** Memory, ingest time,
  quality, and whatever else emerges are candidates, and which one binds depends on the
  envelope — model, quant, context, and the machine. A constraint asserted in advance is
  what sends a sweep looking in the wrong place, which has already happened once here.
- **What has been measured so far, and its exact scope.** In the envelope
  *Qwen3.8-27B · Q4_K_M · contexts to 64k · 32 GB*, **time bound before memory did** — as
  memory was being measured at the time. The full ladder ran with swap delta 0.0 MB at peak
  resident 17.91-20.27 GB while ingest rose to 13.1 minutes at 64k, the prompt rate
  decaying from 109 to 75 tok/s. Then 0014 asked the machine instead of the model and found
  a ceiling the first instrument could not see, above. **Neither result rules memory out
  generally.** Only one quant was tested, and larger models and longer contexts are
  untested. The finding is about this envelope, not about the constraint.
- **The headroom this once claimed was an artefact of the wrong metric.** "A third of the
  machine unused" came from summing per-process RSS, which counts every shared page once
  per resident process. Against anonymous-plus-wired — what actually competes for the
  32 GB — the model wires 20.89 GB and an editor takes 6.61, leaving a margin a browser
  does not fit in. That is why the quant ladder was dropped rather than run: Q5_K_M and
  Q6_K are excluded here by projection, and the projection is only testable on a machine
  with more memory.
- **The optimum is expected to differ by task, not just by profile.** A short tool-calling
  turn, a long refactor and a repo-wide search have different context and latency needs, so
  "one winning config" is an assumption the results have to earn rather than a goal.
- **Two properties, never conflated.** *Inference is local* — no prompt or file content
  reaches a model on hardware the developer does not control. A model served on the
  developer's second Mac or the team's GB10 keeps this property; the network between them
  is trusted and the traffic is plain HTTP, which is a decision and not an oversight. *The
  harness is offline* — it needs no vendor reachability at all. The first is required everywhere; the second is stronger, and every
  setup states which of the two it has. `ANTHROPIC_BASE_URL` alone buys only the first,
  because OAuth refresh and feature-flag fetches do not follow it. **Measured since: all
  four harnesses have the second, the incumbent included** — under token authentication and
  with nonessential traffic disabled, Claude Code completes a task with the network denied
  in the kernel. That claim is scoped to a configuration and to token auth, and it stays
  scoped. What is ruled out entirely is an editor assistant that proxies prompts through
  its vendor, which fails even the first. The one deliberate exception is the coordination
  flow, which sends planning context to a frontier model and may never be silent.
- **Go for anything built.** One static binary, no runtime competing with the model for
  the memory it needs, and one that cross-compiles to Linux on Arm without a second
  toolchain. Where a tool is Python-only — MLX above all — it is confined to
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
