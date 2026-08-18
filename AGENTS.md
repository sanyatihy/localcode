## Work protocol

Run `kit next` at the start of every session: it names the feature and the task. `kit audit`
says what has drifted. `kit help` explains the model; `kit help template` prints the doc
format. Every rule below is a rule some agent got wrong; none is style.

### Picking and finishing work

1. **Asked to plan? Read `docs/VISION.md`, then draft the whole set** — not the first one,
   and without waiting to be asked to decompose. A feature that cannot be traced to the
   vision is the wrong feature, and saying so is cheaper than building it. Split by
   `## What done looks like`: each observable outcome is at least one feature.
   - **A feature is one pull request**, so it is the unit of review. Too big is one nobody
     can review in a sitting — config, storage, API and CLI together is a project. Too small
     changes nothing observable; that is a task box.
   - **A feature records a decision.** A change whose whole story fits in its commit message
     is a `BACKLOG.md` line and a commit, not six sections padded to fill.
   - **Fill `needs:` on every draft.** It is what lets `next` refuse work whose foundation
     is missing, and empty ones are what let agents run at once. Judge by whether the work
     could *start* now: a shared composition root, a shared router, or numbered migrations
     make a feature dependent however separate it looks.
   - **One drafting session is one plan round:** all its docs on one `plan/<something>`
     branch, merged once. The name must not mention a feature id anywhere in it — any
     branch citing one is a claim, so a draft on one claims itself.
   - **The round must reach the default branch before anything on it can be claimed.**
     `claim` refuses otherwise; land it at whatever statuses its docs reached.
   - **Leave every doc `Draft`, and never edit `status:` by hand** — `accept`, `ship` and
     `drop` own it, and there is no status for work in flight: the claim branch says that.
     `kit accept <id>` is the human's, and blesses a design rather than unlocking it.
   - **Unsettled things go in `## Open questions`**; what only a human can answer goes to
     `docs/INBOX.md` via `kit block`, the one thing that still stops a claim.
2. **One agent per feature. Take it with `kit claim <id>`.** It pushes the branch named after
   the doc, and creating that ref on the remote is what decides — and is the whole record
   that the work started; there is nothing to set in the doc.
   - **0** yours. **1** not yours — somebody was first, or the board would not have offered
     it (unmet `needs:`, a BLOCKING question, a doc nobody wrote, an id that disagrees with
     its filename); run `kit next` and take something else, losing is normal. **2** it could
     not run, a plan that has not landed among the reasons.
   - **Never create the claim branch by hand.** A branch only you can see is not a claim, and
     a push reporting "up to date" found somebody else's.
   - **A feature `kit next` lists as taken is not yours** — not to work on, not to tick a box
     in, not to help with because it looked slow. Two agents in one feature produce one
     branch nobody can review.
   - **If nothing is free, stop.** Claimed or waiting is a real answer. An idle agent costs
     nothing; two in one feature costs the feature.
3. **One feature per checkout.** Work in the worktree `kit claim` prints. The kit names no
   starting point in a checkout that already holds one — what is free is still listed, under
   **free, but not in this checkout**. A second feature started where you stand shares one
   branch with the first and they ship as one.
4. **Read the feature doc for the design.** Never restate or re-derive design anywhere else.
5. **Do the topmost unticked `## Tasks` box, and only that one.** List order is the order of
   work. Tick it in the commit that implements it — nobody re-reads the branch to check, so
   the commit is the unit that has to be honest.
6. **Discovered work becomes a box, a new doc (`kit new "<title>"`), or a BACKLOG.md line** —
   never a `TODO` in the code. Append it unless it genuinely blocks the boxes below: list
   order is priority, so appending is what says "later".
7. **A box that turns out wrong may be rewritten, split or deleted** — own commit, reason in
   `## Log`. What you may not do is silently drop one you could not finish. A whole feature
   that should not be built is `kit drop <id> "<why>"`: a doc on the default branch stays on
   the board until it is retired.
