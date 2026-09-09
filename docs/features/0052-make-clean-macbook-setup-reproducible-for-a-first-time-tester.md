---
id: 0052
title: Make clean MacBook setup reproducible for a first-time tester
status: Submitted
created: 2026-09-09
submitted: 2026-09-09
needs:
---

## Problem

A first-time tester on a clean MacBook could not follow the README to a coding
session: installation, PATH, Claude Code and a verifiable first task were missing.
The entry point prioritized measurement commands and named a stale runtime build.

## Non-goals

Changing runtime behavior, choosing new model settings, or claiming validation on
hardware other than the measured machine.

## Design

Use a sequential Terminal walkthrough with a foreground first download, then a
disposable repository task whose created file can be run independently. Foreground
startup exposes progress and avoids the launcher's 20-minute wait on slow downloads.
Keep benchmarks optional and distinguish current Homebrew installs from measured
versions, whose durable record remains in TECH.

## Tasks

- [x] Make clean MacBook setup reproducible for a first-time tester

- [x] Shorten the full README while keeping a complete first-run path and consistent defaults

## Log
- 2026-09-09 — reopen to Draft: Review asks for a shorter setup and a consistent README throughout
