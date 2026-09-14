#!/usr/bin/env bash
# Build the llama.cpp that carries DFlash2, into the scratch prefix config/dflash2-*.env names.
#
# `--spec-type draft-dflash` exists only in PR #27342, which is open and not merged, so the
# binary that serves it cannot be the one on PATH — the Homebrew build stays where it is and
# this writes beside it. 0017 built this by hand on the laptop and recorded only the commit;
# the node needs the same binary, and a recipe nobody can re-run is what VISION calls not
# evidence.
#
# The commit is pinned rather than tracked. `dflash2` is a branch on an open pull request and
# moves; two builds under one name is the failure this whole repo's configs exist to prevent.
set -euo pipefail

SRC="${SRC:-$HOME/.local/src/llama.cpp-dflash2}"
REPO="${REPO:-https://github.com/z-lab/llama.cpp-fork.git}"
REF="${REF:-dflash2}"
COMMIT="${COMMIT:-5ecbe1ac17ec0484c5b44af0bd580cdc9c428ed4}"

command -v cmake >/dev/null 2>&1 || {
  echo "cmake is not installed: brew install cmake (no sudo needed)" >&2; exit 2; }

# Blobless rather than shallow: the pin is a commit, and a shallow clone of a branch head
# cannot check one out once the branch has moved past it.
if [ ! -d "$SRC/.git" ]; then
  mkdir -p "$(dirname "$SRC")"
  git clone --filter=blob:none "$REPO" "$SRC"
fi
git -C "$SRC" fetch origin "$REF"
git -C "$SRC" checkout --detach "$COMMIT"

# Metal on, shared backends, and libcurl so `-hf` can fetch the drafter the config names.
# The same three the laptop's build of this commit was configured with.
cmake -S "$SRC" -B "$SRC/build" \
  -DCMAKE_BUILD_TYPE=Release \
  -DBUILD_SHARED_LIBS=ON \
  -DGGML_METAL=ON \
  -DLLAMA_CURL=ON
cmake --build "$SRC/build" --target llama-server --config Release -j "$(sysctl -n hw.ncpu)"

echo "built:"
"$SRC/build/bin/llama-server" --version 2>&1 | head -2
