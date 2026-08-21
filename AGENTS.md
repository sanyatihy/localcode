## Work protocol

Run `kit next` at the start of every session. It names the feature and the one task to do.
`kit audit` says what has drifted. `kit help` explains the model, and `kit help template`
prints the doc format.

### Planning

Do this only when you are asked to plan.

1. **Read `docs/VISION.md` first.** A feature that cannot be traced to it is the wrong
   feature. Split by `## What done looks like`: each observable outcome is at least one
   feature.
2. **A feature is one shipped increase in what the project can do, or can be trusted to
   do.** Name it in one sentence an operator would recognise: "the board no longer hands one
   feature to two agents", not "refactor `Mine`". No such sentence means it is a task box in
   another feature, or a chore commit. A sentence that needs an "and" is two features. A
   change nobody could review in one sitting is too big — split it by value, not by layer.
3. **Draft the whole set in one session, not the first one only.** Write every doc with
   `kit new "<title>"`. Never copy the template by hand.
4. **Fill `needs:` on every draft.** List the feature ids that must ship before this one
   can start. Leave it empty when the work could start today — empty ones are what let
   agents run at once. `kit help template` says how to judge it.
5. **Put the whole round on one branch named `plan/<something>`, and merge it once.**
   The branch name must contain no four-digit feature id, anywhere in it. A branch name
   containing an id is a claim on that feature.
6. **Leave every doc `status: Draft`.** `kit ship` and `kit drop` are the only things that
   change a status. Never edit `status:` or `shipped:` by hand.
7. **Never invent a product decision on the human's behalf.** What you cannot settle goes in
   `## Open questions`; what only a human can answer goes to `kit block <id> "<question>"`.
8. **The round must reach the default branch before anything on it can be claimed.**
   `kit claim` refuses otherwise.

**Work that is not a feature** — a fix to something already shipped, a chore, a data
update — goes on a branch whose name contains no four-digit id, and needs no doc. Only a
feature gets one.

### Taking work

9. **Take a feature with `kit claim <id>`.** It pushes the branch that claims it. Read the
   exit code:
   - **0** — the feature is yours. Work it.
   - **1** — it is not yours. Run `kit next` and take a different feature. Do not retry.
   - **2** — the command could not run. Read the error, fix it, claim again.
10. **Never create a claim branch by hand.** A branch only you can see is not a claim.
11. **A feature `kit next` lists as `taken` is not yours.** Do not work it, do not tick a
    box in it, do not help with it. Two agents in one feature produce one branch nobody
    can review.
12. **Work one feature per checkout, in the worktree `kit claim` prints.** If `kit next`
    lists features under **free, but not in this checkout**, do not start them here.
13. **If nothing is free, stop and say so.** Claimed or waiting is a real answer.

### Doing the work

14. **Read the feature doc for the design, and nowhere else.** It is the one home for it.
15. **Do the topmost unticked `## Tasks` box, and only that one.** List order is the order
    of work.
16. **Tick the box in the commit that implements it.** One box, one commit. Nobody re-reads
    the branch to check, so the commit is what has to be honest.
17. **Discovered work becomes a new box, a new doc, or a `BACKLOG.md` line — never a `TODO`
    in the code.** Append it, unless it blocks the boxes below.
18. **A box that turns out wrong may be rewritten, split or deleted.** Own commit, reason in
    `## Log`. Never silently drop one you could not finish. A whole feature that should not
    be built is `kit drop <id> "<why>"`.
19. **Stopping mid-feature is `kit release <id>`.** It refuses while the branch carries
    work: that is a pause, so push the branch and open the pull request unfinished instead.
20. **Never put a real secret in a doc.** Name the variable and where the value lives, never
    the value. A pushed commit cannot be unpublished.
21. **Never edit a doc whose status is `Shipped`.** It is frozen history.

### Finishing

22. **When the last box is ticked, run these three in order:** `make check`,
    `kit audit --strict`, `kit ship <id>`.
23. **`kit ship` refuses for four reasons, and each one is expected:** a box still
    unticked, a file uncommitted or untracked, a `needs:` that has not shipped, or a claim
    another checkout holds. Fix the cause and run it again.
24. **Move durable facts to `docs/TECH.md`** — one line each, only what stays true after
    this ships.
25. **Push the branch, open the pull request, and stop. You never merge your own work.**
    Opening a pull request is not approving it. With no remote, say so and leave the work
    on the branch.
26. **Merging somebody else's finished work is real work.** `kit next` lists it under
    `awaiting review`.

### Where two agents can still collide

**Run `kit audit` after every merge that brings in docs from another branch.** `kit new`
reads the next free id from disk and never asks the network, so two agents drafting at once
can both write `0009`. The files differ, so they merge cleanly and `audit` reports
`duplicate-id` at HIGH. Fix it before claiming either: give one doc a free id, rename its
file, and rename its branch.

### How to write

- **Think before coding.** State your assumptions. Name the readings of an ambiguous
  request rather than picking one silently. Stop when confused rather than guessing.
- **Write the minimum code that solves the problem.** Nothing speculative, no abstraction
  for a single use, no feature beyond the box you are on.
- **Touch only what the task requires.** Do not improve adjacent code. Match the style
  already there.
- **Say what "done" looks like before you start, then verify it yourself.** A box whose
  truth you cannot check was too big.
- **Prefer a refusal in the tool to a rule in prose.** Before adding a rule here, ask what
  would make it unnecessary. Prose is the weakest tier and is honest only for judgement.
- **A sentence earns its place by deciding, constraining or measuring something.** One
  claim per sentence. Evidence gets one trailing clause, never two.
- **Write about the system, not the session.** State what holds, not what happened while
  you worked.
- **Every fact has one home.** True after this ships → `docs/TECH.md`, one line. Changed
  this doc's plan → one `## Log` entry naming the change. Neither → the pull request.
