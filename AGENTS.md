## Work protocol

Run `kit next` at the start of every session. It names the feature and the task.
Run `kit audit` when you want to know what has drifted. `kit help` explains the
whole model, and `kit help template` prints the feature doc format.

### Picking and finishing work

1. **Asked to plan something? Read `docs/VISION.md`, then draft — all of it.**
   A feature that cannot be traced to the vision is the wrong feature, and saying so
   is cheaper than building it. Bootstrapping means producing the **whole set**, not
   the first one; do not wait to be asked to decompose. **Split by `## What done
   looks like`** — each observable outcome there is at least one feature.

   **A feature is one pull request** (merge request on GitLab), so it is the unit of
   review. Too big is one nobody can review in one sitting: config, storage, the API
   and the CLI together is a project. Too small changes nothing anyone can observe —
   that is a task box, not a doc.

   **Say what each one waits on in `needs:`**, on every draft: it is what lets
   `kit next` refuse work whose foundation is missing, and several empty ones are what
   let agents run at once. Judge it by whether the work could *start* now — a shared
   composition root, a shared router, or numbered files in one sequence such as
   migrations all make a feature dependent however separate it looks.

   **A drafting session is one plan round: all its docs on one branch, merged once.**
   Name it `plan/<something>`, never after a feature — a branch leading with a
   four-digit ID *is* a claim, so a draft on one claims itself.

   **The plan must reach the default branch before anything can be claimed.** A claim
   pushes `HEAD`, so a doc only in the working tree does not travel with the branch an
   agent is sent to. Land the round first, at whatever statuses its docs reached.

   **Leave every doc `Draft`.** `status:` orders the board; it does not gate it. A
   human runs `kit accept <id>` to say they have blessed the design, which promotes the
   feature rather than unlocking it — agents work Drafts. Never run `accept` yourself
   and never edit `status:` by hand; that is what `accept`, `ship` and `drop` are for.

   **Put what you could not settle in `## Open questions`.** A later agent claiming the
   draft settles them before building. Anything only a human can answer goes to
   `docs/INBOX.md` with `kit block` — the one thing that still stops a claim.
2. **One agent per feature. Take it with `kit claim <id>`.** It pushes the branch named
   after the doc, and creating that ref on the remote is what decides. Exit 0 is yours.
   Exit 1 means not yours: somebody was first, or the board would not have offered it —
   an unmet `needs:`, a BLOCKING question, a doc nobody wrote, an id that disagrees with
   its filename. Either way run `kit next` and take something else; losing is normal.
   Exit 2 is the command unable to run, a plan that has not landed on the default branch
   among the reasons. Never create the claim branch by hand: a branch only you can see is
   not a claim, and a push reporting "up to date" found somebody else's.

   **A feature `kit next` lists as taken is not yours.** Not to work on, not to tick a
   box in, not to "help with" because it looked slow. Two agents in one feature produce
   one branch nobody can review.

   **If nothing is free, stop.** Everything claimed or waiting is a real answer — say
   so and wait. An idle agent costs nothing; two in one feature costs the feature.
3. **One feature per checkout. Work in the worktree `kit claim` prints.** Git provides
   the isolation; the kit refuses to name a starting point in a checkout that already
   holds one. It still lists what is free, under **free, but not in this checkout**, and
   points `kit claim` at a worktree of its own. Starting a second feature where you stand
   puts both in one branch and they ship as one.
4. **Read the feature doc for the design.** Never restate or re-derive design
   anywhere else.
5. **Do the topmost unticked `## Tasks` box, and only that one.** List order is the
   order of work. Tick it in the same commit that implements it — nobody re-reads the
   branch to check, so the commit is the unit that has to be honest. Among features of equal
   status, `kit next` goes by ID, lowest first.
6. **Discovered work** becomes a new `## Tasks` box, a new feature doc
   (`kit new "<title>"`), or a line in `docs/features/BACKLOG.md` if it is real but
   not now. Never a `TODO` in the code. Append it unless it genuinely blocks the
   boxes below it — list order is priority, so appending is what says "later".
7. **A box that turns out wrong may be rewritten, split, or deleted** — in its own
   commit, with the reason in `## Log`. The plan is a working document; only
   `status: Shipped` freezes it. What you may not do is silently drop a box you
   could not finish. A whole feature that should not be built is `kit drop <id>
   "<why>"` — docs live on the default branch for ever, so an abandoned one stays on
   the board until it is retired.
