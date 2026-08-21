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
- ~~**Speculative decoding with a draft model**~~ — **settled by 0017**, and against it: an
  external drafter is 1.1 GB this machine does not have, and the target's own MTP head costs
  nothing and gives 1.26-1.57x. A separate draft model is the wrong shape here.
- **The attended desktop verdict for the MTP config** — 0017 measured it `unattended` only,
  and it peaks at 22.10 GB where 0014's desktop died at 22.29. One 90-second screen with
  somebody driving the machine settles whether it is usable at 32k while you work. Promote
  when: anyone wants to run the fast config and use the Mac at the same time.
- **Qwen3-Coder-Next (80B-A3B)** — MoE built for agentic coding, 3B active. Promote when:
  someone confirms a quant fits 32 GB with usable context; at ~45 GB for Q4 it currently
  does not, and the vision rules out a model zoo.
- **The quant ladder, on a bigger machine** — 0004 was dropped because the admissible wired
  budget on 32 GB is ~22.2 GB and Q4_K_M already sits at 21.75, so Q5_K_M and Q6_K are
  excluded *by projection rather than by measurement*. Promote when: the 128 GB machine
  arrives, which moves that budget and makes the exclusion testable instead of assumed.
- **Quantising the model ourselves** — build custom quants rather than taking bartowski's.
  Promote when: a quant ladder actually runs — which needs the bigger machine, see above —
  and shows a gap in the published quants worth filling.
- **Hermes' persistent memory over a real week** — 0010 scores both harnesses cold,
  which is the only reproducible comparison but deliberately blind to Hermes' main bet.
  Promote when: a harness is in daily use and the question is whether it improved on
  this repo specifically.
- **The host-RAM prompt cache, against the memory ceiling** — llama-server keeps prefixes
  it has evicted from a slot in host RAM and restores them, which is what makes reuse
  survive traffic beside the conversation (0018). The budget is `--cache-ram`, 8192 MiB by
  default and unset in every config here, and it is bought from the same 32 GB the weights
  and the KV reservation sit in. 0014's attended ceiling was walked without accounting for
  it. Promote when: the ceiling is re-walked, or a session at depth is seen swapping with
  the model's own footprint unchanged.
- **Cursor's built-in assistant via an HTTPS tunnel** — rejected in 0008 because it
  routes through Cursor's backend, so code leaves the machine. Promote when: Cursor
  supports calling a base URL from the client, which would remove the entire objection.
- **Restricting the editor's tool set through the `agent` setting** — the extension spends
  36,309 tokens of preamble before anything is typed and has no equivalent of `--tools`,
  which costs 462 s a turn against 45 s in a terminal (0008). The `agent` setting is
  documented to run the main thread as a named subagent and apply *its* tool restrictions,
  which would be `--tools` by another route. Untested. Promote when: the editor flow is
  something somebody wants to use daily rather than prove works.
- **Harder tier-1 tasks that this model actually fails** — 0013 added seven traps and
  Qwen3.8-27B took only one of them: eleven of fourteen tasks score 3/3 at every setting
  measured. The traps are proven to trap (fixture self-tests fail the tempting answers), so
  this is about the model being good at these shapes rather than the fixtures being soft.
  Promote when: a comparison actually needs tier-1 to *rank* rather than to floor-check —
  a quant sweep or a model comparison that cannot separate two candidates would do it, and
  both wait on the bigger machine. Deliberately not scheduled: chasing traps a capable model
  will fail is open-ended, and 0010 ranks harnesses on tokens and turns instead.
- **One pass for floor-check tasks, three for the ones that move** — the twelve flat tasks are
  3/3 at every setting, so their repeats measure nothing and cost most of a sweep's runtime.
  Promote when: sweep runtime is what blocks a feature. No sweep run so far has been large
  enough for it to; a quant grid on the bigger machine would be the first.
- **`--tools` is variadic, so the instruction can be swallowed** — the claude-code adapter
  passes `--tools Read,Edit,Write --permission-mode acceptEdits <instruction>`, and it works
  only because `--permission-mode` terminates the tool list. Drop that flag and the
  instruction joins the tools, the run dies asking for input, and nothing says why. Promote
  when: the adapter is edited for any other reason — a `--` separator or the instruction on
  stdin costs one line. Found driving the same CLI by hand for 0011.
- **The handoff, reconciled with `kit`** — 0016 puts `HANDOFF.md`, three hooks and a box
  parser in localcode, but `kit` owns the work protocol: it writes AGENTS.md, and
  `kit next --json` already publishes the topmost box the driver re-derives. Handing off
  inside a box is the same category of rule as doing the topmost one. Promote when: a second
  project wants it, or the box parser drifts from kit's format. The unresolved part is
  whether kit should ship one vendor's shell hooks at all, or only the rule.
- **The handoff, on a harness that is not Claude Code** — the mechanism is a bounded session
  plus a file, which needs no hooks; only the wiring is Claude Code's, and 0010 kept it as
  the incumbent. Promote when: a harness is scored that compacts or truncates rather than
  stopping, or 0011 finds the window binds on something else. Note Pi ingests a fifth as much
  per task, so the pressure this relieves may be the incumbent's own.
- **The `mlx.fast` challenge harness, as a scorer** — Layr-Labs' Qwen3.8-27B ranked harness pins
  checkpoint, MTP head and prompts by SHA-256, pairs candidate against serial decode in one
  session, and gates on thermals and token fidelity. 0017 imports the gates; the harness itself
  wants ~36 GiB and an M5 Max runner. Promote when: a machine can run it, or the suite needs a
  hidden prompt pool it cannot overfit.
- **antirez's `ds4`, as a method rather than an engine** — a pure-C Metal runtime whose kernels
  are byte-validated against the reference forward, running a 284B MoE at 26.68 tok/s on a
  128 GB M3 Max. Not runnable here and a different model. Promote when: a lossless claim needs
  proving at the kernel level rather than at the token level, which 0017's fidelity gate does
  more cheaply.
