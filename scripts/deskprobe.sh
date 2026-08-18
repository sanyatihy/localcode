#!/usr/bin/env bash
# Print one JSON object of the compositor's state, so desktop usability can be judged
# from outside the model process.
#
# The pass criterion this project needs is whether the machine stays usable, and the
# model's own success cannot answer that — 0003's 64k cell completed its request while
# windows stopped rendering. WindowServer is the process that draws the desktop, so its
# ability to keep working is the cheapest proxy available for whether the desktop is.
#
# Cumulative CPU *time* is reported, not a rate. `ps %cpu` is a decayed average over a
# window macOS does not document, and a threshold set against an undocumented decay is not
# reproducible; a difference between two cumulative readings is arithmetic. Rates are
# derived by the caller from consecutive samples, which is also what makes the sampling
# interval visible in the data rather than baked into it.
#
# The timestamp comes from perl because BSD `date` has no sub-second format and the
# interval is short enough that whole-second quantisation would dominate the rate.
set -euo pipefail

read -r pid cputime <<<"$(ps -Ao pid,time,comm | awk '/SkyLight.*WindowServer/ {print $1, $2; exit}')"
: "${pid:=0}" "${cputime:=0}"

# TIME is MM:SS.ss, HH:MM:SS.ss or DD-HH:MM:SS.ss depending on how long the process has
# been up. Parse all three rather than the one this machine happens to print today.
cpu_seconds=$(awk -v t="$cputime" 'BEGIN{
  d=0; rest=t
  if (split(t, a, "-") == 2) { d=a[1]; rest=a[2] }
  n=split(rest, b, ":")
  if      (n == 3) s = b[1]*3600 + b[2]*60 + b[3]
  else if (n == 2) s = b[1]*60 + b[2]
  else             s = rest
  printf "%.2f", d*86400 + s}')
t=$(perl -MTime::HiRes -e 'printf "%.3f", Time::HiRes::time()')

printf '{"t":%s,"windowserver_pid":%s,"windowserver_cpu_seconds":%s}\n' \
  "$t" "$pid" "$cpu_seconds"
