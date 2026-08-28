---
id: 0050
title: Make the gate the same on a laptop and in CI, and pin what it runs
status: Draft
created: 2026-08-28
shipped:
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

**A pinned version that is not enforced is a comment.** Go's `tool` directive puts
golangci-lint in `go.mod`, so `go tool golangci-lint` resolves the same version on both
sides and the pin becomes a fact rather than a claim in two files that can drift. It also
removes the `make lint` skip path: the tool is present because the module says it is.

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

- [ ] `make lint` and CI run one pinned golangci-lint that `go.mod` names, with no skip
      path on either side
- [ ] `govulncheck` runs in the gate, and the linter set is the nine that were clean with
      the refusal of `gosec` recorded where the next reader will look

## Open questions

- **The repository has no `LICENSE`.** VISION names a second audience — "anyone reproducing
  the same choices on comparable Apple Silicon, which is why every number here is committed
  rather than remembered" — and with no licence that audience has no right to use any of it.
  Only the developer can choose one, so this is the question rather than a task. Leaning:
  a permissive licence, because the deliverable is measurements and configuration meant to
  be copied, and a copyleft one would attach terms to the copying that is the point.

## Log
