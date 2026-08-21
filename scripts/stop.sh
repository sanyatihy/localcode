#!/usr/bin/env bash
# Stop the server and wait for the memory back. `make serve` runs in the foreground, so
# Ctrl-C is the usual end; this is for the shell that is gone, backgrounded or on another
# desk, and for the manual's fourth step.
#
# It adds nothing to stop_server. A bare `pkill` returns while ~17 GB is still being
# released, and the next serve then binds while the old process still holds the port —
# which measures the previous config under the next config's name. scripts/lib.sh owns
# that wait, and re-solving it here is what gave four scripts four answers before.
set -euo pipefail

cd "$(dirname "$0")/.."
# shellcheck source=scripts/lib.sh
. scripts/lib.sh

server_alive || { echo "no $PROC to stop" >&2; exit 0; }
stop_server
echo "stopped $PROC" >&2
