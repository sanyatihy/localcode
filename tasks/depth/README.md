# Depth tasks

Nine fixtures that put a real prompt behind the question, in two families:

| fixture | asks | why it exists |
|---|---|---|
| `retrieval-2000/8000/16000/32000` | recall one planted key | catches damage that only shows at depth |
| `retrieval-distractor-2000/8000` | pick the *named* key out of three | recalling *a* key stops counting as an answer |
| `decode-8000/16000/32000` | recall the key, then count to 120 | a decode rate needs an answer long enough to time |

Run explicitly — `-tasks tasks` does not pick them up, because discovery globs
`tasks/*.json` and descends only into directories holding a `task.json`:

    go run ./cmd/eval -tasks tasks/depth ...

**The `decode-*` fixtures are an instrument, not a floor check.** A plain retrieval answer
is about 12 tokens, which is too few to divide into a rate; these return 384. They exist
because a speculative decoder's advantage decays with depth, so a ratio measured at 200
tokens says nothing about the context this project actually serves.

32,000 tokens of prompt against a 32,768 context is the deepest the grind profile can
serve, and it costs about six minutes of ingest per run.

**Not part of the ranking suite, and deliberately so.** These tasks are 71% of a
pass's runtime (227 s of 321 s with reasoning off) and have returned 3/3 at every setting
ever measured, so in a sampling or harness sweep they cost most of the clock and rank
nothing. They are a floor check against damage at depth, which is a property of the *KV
cache type and the serving backend* — not of sampling, thinking, or which harness drives
the model.

So run them when that axis moves: a KV cache type change, a backend change, or a new
model. Not on every sweep.
