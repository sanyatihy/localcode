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
   another feature, or a chore commit that rides on a pull request under a branch name
   carrying no id. A separate doc earns its place when it holds a decision someone must read
   before building, that is not recoverable from the code or the commit. A change nobody
   could review in one sitting is too big — split it by value, not by layer.
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

### Taking work

8. **Take a feature with `kit claim <id>`.** It pushes the branch that claims it. Read the
   exit code:
   - **0** — the feature is yours. Work it.
   - **1** — it is not yours. Run `kit next` and take a different feature. Do not retry.
   - **2** — the command could not run. Read the error, fix it, claim again.
9. **A feature `kit next` lists as `taken` is not yours.** Do not work it, do not tick a
   box in it, do not help with it. Two agents in one feature produce one branch nobody
   can review.
10. **Work one feature per checkout, in the worktree `kit claim` prints.** If `kit next`
    lists features under **free, but not in this checkout**, do not start them here.

### Doing the work

11. **Read the feature doc for the design, and nowhere else.** It is the one home for it.
12. **Do the topmost unticked `## Tasks` box, and only that one.** List order is the order
    of work.
13. **Tick the box in the commit that implements it.** One box, one commit. Nobody re-reads
    the branch to check, so the commit is what has to be honest.
14. **Discovered work becomes a new box, a new doc, or a `BACKLOG.md` line — never a `TODO`
    in the code.** Append it, unless it blocks the boxes below.
15. **A box that turns out wrong may be rewritten, split or deleted.** Own commit, reason in
    `## Log`. Never silently drop one you could not finish. A whole feature that should not
    be built is `kit drop <id> "<why>"`.
16. **Stopping mid-feature is `kit release <id>`.** It refuses while the branch carries
    work: that is a pause, so push the branch and open the pull request unfinished instead.
17. **Never put a real secret in a doc.** Name the variable and where the value lives, never
    the value. A pushed commit cannot be unpublished.
18. **Never edit a doc whose status is `Shipped`.** It is frozen history.

### Finishing

19. **When the last box is ticked, run this project's own gate, then `kit audit --strict`,
    then `kit ship <id>`.** In that order: `ship` freezes the doc, so anything the other
    two would have made you change has to be found before it.
20. **Push the branch, open the pull request, and stop. You never merge your own work.**
    Opening a pull request is not approving it. With no remote, say so and leave the work
    on the branch.

### Where two agents can still collide

**Run `kit audit` after a merge that brings in docs drafted with no remote.** `kit new`
reserves an id by pushing `refs/kit/ids/<id>`, so two agents drafting off one remote are
never handed the same number; with no remote there is nothing to reserve against and both
can write `0009`. They merge cleanly, so `audit` reports `duplicate-id` at HIGH. Fix it
before claiming either: give one doc a free id, rename its file, and rename its branch.

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

### Where the rest lives

These rules are the judgement no refusal can make. Everything else the kit knows, it
prints: `kit help checks` for what `audit` reports, `kit help template` for the doc format,
`kit help backlog` for what a deferred entry looks like, and **`kit help go-checklist` for
the engineering practice a Go project is held to** — read that before writing code, not
after.
