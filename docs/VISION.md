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

- **Not multi-user.** One machine, one developer. Auth, quotas and LAN serving would
  change every design here.
- **Not a model zoo.** A model earns a place by scoring on the scorer; "worth a look"
  is a BACKLOG line, not a download.
- **No per-request cloud fallback.** A router that silently retries against a hosted
  model would make the local numbers meaningless. This is distinct from the
  coordination flow above, where the tiers are separate roles a human assigned.

## Constraints

- **Hardware is fixed: M2 Max, 32 GB unified memory, 30 GPU cores.** A larger machine was
  considered and decided against, so this is the envelope every answer here is for. **Some
  exclusions are therefore permanent rather than provisional** — Q5_K_M and Q6_K stay ruled
  out *by projection*, and no run will ever test that; **MTPLX is excluded by measurement
  rather than by assumption**, its filled peak sitting 1.5 GiB above what this machine can cap
  at while leaving the system its reserve; Hermes stays unattended-only. Where a conclusion rests on a projection this machine cannot check, it says
  so and stays that way.
  **Measured numbers are still recorded with the machine they came from**, and this machine's
  limits still live in `config/machine.json` rather than in code — for the second audience
  rather than a second machine, since anyone reproducing these choices is on different
  silicon. No config may make the machine unusable for the editor and browser the developer
  is running while the agent works.
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
  reaches a model this machine does not run. *The harness is offline* — it needs no vendor
  reachability at all. The first is required everywhere; the second is stronger, and every
  setup states which of the two it has. `ANTHROPIC_BASE_URL` alone buys only the first,
  because OAuth refresh and feature-flag fetches do not follow it. **Measured since: all
  four harnesses have the second, the incumbent included** — under token authentication and
  with nonessential traffic disabled, Claude Code completes a task with the network denied
  in the kernel. That claim is scoped to a configuration and to token auth, and it stays
  scoped. What is ruled out entirely is an editor assistant that proxies prompts through
  its vendor, which fails even the first. The one deliberate exception is the coordination
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