8. **Stopping mid-feature? Release the claim.** Delete the branch locally *and* on the remote
   (`git push origin --delete <branch>`), and say why in `## Log`. The pushed branch is the
   claim, so deleting only your copy leaves the feature taken for good.
9. **Blocked on a decision only a human can make?** `kit block <id> "<question, with your
   leaning>"`, then move to another feature — `--fyi` for what work continues without. Do not
   guess, and do not treat this as licence to take work that is not yours.
10. **Never put a real secret in a doc.** Docs are committed and a pushed commit cannot be
    unpublished. Name the variable and where the value lives, never the value.
    `secret-in-doc` is a backstop, not the rule: a password in prose matches nothing.
11. **Never edit a doc with `status: Shipped`.** It is frozen history.
12. **When the last box is ticked: `make check`, `kit audit --strict`, then `kit ship <id>`.**
    Those two are the gate — the code holds, the docs match the repo. `ship` refuses past it
    too, on an unmet `needs:` or a claim another checkout holds; neither is a malfunction.
    Then move durable facts to `docs/TECH.md`.
13. **Shipping is a handoff: you never merge your own work.** Push the branch, open the pull
    request if the forge has them — opening is not approving — and stop.
    - **Three roles, one to an agent per feature:** a **drafter** plans the round, a
      **builder** claims a feature and ships it, a **coordinator** merges what somebody else
      built. The trunk is the barrier: a plan lands before it can be claimed, and a claim
      lands only through somebody who did not build it.
    - **`kit next` lists what waits under `awaiting review`**, marking what you built and
      what a human must see — `review: human`, written by whoever accepts the round
      (`kit accept --review`). Honour that mark; never write it.
    - All of this is legibility, resting on agents having git identities of their own and on
      branch protection to enforce it; `docs/TECH.md` says why. With no remote there is
      nowhere to hand off to: say so and leave the work on the branch.

### Where two agents can still collide

The claim is airtight and the ID is not: `kit new` reads the next free ID off disk and never
asks the network, so two sessions drafting off one trunk both produce `0009` and the differing
filenames merge cleanly. `audit` reports `duplicate-id` at HIGH and `claim` refuses a shared
ID — one branch would claim both features. Fix it before claiming either (free ID, rename the
file, rename its branch), and run `kit audit` after any merge bringing in docs from elsewhere.

### Doing the work

These govern *how*. Bias toward caution on anything non-trivial; use judgement on the rest.

- **Think before coding.** State assumptions out loud; name the readings of an ambiguous
  request rather than silently picking one. Push back when a simpler approach exists, and
  stop when confused rather than guessing past it.
- **Simplicity first.** The minimum code that solves the problem: nothing speculative, no
  features beyond the box you are on, no abstraction for a single use.
- **Surgical changes.** Touch only what the task requires: no improving adjacent code, no
  refactoring what is not broken, match the style already there. Clean up your own mess and
  nobody else's.
- **Define success, then loop.** Say what "done" looks like before starting and verify it
  yourself rather than reporting completion and hoping. A box whose truth you cannot check
  was too big.
- **Enforce, do not instruct.** Before adding a rule here, ask what would remove the need for
  it — a refusal in `kit`, or protection on the remote. Prose is the weakest tier and is
  honest only for judgement and taste.
- **Write about the system, not the session.** A sentence about *the work* — which session it
  happened in, what the previous attempt did, who got a rule wrong — stops being true the
  moment it is fixed. State what holds; the incident belongs in the pull request, and
  anything durable in `docs/TECH.md`.
- **Write the record while you work, and only what the code cannot say.** A ticked box in
  the implementing commit and a `## Log` line for what the doing taught cost nothing and are
  accurate; an essay written in advance is expensive and often wrong by the end. Cut
  restatement and hedging — never a decision, a constraint, or the evidence behind it.
