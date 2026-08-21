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

MLX wins on wired memory and on warm reuse, and loses on `/v1/messages`, on reporting no
served config to check a run against, and on stalling rather than degrading under memory
pressure. The table, and what would reverse it:
[llama.cpp against MLX](../../docs/TECH.md#llamacpp-against-mlx).

**`--prompt-cache-bytes` and `--prompt-cache-size` are mandatory here, not tuning**, and the
slot count is the part that bites: capacity is allocated eagerly *per slot* at startup, and
the LRU is unbounded by default. [`config/mlx-4bit.env`](config/mlx-4bit.env) serves **2**,
which is what the scored comparison ran on and what one agent conversation needs. Reuse
breadth and depth headroom trade directly against each other on 32 GB.
