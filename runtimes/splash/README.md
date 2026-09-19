# Splash — the runtime being measured against llama.cpp

Splash is Apache-2.0 and serves Qwen3.8-27B with the DFlash2 drafter as its only decode
path. It is here because it answers `/v1/messages`, which is the first of the two things
[TECH](../../docs/TECH.md) holds against MLX, and because no runtime but llama.cpp has ever
been driven through a whole chain on this project. What it is worth is
[0060](../../docs/features/0060-measure-splash-against-llama-cpp-on-the-node-through-a-whole-chain.md)'s
question; the vendor's figures are claims, and nothing here is a reading until a row says so.

```sh
./runtimes/splash/setup.sh                   # brew install incoai/tap/splash, or say it is there
./runtimes/splash/serve.sh                   # 127.0.0.1:8000, from config/splash-27b.env
```

It needs an M3 or newer, macOS 26.4 or later and 36 GB, so it runs on the node and not on
the laptop. There is no venv: Splash ships as a Homebrew bottle with its own Python, and
what pins this runtime is the version `setup.sh` prints. An installed Splash is never
upgraded by `setup.sh`, because a version that moves between runs is a confound.

## Weights

The package is one repository, `incoai/Qwen3.8-27B-Splash`, 17.4 GB over 82 files — target,
drafter, tokenizer and vision in one. `splash serve` downloads it with `huggingface_hub`
into the ordinary Hub cache and then symlinks its own model path at the snapshot, so the
cache is the only copy and staging it is staging the model.

The node's route to the Hugging Face CDN cannot deliver that (0058: 8–130 KB/s, stalled
twice at zero), so it is downloaded on the laptop and copied over the link, which runs at
37 MB/s:

```sh
hf download incoai/Qwen3.8-27B-Splash
rsync -a --copy-unsafe-links ~/.cache/huggingface/hub/models--incoai--Qwen3.8-27B-Splash \
  <user>@<node>:.cache/huggingface/hub/
```

`--copy-unsafe-links` is not optional: `huggingface_hub` 1.x keeps the bytes in a shared
store at the cache root and leaves symlinks into it under the repository, so a plain `-a`
copies 17.4 GB of dangling links. It copies those out-of-tree targets as files and leaves
the snapshot's own links alone, which is the layout every other model in that cache is in.

`serve.sh` then exports `HF_HUB_OFFLINE=1`, which is what makes Splash fall back to the
staged snapshot instead of checking the Hub for the repository's `main` at every load. It
verifies that snapshot against the package manifest either way, so for the same snapshot
offline changes where the weights are found and not what is served. Online, a `main` that
has moved past the staged revision would be downloaded and served instead.

## The config

[`config/splash-27b.env`](config/splash-27b.env) is the single source of truth for how a
measurement was produced, and `serve.sh` adds no flags of its own. It holds Splash to the
same 49,152-token window and the same 30,720 MiB ceiling `scripts/node/cap.sh` applies to
llama.cpp, so the pair differs by the runtime rather than by what each was allowed.

`HOST` and `PORT` in that file are not flags. `splash serve` has neither: its launcher binds
`127.0.0.1:8000` and refuses to start when anything else owns that address. They are
recorded because a measurement has to say where it was driven, and `serve.sh` refuses a
config naming anything else rather than let a row claim an address nothing bound.
