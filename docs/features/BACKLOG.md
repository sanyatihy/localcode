# Backlog

Ideas that are not features yet. Nothing here is scheduled or prioritised — the
default state is **stays frozen**.

Brainstorm output lands here. Promote one to its own `NNNN-*.md` with `kit new`
only on a real signal: someone asked, a metric moved, something else needs it.

Keep each entry to a few lines: what it is, and what would make it worth doing.

An entry earns its place by being consulted when its trigger fires. A decision already
taken belongs in `docs/TECH.md`, which is where anyone would look for it; an idea whose
trigger nobody watches is not an idea, it is a hedge.

- **The attended desktop verdict for the MTP config** — 0017 adopted it for the grind
  profile and measured it `unattended` only. It peaks at 22.10 GB where 0014's desktop died
  at 22.29, so the margin is 0.19 GB and the answer is not obvious. One 90-second screen with
  somebody driving the machine settles it. Promote when: anyone wants to run the fast config
  and use the Mac at the same time — which is the likeliest trigger in this file, since the
  developer owns the machine.
- **The host-RAM prompt cache, against the memory ceiling** — llama-server keeps prefixes it
  has evicted from a slot in host RAM and restores them, which is what makes reuse survive
  traffic beside a conversation (0018). The budget is `--cache-ram`, 8192 MiB by default, and
  it is bought from the same 32 GB the weights and the KV reservation sit in. 0041 screened a
  load and one prompt with the cache on and off and found them 12 MB apart, so **the budget is
  a ceiling filled lazily and a ladder cell is not where it binds**; what is still unmeasured
  is a long session that has actually saved prefixes into it. Promote when: a session at depth
  is seen swapping with the model's own footprint unchanged.
- **Restricting the editor's tool set through the `agent` setting** — the extension spends
  36,309 tokens of preamble before anything is typed and has no equivalent of `--tools`, which
  costs 462 s a turn against 45 s in a terminal (0008). The `agent` setting is documented to
  run the main thread as a named subagent and apply *its* tool restrictions, which would be
  `--tools` by another route. Untested, and cheap to test. Promote when: the editor flow is
  something somebody wants to use daily rather than prove works.
- **`--tools` is variadic, so the instruction can be swallowed** — `internal/harness/claudecode.go`
  passes `--tools <list> --permission-mode <mode> <instruction>`, and works only because
  `--permission-mode` terminates the tool list. Drop that flag and the instruction joins the
  tools, the run dies asking for input, and nothing says why. Promote when: that call site is
  edited for any other reason — a `--` separator or the instruction on stdin costs one line.
  **Its real home is a comment at the call site**, since that is where it fires; it sits here
  until someone puts it there.
- **`serverUp`'s error text reaches nobody** — it composes a message naming the command that
  fixes a dead endpoint, and both callers in `cmd/localcode/main.go` test it with `== nil` and
  throw it away. What a user actually sees is `no server at <url>, and -no-serve was given`, or
  a twenty-minute wait. Two call sites, one line each. Promote when: someone hits a dead
  endpoint and cannot tell from the output what to start — or when that file is open anyway.
- **A controlled chain on real source at both ceilings** — 0034 settled the driver's default
  on two readings that disagree, and neither is the measurement wanted. The fixture pair is
  controlled but runs work whose calls cost a third of real ones, so the small ceiling never
  binds in it; the real-source pair binds every session but interleaved the two
  configurations across phases of one instruction, so its arms did different work. What
  would settle it is one instruction on a real repository run twice, once per ceiling, from
  the same starting commit — scored by what landed rather than by wall clock, since the two
  are within 3% per tool call. Promote when: the default is questioned again, or a repository
  and an instruction worth spending two full chains on are in hand.
- **Spill a tool result to a file instead of dropping it** — Pi caps tool output at 50 KB or
  2,000 lines; `localcode` sets `BASH_MAX_OUTPUT_LENGTH` and the excess is simply gone, so a
  session that needed it re-runs the command through a filter and pays a turn plus the
  command's own time. 0023 considered a spill and dropped it, reasoning that a `PostToolUse`
  hook runs once the result is already in the conversation and so can only add context. That
  is true of `PostToolUse` and not of `PreToolUse`, which is the hook the gate already uses
  to rewrite an input — `ClampRead` narrows a `Read` there today, and the `Verdict.Input`
  channel it returns through is not specific to `Read`. A `Bash` command rewritten to
  redirect into the session directory and return only its tail would leave the whole output
  where the session can grep it, and name the path so the loss is visible rather than
  silent. Against it: a targeted re-run costs one call and so does a grep, so the win is on
  slow commands and on the session knowing what it lost, not on tokens. Promote when: a
  chain is measured losing work to a truncated result, or `go test` output at a real
  repository's size is shown to exceed the cap that a session then cannot recover.

