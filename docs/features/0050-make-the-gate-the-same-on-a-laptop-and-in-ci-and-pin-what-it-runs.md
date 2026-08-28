---
id: 0050
title: Make the gate the same on a laptop and in CI, and pin what it runs
status: Shipped
created: 2026-08-28
shipped: 2026-08-28
needs:
---

## Problem

`.golangci.yml` says it is "pinned so CI and a laptop agree", and nothing enforces it: CI
pins the binary to v2.12.2 and `make lint` runs whatever is on `PATH`, so a laptop can pass
a gate CI fails. The workflow's three actions are the only third-party code this repo runs
— it has no Go dependencies at all — and all three are pinned by mutable tag. There is no
vulnerability scan, which this project's own Go checklist asks for.

## Non-goals

- **Not more linters for their own sake.** `gosec` was run and produced 18 findings, every
  one of them a property of what this tool is: seeded RNG that must be deterministic,
  subprocesses that are the job, `0644` state files on a machine VISION says is
  single-user. It is refused, and the reason is recorded so it is not re-litigated.
- **Not reproducible binaries.** `-trimpath` and `CGO_ENABLED=0` belong to artifacts
  somebody downloads. `make install` builds one binary into one developer's `~/.local/bin`
  with that checkout's path deliberately compiled in.
- **Not pinning the workflow's actions to commit SHAs.** They are the only third-party code
  here and their tags are mutable, which is a real hole and a shallow one: the workflow
  grants `contents: read`, holds no secret, and publishes nothing, so what a moved tag could
  reach is a public checkout and a lint run. It is a `BACKLOG.md` line rather than a box
  here, because the gate has a defect that costs something today and this does not.

## Design

**A pinned version that is not enforced is a comment.** `go run <pkg>@<version>` resolves
one version on both sides, so the pin becomes a fact rather than a claim in two files that
can drift, and it removes the `make lint` skip path: the tool is fetched rather than looked
for. The version is a variable in the Makefile, which is the one place naming it.

**Not a `tool` directive, which was this doc's first answer.** Measured: adopting
golangci-lint that way adds 212 require lines and 926 `go.sum` entries to a module that has
neither. Zero dependencies is a property worth more than the convenience, and `go run` with
a version buys the same pin without touching the module graph — 11.8 s to build cold, and
cached after.

**`govulncheck` still earns its place with no dependencies.** What it checks then is the
standard library and the toolchain, which this repo pins in `go.mod` and which does get
advisories. It reports only vulnerabilities on paths the code actually reaches, so a finding
is a finding rather than a version comparison.

**Nine linters are adopted because they found nothing.** `errorlint`, `bodyclose`, `nilerr`,
`unconvert`, `wastedassign`, `misspell`, `copyloopvar`, `durationcheck` and `makezero` were
run over the tree and reported zero issues. That is the argument for them: they cost no
cleanup and they hold properties this code already has — wrapped errors compared with
`errors.Is`, response bodies closed, a nil error never returned beside a non-nil result.

## Tasks

- [x] `make lint` and CI run one pinned golangci-lint named in one place, with no skip
      path on either side and no change to the module graph
- [x] `govulncheck` runs in the gate, and the linter set is the nine that were clean with
      the refusal of `gosec` recorded where the next reader will look

## Open questions

- **The repository has no `LICENSE`.** VISION names a second audience — "anyone reproducing
  the same choices on comparable Apple Silicon, which is why every number here is committed
  rather than remembered" — and with no licence that audience has no right to use any of it.
  Only the developer can choose one, so this is the question rather than a task. Leaning:
  a permissive licence, because the deliverable is measurements and configuration meant to
  be copied, and a copyleft one would attach terms to the copying that is the point.

## Log
- 2026-08-28 — the first box named `go.mod` as where the pin lives, and a `tool` directive
  is what that means. Measured before writing it: 212 require lines and 926 `go.sum`
  entries against a module that has zero dependencies and no `go.sum`. The pin is worth
  having and that price is not, so the box now asks for one place rather than that place,
  and `go run <pkg>@<version>` is what it turns out to be. It also lets CI drop
  `golangci-lint-action`, which is one of the three third-party actions the backlog holds
  an entry about.
- 2026-08-28 — `govulncheck`'s first run was not a formality: it reported five standard
  library advisories reachable from this code — `net/url`, `crypto/tls` twice,
  `encoding/asn1` and `net/http` — all fixed in go1.26.6 against the go1.26.4 `go.mod`
  pinned. The toolchain is bumped in the same commit, which is the whole of the fix and is
  the argument for the step.
