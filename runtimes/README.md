# The runtimes that are not llama.cpp

llama.cpp is what this project serves, and [`scripts/serve.sh`](../scripts/serve.sh) starts
it. These are the alternatives that were measured against it and lost.

[`mlx/`](mlx/) is Apple's MLX. It wins on memory and warm reuse, and loses on `/v1/messages`,
on reporting no served config to check a run against, and on stalling rather than degrading
when memory runs out. It is kept because a comparison nobody can re-run is not evidence, and
because it can still be reopened without new hardware: `mlx_lm` gaining `qwen3_5_mtp` support
and beating llama.cpp's own MTP path, or MLX gaining `/v1/messages`.

It pins its own Python and is called rather than imported — the vision keeps Python out of the
Go, and out of the memory the model needs. It must never be **loaded** at the same time as
llama.cpp, since on 32 GB either alone is most of the machine, though it serves on a port of
its own so both can be configured at once.

**MTPLX was here and is not.** An MLX runtime with native multi-token prediction, it loaded and
then ran out of GPU memory under a real prompt, and more memory was the only thing that would
have changed that. With a larger machine decided against it has no route back, so the runtime
is gone and the refusal stays: the screen rows are in
[`docs/data/2026-08-20-m2max-32gb-0017-screen.jsonl`](../docs/data/2026-08-20-m2max-32gb-0017-screen.jsonl)
and the verdict is in
[Speculative decoding](../docs/TECH.md#speculative-decoding-adoptable-at-the-top-of-the-context-not-the-bottom).

The numbers: [llama.cpp against MLX](../docs/TECH.md#llamacpp-against-mlx).
