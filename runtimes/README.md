# The runtimes that are not llama.cpp

llama.cpp is what this project serves, and [`scripts/serve.sh`](../scripts/serve.sh) starts
it. These are the alternatives that were measured against it and lost.

| | verdict |
|---|---|
| [`mlx/`](mlx/) | Apple's MLX. Wins on memory and warm reuse, loses on `/v1/messages`, on introspection, and on how it fails under pressure |
| [`mtplx/`](mtplx/) | An MLX runtime with native multi-token prediction. Loads, then runs out of GPU memory under a real prompt |

They are kept because a comparison nobody can re-run is not evidence, and because the 128 GB
machine is the condition that reopens both. Each pins its own Python and is called rather
than imported: the vision keeps Python out of the Go, and out of the memory the model needs.

Neither may be **loaded** at the same time as llama.cpp — on 32 GB any one of them is most of
the machine — though each serves on a port of its own so all three can be configured at once.

The numbers: [llama.cpp against MLX](../docs/TECH.md#llamacpp-against-mlx) and
[Speculative decoding](../docs/TECH.md#speculative-decoding-adoptable-at-the-top-of-the-context-not-the-bottom).
