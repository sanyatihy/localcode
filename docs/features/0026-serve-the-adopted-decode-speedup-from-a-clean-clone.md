---
id: 0026
title: Serve the adopted decode speedup from a clean clone
status: Draft        # Draft | Shipped | Dropped — kit ship and kit drop write it
created: 2026-08-21
shipped:             # written by kit ship, never by hand
needs:               # feature ids that must ship first, e.g. 0002, 0003. Empty = can start now
---

## Problem

`README.md`'s settled table names the model's own MTP head as the decode speedup and
`config/README.md` marks `mtp-32k.env` **adopted for the grind profile**, but that config
names `SERVER_BIN="$HOME/.local/src/llama.cpp-dflash2/build/bin/llama-server"` — a build of
an unmerged llama.cpp pull request that the repo records only as a PR number and a fork
commit in a comment. Getting Started installs the Homebrew build, which does not carry the
mechanism, and `make serve` serves `config/tuned.env`, which does not speculate. Against
"every server invocation lives in the repo", the reproducible artefact for an adopted answer
is a path in one developer's home directory.

## Non-goals

- **Not vendoring llama.cpp.** The Homebrew build stays the dependency for everything else,
  and a second copy of a C++ tree in this repo is not the smaller problem.
- **Not re-measuring the speedup.** 0017's rows stand; this is about whether anyone else
  can serve what they describe.
- **Not adopting it for the editor profile.** 49,152 is measured to refuse on the first
  prefill batch, and that verdict does not change.

## Design

**Deliberately unwritten until the blocking question is answered.** The two readings need
different work and one of them is `kit drop`, so designing for either now would be inventing
the answer. What holds in both: the record names the exact source and commit rather than a
home directory, and `README.md` stops describing as adopted anything a reader following
Getting Started cannot serve.

## Tasks

- [ ] The speculative configs name a binary the repo says how to obtain, pinned to the commit 0017's numbers were taken on
- [ ] A reader following `README.md` can either serve the adopted config or is told plainly why not
- [ ] `config/dflash2-32k.env` and `dflash2-49k.env` are kept or cut on the same rule, since they name the same binary for a candidate that never generated a token

## Open questions

Settled by the blocking question in [`docs/INBOX.md`](../INBOX.md).

## Log
