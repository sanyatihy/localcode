#!/usr/bin/env bash
# Create the isolated environment MTPLX runs in, mirroring runtimes/mlx/setup.sh: the vision confines
# Python to its own environment and calls it, and this one cannot share runtimes/mlx/'s — MTPLX pins
# transformers below the version 0006's comparison was measured on, and a shared venv would
# silently move that comparison's runtime.
#
# Idempotent: safe to re-run, and the venv is gitignored while the pins are committed.
set -euo pipefail
cd "$(dirname "$0")"

python3 -m venv .venv
./.venv/bin/pip install --quiet --upgrade pip
./.venv/bin/pip install --quiet -r requirements.txt
echo "mtplx env ready:"
./.venv/bin/mtplx --version
