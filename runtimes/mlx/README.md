# MLX — the runtime that lost, kept reproducible

Apple's MLX serving the same model, so `cmd/eval` can score it with nothing changed but
`-endpoint`. **llama.cpp won and stays**; this directory exists because a comparison
nobody can re-run is not evidence.

```sh
./runtimes/mlx/setup.sh                      # one venv, from the committed pins
./runtimes/mlx/serve.sh                      # mlx_lm on 127.0.0.1:8082, so both can be configured at once
./runtimes/mlx/compare.sh mlx http://127.0.0.1:8082
```

Both runtimes can be **configured** at once and must never be **loaded** at once: on 32 GB
either alone is most of the machine.

Python is confined here and called, never merged into the Go code — one static binary, no
runtime competing with the model for the memory it needs. `requirements.txt` is pinned so a
re-run reproduces the runtime the comparison was measured on; `.venv/` is gitignored.

## What it was measured to be

MLX wins on memory — **18.02 GB wired against llama.cpp's 20.89**, zero swapped runs
against seven — and on warm reuse, 39× against 10×. It loses on three things that mattered
more: it serves no Anthropic `/v1/messages`, which the editor flow needs; it reports no
served config or timings, so a run cannot be checked against its label; and its failure
mode under memory pressure is a hard stall rather than degradation.

**`--prompt-cache-bytes` and `--prompt-cache-size` are mandatory here, not tuning**, and
the slot count is the part that bites. `mlx_lm`'s LRU is unbounded by default: one 16k
prompt drove free memory to zero, with swap flat because wired pages cannot be paged out.
Bounded but generous is no better — capacity is allocated eagerly *per slot* at startup, so
16 slots left 0.11 GB free on the first request where 2 left 5.38, and every depth row of
that run hit its budget.

[`config/mlx-4bit.env`](config/mlx-4bit.env) therefore serves **2**, which is what the
scored comparison ran on and what one agent conversation needs. Reuse breadth and depth
headroom trade directly against each other on 32 GB.

The full table, and what would reverse the decision:
[llama.cpp against MLX](../../docs/TECH.md#llamacpp-against-mlx).
