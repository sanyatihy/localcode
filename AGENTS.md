## Work protocol

Run `kit next` at the start of every session and after finishing or releasing work.
It names the feature and next task. `kit audit` reports drift; `kit help <command>`
explains a refusal. Fix its cause rather than bypassing it with raw git or hand edits.

### Planning

Plan when asked. For work that began without a plan, use the spike route below.

1. Read `docs/VISION.md` first. A feature is a verifiable improvement serving its
   outcomes or constraints: a capability added or removed, an exposure closed, a failure
   eliminated, or a measurable gain in performance, cost or maintainability. State who
   benefits and what observable result proves the improvement.
2. Prefer the largest coherent outcome a reviewer can safely assess in one pull request.
   Keep implementation layers, migration, tests and documentation as task boxes within
   that feature. Split when outcomes have independent value, need separate approval, or
   cannot be reviewed safely together. Do not split merely to give each agent a doc.
   A feature doc holds decisions worth approving before building; a small change with no
   such decision can be a task box or recorded direct work through the spike route.
   Removing an integrated feature is new work citing its frozen doc; `drop` only retires an unbuilt plan.
3. Start the round with `kit start --plan "<round>"`. For a new repo, it publishes an
   empty trunk base and preserves setup files for the first PR. Draft the whole round
   with `kit new "<title>"` on that `plan/<round>` branch whose
   name contains no four-digit id. Fill `needs:` with prerequisite feature ids; leave
   it empty when work can start independently. Keep every draft `status: Draft`.
4. Run `kit finish` to validate, commit and push the planning round; open one PR for
   human review. Planned features require the human's approval through that merge before implementation starts. After it
   merges, return to a trunk checkout, pull with --ff-only, and run kit audit and kit next.
5. Put uncertainty in `## Open questions`. Use `kit block <id> "<question>"` for a
   decision only the human can make. Never invent a product decision to unblock yourself.
   Once answered, record the decision in the affected plan, remove its blocking inbox
   line, and commit both. Integrate that answer into a paused checkout before resuming.

### Taking work

6. From an updated trunk checkout, take a feature with `kit start <id>` (the guarded
   claim route; `kit claim <id>` remains available). Read the exit code:
   - **0**: the claim is yours; work in the worktree the command names.
   - **1**: it is not yours; run `kit next` and choose different work.
   - **2**: the command could not run; fix the reported cause before retrying.
   A claimed branch or worktree belonging to another agent is not yours to clean up.
7. Each agent works in its own checkout. Do not share a working tree, index or claim.
   A branch citing a four-digit id is a claim; recorded direct work and spikes cite none.
8. For directly requested changes or exploration, use `kit start --work "<outcome>"`.
   It records intent on a spike branch of its own; small changes also use this route.
   Do not turn a pending plan into a spike to bypass approval. When done,
   `kit finish -- <gate argv>` harvests recorded work and names missing review facts.
   Existing spikes can use `kit harvest "<title>" --box "<outcome>"` to record the
   work and name its branch after the feature. Fill
   Problem and Design before submitting. With no commits, harvest creates an empty trunk
   base; publish it for the first PR. `kit harvest --nothing "<what it showed>"` records
   a discarded spike.

### Doing the work

9. Read the feature doc for its design and `docs/TECH.md` for the system it changes.
   Read `kit help go-checklist` or `kit help python-checklist` before writing code.
10. Do the first unticked Tasks box. Tick it in the commit implementing it, then continue
    to the next box until the feature is ready for review or blocked. One box, one commit;
    the subject names the outcome. Explain only what the diff and feature doc cannot.
11. State assumptions and an observable definition of done before coding; verify it.
    Change only what the task requires. Match the project's style and avoid speculative
    abstractions or adjacent cleanup.
12. Discovered work becomes a task box, a new feature, or a BACKLOG entry. Append a box
    unless it blocks the work below. Revise an incorrect task in its own commit with
    the reason in Log; never silently delete unfinished work.
13. Use `kit drop <id> "<why>"` for a plan that should not be built. Use `kit release
    <id>` to surrender untouched work. To pause work with changes, commit and push it,
    open the pull request unfinished, and say what remains.
14. Never put real secrets in docs. Name the variable and where its value lives.
    Only `kit submit`, `kit drop` and `kit reopen` write status and submitted fields.

### Finishing and review

15. Before submitting, move durable facts to `docs/TECH.md`; leave only changes to the
    plan in Log, which may be empty. Put validation and review context in the PR.
16. Run `kit finish -- <gate argv>` with the project's actual gate executable and arguments.
    It requires complete tasks and a clean tree, runs the gate and strict online audit,
    submits, and verifies the push. The individual project gate → `kit audit --strict` →
    `kit submit <id>` → push sequence remains supported. A failed or incomplete check is not a pass. Explicit offline
    audits cover local checks only; complete the online audit before handing off to review.
17. Push the branch, open the pull request, and hand it to the human reviewer.
    `submit` records readiness for review; the feature is integrated only after merge.
    You never merge your own work.
    With no remote or forge access, report the incomplete PR handoff and preserve the branch.
    Review waits apply to that feature. Return to a clean, updated trunk checkout, run
    `kit next`, and continue independent approved work without waiting for that merge.
18. For review changes before merge, run `kit reopen <id> "<why>"`, revise the tasks,
    implement the changes, and repeat the gate and submit steps. A Submitted doc on the
    trunk is frozen history; further changes need a new doc citing it.
19. After any merge, return to a trunk checkout, pull with --ff-only, then run
    `kit audit` and `kit next`. Dependencies unlock only once their merged work is here.
    Release your retained merged claims with `kit release`; custom checkouts are preserved.
    If work merged as Draft, preserve unchecked tasks and resolve its retained claim;
    never mark missing validation complete. Duplicate ids from offline drafting must be
    resolved before either doc is claimed.

### Writing

- Keep decisions, constraints and measurements; omit narration recoverable from git.
- Give each fact one home: TECH for what remains true, the feature for its plan, Log
  for a change to that plan, and the PR for validation and handoff.
- Cite another feature by id instead of copying its reasoning.
- A comment explains a contract or constraint the code cannot express.
- Prefer enforcement in the tool to another rule in this protocol. Kit can refuse its
  own commands; remote permissions and required checks enforce who may merge.

`kit help template` gives the doc format, `kit help backlog` the deferred-entry format,
and `kit help checks` the audit checks and their limits.
