"""The desktop verdict, in one place because two instruments now read it.

The rule and its thresholds are 0014's, and `scripts/ladder.sh` documents why each bound
sits where it does. What matters here is that a fill and a screen judge a cell the same
way: a threshold with two homes drifts, and the verdict is the whole point of both.
"""

SATURATED_CORES = 0.90
STALLED_CORES = 0.02
SUSTAIN_SECONDS = 30


def load(path):
    """Read a deskprobe series, ordered by its own timestamps."""
    try:
        with open(path) as fh:
            return sorted((__import__("json").loads(x) for x in fh if x.strip()),
                          key=lambda r: r["t"])
    except (OSError, ValueError):
        return []


def _cores(a, b):
    span = b["t"] - a["t"]
    return None if span <= 0 else (b["windowserver_cpu_seconds"] - a["windowserver_cpu_seconds"]) / span


def verdict(desk, condition, saturated=SATURATED_CORES, stalled=STALLED_CORES,
            sustain=SUSTAIN_SECONDS):
    """Judge one cell from its compositor series. Returns the verdict and what it read."""
    steps = [c for c in (_cores(a, b) for a, b in zip(desk, desk[1:])) if c is not None]

    # Every window of at least the sustain length. A single spike is the compositor doing
    # its job; a spike that does not end is the compositor losing.
    sustained = []
    for i in range(len(desk)):
        k = i + 1
        while k < len(desk) and desk[k]["t"] - desk[i]["t"] < sustain:
            k += 1
        if k < len(desk):
            c = _cores(desk[i], desk[k])
            if c is not None:
                sustained.append(c)

    if not condition.startswith("attended"):
        v = "not_applicable"
    elif not sustained:
        v = "insufficient_samples"
    elif max(sustained) >= saturated:
        v = "fail_saturated"
    elif min(sustained) <= stalled:
        v = "fail_stalled"
    else:
        v = "pass"

    return {
        "desktop_verdict": v,
        "ws_cpu_peak_cores": round(max(steps), 3) if steps else None,
        "ws_cpu_sustained_max_cores": round(max(sustained), 3) if sustained else None,
        "ws_cpu_sustained_min_cores": round(min(sustained), 3) if sustained else None,
        "ws_span_seconds": round(desk[-1]["t"] - desk[0]["t"], 1) if len(desk) > 1 else 0,
        "ws_samples": len(desk),
        "desktop_thresholds": {"saturated_cores": saturated, "stalled_cores": stalled,
                               "sustain_seconds": sustain},
    }
