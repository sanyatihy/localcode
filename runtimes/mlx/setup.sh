#!/usr/bin/env bash
# Create the isolated environment MLX runs in. The vision confines Python to its own
# environment and calls it — nothing here is imported by the Go code, and the Go binary
# never shares a process with a Python runtime competing for the 32 GB the model needs.
#
# Idempotent: safe to re-run, and the venv is gitignored while the pins are committed, so
# the environment is reproducible without the 2 GB of wheels being in the repo.
set -euo pipefail
cd "$(dirname "$0")"

python3 -m venv .venv
./.venv/bin/pip install --quiet --upgrade pip
./.venv/bin/pip install --quiet -r requirements.txt
echo "mlx env ready:"
./.venv/bin/python -c "import mlx.core as mx, mlx_lm; print('  mlx', mx.__version__, '| mlx_lm', mlx_lm.__version__)"
