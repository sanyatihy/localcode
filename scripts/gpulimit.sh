#!/usr/bin/env bash
# The ceiling on what the GPU may wire, in MiB, and where the number came from.
#
# `sysctl iogpu.wired_limit_mb` answers 0 on an untouched machine, and 0 does not mean
# unlimited: the kernel derives a limit from installed memory and does not publish it
# through that sysctl. Metal does, as the device's recommended maximum working set — so
# that is what this reads, and it is a reading rather than a fraction somebody assumed.
#
# It matters because the fraction is not what this repo assumed. Measured on this machine:
# 22,906,503,168 bytes, which is 21,845 MiB and exactly two thirds of 32 GiB, against the
# three quarters every headroom figure recorded before 2026-08-25 was computed from.
#
#   ./scripts/gpulimit.sh    # -> "21845 metal"
set -euo pipefail

mb=$(sysctl -n iogpu.wired_limit_mb 2>/dev/null || echo 0)
if [ "${mb:-0}" -gt 0 ]; then
  # Set by hand, and then it is the limit: the sysctl overrides what the kernel derived.
  printf '%s sysctl\n' "$mb"
  exit 0
fi

if ! command -v swift >/dev/null 2>&1; then
  # No reading is better than a fraction: a headroom figure computed from a guess is what
  # sent three features looking in the wrong place.
  printf '0 unread\n'
  exit 0
fi

bytes=$(printf 'import Metal\nprint(MTLCreateSystemDefaultDevice()!.recommendedMaxWorkingSetSize)\n' \
  | swift - 2>/dev/null | tr -dc '0-9') || true
if [ -z "$bytes" ] || [ "$bytes" = "0" ]; then
  printf '0 unread\n'
  exit 0
fi
printf '%s metal\n' "$((bytes / 1048576))"
