#!/usr/bin/env bash
# Print one JSON object of the memory facts that decide whether a config is viable.
#
# Swap is reported as a cumulative total because macOS does not release it within a
# session. The signal is therefore the *delta* across a step, not the absolute value:
# a machine that has already swapped stays swapped until it reboots.
set -euo pipefail

PAGE=4096
read -r free compressed <<<"$(vm_stat | awk -v p=$PAGE '
  /Pages free/            {f=$3}
  /Pages occupied by compressor/ {c=$5}
  END {gsub(/\./,"",f); gsub(/\./,"",c); printf "%.3f %.3f", f*p/1073741824, c*p/1073741824}')"
swap=$(sysctl -n vm.swapusage | awk '{gsub(/M/,"",$6); print $6}')
rss=$(ps -Ao rss,comm | awk '/llama-server/ {s+=$1} END {printf "%.3f", s/1024/1024}')
[ -z "$rss" ] && rss=0

printf '{"free_gb":%s,"compressed_gb":%s,"swap_used_mb":%s,"llama_rss_gb":%s}\n' \
  "$free" "$compressed" "$swap" "$rss"
