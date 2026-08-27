# MTPLX — the candidate this machine could not hold

MTPLX is an MLX runtime that implements native multi-token prediction. It was screened as a
speculative-decoding candidate and **refused**: it loads, and then runs out of GPU memory
under a real prompt. Its checkpoint is 20.68 GB of its own, against a machine where the
model already wires 20.89.

```sh
./runtimes/mtplx/setup.sh                    # its own venv — see below
./runtimes/mtplx/serve.sh                    # 127.0.0.1:8083
```

**It cannot share `runtimes/mlx/`'s environment.** MTPLX pins `transformers` below the version the
llama.cpp-against-MLX comparison was measured on, and a shared venv would silently move
that comparison's runtime. Two directories, two sets of pins, both committed.

The speedup this was chasing was found elsewhere and for nothing: the served GGUF has
carried its own MTP head all along, and llama.cpp PR #27342 makes a draft context against
the same weights. That is measured at **1.26–1.57×**, lossless, and costs 0.43 GB — see
[Speculative decoding](../../docs/TECH.md#speculative-decoding-adoptable-at-the-top-of-the-context-not-the-bottom).

Kept because the refusal is a result. What would reopen it is a machine with room for a
second checkpoint, which is the same condition that reopens MLX.
