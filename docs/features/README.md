# Features

One file per feature. This directory is the board — the files, plus each one's
`status:`, are what is in flight. Start a feature by copying [TEMPLATE.md](TEMPLATE.md).

Run `kit next` to be told which feature and task to pick up.

## Conventions

- **`Draft → Accepted → Shipped`** (exits: `Dropped` / `Superseded`). `status:` in the
  frontmatter is canonical. There is no "in progress": the claim branch says that, and a
  second copy in the doc could only disagree with it.
- **List order is the order of work.** Do the topmost unticked `## Tasks` box. If two
  boxes can genuinely happen in either order, it does not matter which is first.
- **A branch named after the doc is the claim on it** (`0009-ton-payment-rail`).
  One agent per feature; that keeps two agents out of the same code.
- **One source of truth per fact.** While in flight the feature doc owns the spec.
  On ship, durable facts move to the as-built doc and this file freezes as history.
- **Each `## Log` entry names a change to the plan.** What the work did before it
  converged lives in the PR; what is still true after ship lives in the as-built doc.
- **As much as necessary and no more.** Cut restatement and hedging; never cut a
  decision, a constraint, or the evidence for it. Each section states its own budget.
- **IDs are stable** (`0007`). Cite them in commits and branch names; files never move.
- **`check:` is optional.** Set it only when the feature is a bet worth revisiting —
  a pricing change, a growth experiment. Most features do not need one.
- **`review:` is optional and the human's.** `kit accept <id> --review` writes
  `review: human`: a person merges this one, whatever branch protection allows on its
  own. Empty means a coordinator may. No agent sets it.
