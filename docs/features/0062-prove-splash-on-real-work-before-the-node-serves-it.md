---
id: 0062
title: Prove Splash on real work before the node serves it
status: Draft
created: 2026-09-20
submitted: 
needs:
---

## Problem

0060 measured Splash at 2.8 to 3.8 times llama.cpp's decode and a chain in under half the
wall, and none of it on the unit VISION judges by. All nine chains were the generated
fixture and finished in one session, so no handoff, no session deep enough for a side call
to evict the conversation's prefix, and no run longer than five minutes has gone through
it. It left 0.25 GB of the node free, read with a console session logged in. Serving it on
that evidence would put the owner's working day on a runtime nobody has worked a day on.

## Non-goals

- Serving Splash from the daemon, or reaching it from the laptop without a tunnel typed
  by hand. 0063, which needs this feature's verdict.
- A sampling sweep. Splash refuses `presence_penalty`, and nothing here has measured that
  the penalty prevents anything: it is the model card's pair, and 0060's llama.cpp sides
  scored the same at 0 as at 1.5. Whether its absence shows is read off the long chain. A
  `repetition_penalty`, which Splash accepts, is tried only if that chain repeats itself.
- The laptop, the GB10, Splash's second model, and concurrency.
- Naming the real-work repository. It is private; rows carry counts and no path.

## Design

Every row is taken with nobody logged in on the node, which is the state 0056's floor and
record were measured in and 0060 could not use. The USB link needed a session on the node
after every reboot; the Thunderbolt bridge that replaced it on 2026-09-19 has static
addresses from `networksetup` and is expected not to. That is checked first, by a reboot
to the login window, because it decides the state of everything after it. If the bridge
does need a session, the rows are taken logged in, labelled so, and the memory box says
what that costs.

Splash rows cannot be checked against their labels today: `served_n_ctx` is 0 and
`served_model` is `.`. `internal/eval/client.go` and `cmd/prefixprobe` read the served
context, the model id and the build id from `/status` and `/v1/models` where `/props`
answers 404, as `cmd/localcode/main.go` does since 0060, and only on a 404. The rows gain
no new field; the existing ones stop being empty. This comes before the measurements so
they are the first rows of this runtime that say what served them.

The memory margin is chosen, not inherited. `--max-memory` equal to the GPU cap left
0.25 GB free. Splash is screened filled at 49,152 by `scripts/screen.sh` at the cap and at
two lower ceilings, and `runtimes/splash/config/splash-27b.env` takes the largest one that
leaves at least 1 GB free through the window with nothing swapped. Decode is read at each,
because a ceiling that slows it is a price and the table has to show it.

What a side call costs a conversation is `cmd/prefixprobe`'s question, asked of both
runtimes with and without `-interleave`. 0060's depth suite showed each prefix cache
missing where the other hit, on interleaved near-identical prompts, which is not a
session; this is the instrument 0018 built for the session case.

The verdict is one instruction on the real-work repository, driven on both runtimes from
one starting commit, as 0040 drove two harnesses. It is driven from the laptop, where that
repository lives: llama.cpp through the node's endpoint, and Splash through
`ssh -L 8000:127.0.0.1:8000` to the node, since Splash binds loopback only and the
launcher treats a loopback endpoint as local. `MAX_THINKING_TOKENS=0` is set for both
sides; llama.cpp ignores it. The instruction is large enough that the llama.cpp side
takes several sessions, so handoffs happen on both. Each side is read by `localcode
account -jsonl` and judged by the repository, per 0026: tests passing and commits made,
then sessions, handoffs written and inherited, why each session ended (0029), wall,
tokens ingested, reused and generated. A session that ends on its timeout or its call
budget, or whose transcript repeats one call until it does, is recorded as a loop. The
published rows are aggregates without the repository's name or paths.

Splash passes if its chain finishes what llama.cpp's finishes, every handoff is inherited,
no session loops, nothing swaps, and its wall is lower. Anything else is recorded as the
reason 0063 is dropped or revised. TECH's sentence that the penalty "stops a non-thinking
model looping" is corrected in the verdict box either way: it is the model card's
recommendation and no row here supports the claim.

Changed: `internal/eval/client.go`, `cmd/prefixprobe/main.go`, their tests,
`runtimes/splash/config/splash-27b.env`, `docs/TECH.md`, `docs/data/`.

## Tasks

- [x] The node is rebooted to the login window and reached over the Thunderbolt bridge with nobody logged in, and TECH records whether the bridge needs a session, which fixes the condition every later row carries
- [x] `internal/eval/client.go` and `cmd/prefixprobe` read the served context, model and build from `/status` and `/v1/models` only where `/props` answers 404, covered by tests, so a Splash row names what served it
- [x] Splash is screened filled at 49,152 at the cap and two lower `--max-memory` ceilings with decode read at each, and TECH records where the node's memory sits under it and why the ceiling stays at the cap
- [x] `cmd/prefixprobe` runs against both runtimes with and without `-interleave`, and TECH records what a side call costs a conversation on each
- [x] One real-work instruction is driven on both runtimes from one starting commit, Splash through an SSH forward, and the account rows are published as aggregates carrying no name or path
- [x] TECH carries the verdict against the five pass conditions with the rows cited, says whether 0063 proceeds, is dropped or is revised, and corrects the sentence about what `presence_penalty` is for
- [x] The external review's findings are closed: `prefixrun.sh` ends the server it started by pid on every way out, the per-session counters and memory samples behind TECH's claims are published, and the verdict no longer clears 0063 on a comparison that was not made
- [x] TECH says where the 24,576-token ceiling comes from: the launcher's default at a served 49,152, with the sessions' overshoot of it read from the published rows

## Log

- 2026-09-20: the scorer also asks `/v1/messages` by the served model's id. The plan has it
  read what is served; it did not know the Messages dialect sends the name `local`, which
  llama.cpp ignores and Splash answers 404 to, so `cmd/prefixprobe` could not have run
  against Splash at all. `Props` keeps the id on the client when it comes from `/v1/models`.
- 2026-09-20: the memory box no longer picks a ceiling by free memory, because the criterion
  was wrong. Design had the config take the largest `--max-memory` leaving 1 GB free. Three
  ceilings read the same: about 20 GB peak wired, under 0.2 GB of free pages, nothing swapped
  or compressed, decode unchanged. `--max-memory` does not bind at this context, and free
  pages are low because the kernel keeps the 17 GB weights file it read as file cache, which
  it reclaims on demand. The config keeps the cap, and the box records the breakdown.
- 2026-09-20: `scripts/prefixrun.sh` takes a serve command. The plan runs the probe against
  both runtimes and the script that walks its conditions started `scripts/serve.sh` only.
- 2026-09-20: the real-work pair forks an interrupted chain the owner named and restores the
  scratch scripts that chain left in `/tmp` before each arm. The plan has both arms start
  from one commit; the work also lived in a handoff and in files outside the folder, and an
  arm that inherited the other's edits to them would not have started where it did. The
  original folder's checksum is unchanged after both arms.
- 2026-09-19 — reopen to Draft: TECH says the 24,576 ceiling came from the forked chain's settings; it is the launcher's default for a 49,152 window
