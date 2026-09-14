#!/usr/bin/env bash
# Apply the node's GPU wired cap. Run as root at boot by com.localcode.gpucap, because
# `iogpu.wired_limit_mb` does not survive a reboot and the sysctl needs root.
#
# The value is total memory less the machine file's reserve, computed here rather than
# written into the plist: the reserve is a measurement 0056 takes on the node, and a number
# copied into a property list is a second home for it.
set -euo pipefail
cd "$(dirname "$0")/../.."

# The node's own file, not the laptop's. A node that serves and nothing else keeps far less.
MACHINE="${MACHINE:-config/machine-m5pro-24gb.json}"
export MACHINE

# shellcheck source=scripts/lib.sh
. scripts/lib.sh

total_mb=$(( $(sysctl -n hw.memsize) / 1048576 ))
want=$(( total_mb - $(reserve_gb) * 1024 ))
exec ./scripts/gpuraise.sh "$want"
