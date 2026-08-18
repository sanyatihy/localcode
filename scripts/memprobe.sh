#!/usr/bin/env bash
# Print one JSON object of the memory facts that decide whether a config is viable.
#
# Swap is reported as a cumulative total because macOS does not release it within a
# session. The signal is therefore the *delta* across a step, not the absolute value:
# a machine that has already swapped stays swapped until it reboots.
#
# Wired memory is here because swap and RSS could not see the failure that matters.
# Metal holds the model in wired buffers, which are never paged out and are capped by
# `iogpu.wired_limit_mb`; a config can exhaust that cap while free memory looks healthy
# and nothing swaps at all. That is the state 0003's ladder scored `ok` at 64k while the
# desktop stopped rendering.
#
# The limit is reported with its provenance. A sysctl of 0 means the system default,
# which Apple documents as roughly 75% of installed RAM but does not expose as a number,
# so the derived figure is labelled `default-assumed` and never presented as a reading.
# Headroom computed against it inherits that uncertainty — deliberately, because the
# alternative is omitting the only ceiling figure available.
set -euo pipefail

# Read the page size rather than assuming it. This was hardcoded to 4096 and this machine
# pages at 16384, so every free and compressor figure the ladder recorded was understated
# fourfold. It changed no conclusion — free memory is pinned near zero at any scale factor,
# which is exactly why it is not a pressure signal — but a wrong number in a committed
# results file is a wrong number, and wired memory is about to be read off the same counter
# to decide which configs are admissible.
PAGE=$(pagesize)
read -r free compressed wired <<<"$(vm_stat | awk -v p=$PAGE '
  /Pages free/                   {f=$3}
  /Pages occupied by compressor/ {c=$5}
  /Pages wired down/             {w=$4}
  END {gsub(/\./,"",f); gsub(/\./,"",c); gsub(/\./,"",w)
       printf "%.3f %.3f %.3f", f*p/1073741824, c*p/1073741824, w*p/1073741824}')"
swap=$(sysctl -n vm.swapusage | awk '{gsub(/M/,"",$6); print $6}')
rss=$(ps -Ao rss,comm | awk '/llama-server/ {s+=$1} END {printf "%.3f", s/1024/1024}')
[ -z "$rss" ] && rss=0

limit_mb=$(sysctl -n iogpu.wired_limit_mb 2>/dev/null || echo 0)
if [ "$limit_mb" -gt 0 ]; then
  limit_gb=$(awk -v m="$limit_mb" 'BEGIN{printf "%.3f", m/1024}')
  limit_source=sysctl
else
  limit_gb=$(awk -v b="$(sysctl -n hw.memsize)" 'BEGIN{printf "%.3f", b*0.75/1073741824}')
  limit_source=default-assumed
fi
headroom=$(awk -v l="$limit_gb" -v w="$wired" 'BEGIN{printf "%.3f", l-w}')

printf '{"free_gb":%s,"compressed_gb":%s,"swap_used_mb":%s,"llama_rss_gb":%s,"wired_gb":%s,"wired_limit_mb":%s,"wired_limit_gb":%s,"wired_limit_source":"%s","wired_headroom_gb":%s}\n' \
  "$free" "$compressed" "$swap" "$rss" "$wired" "$limit_mb" "$limit_gb" "$limit_source" "$headroom"
