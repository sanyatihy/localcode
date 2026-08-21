# Depth tasks

Retrieval at 2k, 8k, 16k and 32k, with and without distractors below 16k. Run explicitly:

    go run ./cmd/eval -tasks tasks/depth ...

The 32k fixture is the deepest the grind profile can serve — 32,000 tokens of prompt
against a 32,768 context — and it costs about six minutes of ingest per run, which is why
it is not in the floor check either. It exists because a speculative decoder's advantage
decays with depth, so a ratio measured at 200 tokens says nothing about the context this
project actually serves.

**Not part of the ranking suite, and deliberately so.** These tasks are 71% of a
pass's runtime (227 s of 321 s with reasoning off) and have returned 3/3 at every setting
ever measured, so in a sampling or harness sweep they cost most of the clock and rank
nothing. They are a floor check against damage at depth, which is a property of the *KV
cache type and the serving backend* — not of sampling, thinking, or which harness drives
the model.

So run them when that axis moves: a KV cache type change (0008 names one), a backend
change (0006), or a new model. Not on every sweep.

`-tasks tasks` does not pick these up: task discovery globs `tasks/*.json` and descends
only into directories holding a `task.json`.