- **Flip the chain's default harness to Pi** — 0040 measured Pi cheaper than the incumbent
  on every per-session column across four chains, and TECH says to drive a chain with
  `-harness pi`, but `harness.DefaultAgent` is still `claude-code`. Promote when: more
  chains agree, or a reading exists of what the default costs one-shot patch work, which
  0010 says is the job the incumbent wins.
- **Move `AppendJSON` out of the scoring package** — `cmd/localcode` imports `internal/eval`
  for a durable JSONL append and nothing else, so the session supervisor depends on the
  scorer to write a line. Promote when: a third caller needs it, or something else forces
  the import graph open. Behaviour-neutral either way.
- **Declare `Agent` in the package that consumes it** — the interface sits in
  `internal/harness` beside its implementations; its only consumer is `cmd/localcode`.
  `Driver` gets this right, declared in `internal/eval`. The registry behind `NewAgent` is a
  real reason to keep the constructor where it is, and none to keep the interface there.
  Promote when: a second consumer appears, or the registry moves anyway.
- **Fold the two harness adapter families into one** — every harness is written twice:
  `ClaudeCode`/`claudeCodeAgent`, `Pi`/`piAgent`, once behind `Driver` for `cmd/tier2` and
  once behind `Agent` for `cmd/localcode`, 863 lines across six files. Looked at as debt it
  is obvious and it was examined and left: the two lifecycles genuinely differ — tier 2 runs
  a harness to completion in a scratch checkout, `localcode` runs a budgeted session with
  hooks — and tier 2 is the instrument VISION requires to keep reproducing its numbers, so
  the merge risks a published comparison for a gain that is aesthetic. Promote when: a third
  harness has to be written twice, or a bug is found in one copy and not the other.
- **Deny a read of a `.env` inside the repository being worked on** — 0044 denies the
  credential roots outside the working tree, and the commonest real secret on a developer's
  disk is inside it, where a path deny cannot reach without breaking the case where fixing
  that file is the task. Promote when: a session is seen reading one it had no reason to,
  or the sandbox gains a rule that can express "read once, never write onward".
- **Split `cmd/localcode/main.go`** — 787 lines carrying flag parsing, subcommand dispatch,
  the server lifecycle, seatbelt profile generation, the state-directory layout and the hook
  entry point. Examined and left: the split is a pure move with no behaviour change, it
  costs every `git blame` on the file, and no feature so far has had to edit three of those
  concerns at once. Promote when: one does, or the file grows a seventh concern.
- **Structured logging, which this project should not adopt** — the Go checklist asks for
  `log/slog` with key-value pairs and this repo uses `fmt.Fprintf` to stderr throughout.
  That is correct here and the checklist's own CLI section says why: stdout is data, stderr
  is diagnostics, and the diagnostics are sentences a person reads while a chain runs.
  Recorded so it is not "fixed" later. Promote when: something other than a person consumes
  the driver's stderr.
- **A coverage floor in the gate** — `cmd/eval` and `cmd/tier2` sit at 26% and 24% against
  70-91% elsewhere. Both are flag wiring around a loop that needs a live server and a model,
  so a floor would buy tests written to satisfy a number. Promote when: a bug is found in
  the part of either that is not covered, which is the signal a floor is standing in for.
- **Pin the workflow's actions to commit SHAs** — `actions/checkout@v5`, `setup-go@v6` and
  `golangci-lint-action@v9` are the only third-party code this repo runs, and a major-version
  tag can be moved under it. Left out of 0050 on blast radius rather than on principle: the
  job grants `contents: read`, carries no secret and publishes nothing, so a moved tag reaches
  a public checkout and a lint run. Promote when: the workflow gains a secret, a write
  permission, or a publishing step — any one of those turns this from tidy into necessary.
- **Retire `runtimes/mtplx`** — 165 lines and three configs for a runtime VISION calls
  permanently excluded by measurement, its filled peak 1.5 GiB above what this machine can
  cap at. Its two shell scripts are gated by shellcheck on every `make check`, so it is not
  free. Against cutting it: the same reasoning that keeps the harness adapters — the numbers
  behind a published exclusion should stay re-takeable, and `harness/README.md` freezes
  versions for exactly that reason. Unresolved, and the developer's call. Promote when: the
  exclusion is questioned, or a Python dependency in it stops resolving.