8. **Stopping mid-feature? Release the claim.** Delete the branch locally *and* on the
   remote (`git push origin --delete <branch>`), set the status back to what it was,
   and say why in `## Log`. The pushed branch is the claim, so deleting only your local
   copy leaves the feature taken for good.
9. **Blocked on a decision only a human can make?** Run
   `kit block <id> "<question, with your leaning>"`, then move to another feature. Do
   not guess. Use `--fyi` for anything work can continue without. That is not licence
   to take work that is not yours; see **One agent per feature**.
10. **Never put a real secret in a doc.** Docs and `docs/INBOX.md` are committed, and a
   pushed commit cannot be unpublished. Name the variable and where the value lives —
   never the value. `secret-in-doc` catches the shapes it can be certain about, which
   is a backstop and not the rule: a password in prose matches nothing and still leaks.
11. **Never edit a doc with `status: Shipped`.** It is frozen history.
12. **When the last box is ticked, run `make check` and `kit audit --strict`, then
    `kit ship <id>`.** Those two are the gate: green means the code holds and the docs
    and the repo agree. Shipping by hand is where `unticked-shipped` and
    `shipped-no-date` come from. `ship` refuses past that gate too — an unmet `needs:`,
    or a claim held by another checkout — and neither is a malfunction. Then move durable
    facts into `docs/TECH.md`.
13. **Shipping is a handoff: you never merge your own work.** Push the branch, open the
    pull request if the forge has them — opening is not approving — and stop. Three roles,
    one to an agent per feature: a **drafter** plans the round, a **builder** claims a
    feature and ships it, a **coordinator** merges what somebody else built. The trunk is
    the barrier between them — a plan lands before anything on it can be claimed, and a
    claim lands only through somebody who did not build it. `kit next` lists what waits
    under **awaiting review**, marking what you built and what a human must see:
    `review: human`, written by whoever accepts the round (`kit accept --review`) — honour
    it, never write it. All of that is legibility, resting on agents having git identities
    of their own and on branch protection doing the enforcing; `docs/TECH.md` says why.
    With no remote the work has nowhere to go — say so and leave it on the branch.

### Where two agents can still collide

The claim is airtight and the ID is not. `kit new` takes the next free ID from what
it can see on disk and never goes to the network, so two sessions drafting against
the same trunk both produce `0009`. A plan round in flight is invisible to the next
one, and a round collides a batch of IDs where a single draft collided one. Nothing
in git objects: the filenames differ, so the merge is clean.

`kit audit` reports it as `duplicate-id`, and it is HIGH because the ID is the
primary key — one branch would claim both features and one INBOX line would block
both. `kit claim` refuses an ID that two docs share for the same reason.

Fix it before claiming either: give one doc a free ID, rename its file to match, and
rename its branch if it already has one. Run `kit audit` after any merge that brought
in feature docs written elsewhere.

### Doing the work

The rules above decide *what* to work on. These govern *how*, and bias toward caution
over speed on anything non-trivial. Use judgement on trivial tasks.

- **Think before coding.** State assumptions out loud. Where a request is ambiguous,
  name the readings rather than silently picking one. Push back when a simpler
  approach exists, and stop when confused — say what is unclear instead of guessing
  past it. **Blocked on a decision only a human can make** is where a question goes
  when only a human can settle it.
- **Simplicity first.** The minimum code that solves the problem. Nothing
  speculative, no features beyond the box you are on, no abstraction for a single
  use. If a senior engineer would call it overcomplicated, it is.
- **Surgical changes.** Touch only what the task requires. Do not improve adjacent
  code, comments, or formatting, and do not refactor what is not broken — match the
  style already there. Clean up your own mess and nobody else's. Unrelated problems
  you spot are **Discovered work**'s business, not this change's.
- **Define success, then loop.** Say what "done" looks like before starting, and
  verify it yourself rather than reporting completion and hoping. A box you cannot
  check the truth of was too big; split it.
- **Enforce, do not instruct.** A rule in prose is one an agent can skim past; a refusal
  in the tool is one it has to route around deliberately. Before adding a rule here, ask
  what would remove the need for it — a refusal in `kit`, or protection on the remote.
  Prose is the last tier, honest only for judgement and taste; `docs/TECH.md` has why.
- **Write as much as necessary and no more.** Cut restatement, hedging and padding.
  Never cut a decision, a constraint, or the evidence behind it — terseness is not a
  virtue on its own. Both failures are real: a doc nobody reads spends the context
  window every later session needs, and one too thin to act on costs a session to
  reconstruct. Honour each section's stated budget; prefer a table or a list to prose.
