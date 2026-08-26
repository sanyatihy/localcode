#!/usr/bin/env bash
# Apply or undo a raise of the GPU wired cap, and refuse one the system cannot survive.
#
# `iogpu.wired_limit_mb` is the budget Metal allocates the model's buffers from. 0042 exists
# because a config can be refused by it while free memory, swap and resident size all look
# healthy, so moving it is the only way to attribute that refusal — and moving it by hand is
# how a row gets recorded under a cap nobody wrote down.
#
# The raise does not survive a reboot, and that is what bounds the risk rather than any
# check here: starving the system side of unified memory freezes the machine instead of
# returning an error, and a restart is the way out of one.
#
#   ./scripts/gpuraise.sh          # what is in force, and where it came from
#   ./scripts/gpuraise.sh 24576    # raise to 24,576 MiB
#   ./scripts/gpuraise.sh reset    # back to the kernel's own derivation
set -euo pipefail
cd "$(dirname "$0")/.."

# What the system side keeps, whatever the GPU is allowed. The same reserve scripts/rungs.sh
# holds back for the OS, the editor and the browser, and for the same reason: the ceiling
# this raises is bought from one pool. Overridable because it is a judgement, not a reading.
RESERVE_GB="${RESERVE_GB:-8}"

[ "$(uname -s)" = "Darwin" ] || { echo "gpuraise: iogpu.wired_limit_mb is a macOS sysctl" >&2; exit 2; }

total_mb=$(( $(sysctl -n hw.memsize) / 1048576 ))
ceiling_mb=$(( total_mb - RESERVE_GB * 1024 ))

read -r current_mb current_source <<<"$(./scripts/gpulimit.sh)"
report() { printf '%s MiB (%s)\n' "$1" "$2"; }

case "${1:-}" in
  "")
    report "$current_mb" "$current_source"
    exit 0
    ;;
  reset|0)
    sudo sysctl -w iogpu.wired_limit_mb=0 >/dev/null
    ;;
  *[!0-9]*)
    echo "gpuraise: want a value in MiB, 'reset', or nothing" >&2
    exit 2
    ;;
  *)
    want="$1"
    # A raise on top of a raise cannot be checked: gpulimit reports the sysctl once one is
    # set, so the kernel's own derivation — the floor a lowering would go under — is no
    # longer readable. Reset first and the next call sees it again.
    if [ "$current_source" = "sysctl" ]; then
      echo "gpuraise: $current_mb MiB is already set by hand; reset before raising again" >&2
      exit 2
    fi
    if [ "$current_source" = "unread" ]; then
      echo "gpuraise: the derived cap could not be read, so a raise cannot be checked against it" >&2
      exit 2
    fi
    # Lowering is a different experiment, and doing it under the same command is how a row
    # ends up attributed to a cap nobody meant to set.
    if [ "$want" -lt "$current_mb" ]; then
      echo "gpuraise: $want MiB is below the derived $current_mb MiB — that is a lowering, not a raise" >&2
      exit 2
    fi
    if [ "$want" -gt "$ceiling_mb" ]; then
      echo "gpuraise: $want MiB leaves the system under ${RESERVE_GB} GiB of ${total_mb} MiB; the most this allows is $ceiling_mb" >&2
      exit 2
    fi
    sudo sysctl -w iogpu.wired_limit_mb="$want" >/dev/null
    ;;
esac

# A sysctl that returned 0 is not a sysctl that took. Read it back through the same path
# every row reads it through, so what is reported is what a measurement will record.
read -r now_mb now_source <<<"$(./scripts/gpulimit.sh)"
printf 'was %s MiB (%s), now ' "$current_mb" "$current_source"
report "$now_mb" "$now_source"
echo "gpuraise: not persistent — a reboot restores the kernel's derivation" >&2
