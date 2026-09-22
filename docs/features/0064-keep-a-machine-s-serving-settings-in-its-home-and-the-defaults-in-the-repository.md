---
id: 0064
title: Keep a machine's serving settings in its home and the defaults in the repository
status: Draft
created: 2026-09-22
submitted:
needs:
---

## Problem

<!-- What was this for? The tension that made it worth doing, in one to three sentences.
     The diff says what was built and cannot say why it was worth building. -->

## Non-goals

<!-- What this deliberately does not do, and where that lives instead. Delete the section
     if nothing was ruled out. -->

## Design

<!-- What was chosen and what it beat: an alternative rejected, a constraint that shaped
     the answer, a cost accepted. Only what a reader cannot recover from the code. -->

## Tasks

- [x] The daemon's wrapper applies ~/.config/localcode/serve.env over config/node.env, serves the merged file and names it, covered by a test, and README and TECH say so

## Log

<!-- What the doing taught that a plan would not have predicted: a premise falsified, an
     approach abandoned, a measurement that changed the shape. -->
