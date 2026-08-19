# Harness configuration

How each candidate harness is pointed at the local endpoint. All four work.

| harness | mechanism | where it lives | status |
|---|---|---|---|
| **Pi** | extension registering a provider | [`pi/local-provider.js`](pi/local-provider.js), loaded with `pi -e` | works |
| **OpenCode** | `provider` block using `@ai-sdk/openai-compatible` | [`opencode/opencode.json`](opencode/opencode.json), copied into the working dir | works |
| **Hermes** | top-level `model:` block with `provider: custom` | global, not repo-local | [works](hermes/README.md), needs 64k |
| **Claude Code** | environment variables, and the only Anthropic Messages client | [`claude-code/claude-code.env`](claude-code/claude-code.env) | [works](claude-code/README.md), needs a patched chat template |

## Which projects these are

Checked against each project's own page, because two of these names are ambiguous and one
of the ambiguities is a different project altogether.

| harness | upstream | installed here | via |
|---|---|---|---|
| **Pi** | `earendil-works/pi`, published as npm `@earendil-works/pi-coding-agent` | 0.84.1 | homebrew/core `pi-coding-agent` |
| **OpenCode** | `anomalyco/opencode` | 1.18.18 | tap `anomalyco/tap` |
| **Hermes** | `NousResearch/hermes-agent` | v0.20.1 (2026.8.13) | homebrew/core `hermes-agent` |
| **Claude Code** | Anthropic, `claude` on PATH | 2.1.233 | — |

**Two unrelated projects answer to "opencode", and the archived one does not lead here.**
`opencode-ai/opencode` was archived on 2025-09-18, and its notice sends readers to **Crush**,
continued by the original author with the Charm team. The project scored here is
`anomalyco/opencode`, which is a different lineage that carries the same name — so following
the archived repository's own advice arrives somewhere else again.

**Hermes' 64,000-token floor is documented nowhere upstream.** Its page states no minimum
context, so the floor is visible only in the refusal and in the source. That is why this repo
asserts it in a test rather than citing it.

**These versions are frozen for the comparison.** Pi and Hermes both have newer releases
(0.84.2 and 2026.8.18); they are deliberately not taken, because 0008's per-harness overhead
figures were measured on the versions above and upgrading mid-comparison would mix two
measurements under one name. Upgrading is its own decision, and its own re-measurement.

Only Claude Code takes a local endpoint through an environment variable. Pi ignores
`OPENAI_BASE_URL` and calls `api.openai.com` (401); OpenCode ignores `LOCAL_ENDPOINT` —
its model list is byte-identical with and without it. Both facts appear in write-ups
online and neither is true.

## Verified, not assumed

Each "works" above means the harness completed the `patch-nil-check` fixture in a scratch
Go module and the **unseen test passed afterwards** — not that it started, and not that it
claimed success.

    Pi           38.5 s   at 32k
    Claude Code  59.8 s   at 32k, --tools Read,Edit,Write
    OpenCode    3 m 06 s  at 32k
    Hermes      4 m 45 s  at 64k — it refuses anything under 64k

All four run under `cmd/tier2`, which stages the fixture in a scratch module and scores
what the harness left behind:

    go run ./cmd/tier2 -drivers claude-code -fixture tasks/patch-nil-check \
      -instruction "session.go has a bug: RefreshToken panics when given a nil token. …"

The instruction behind the three earlier timings was not recorded, so their numbers and
Claude Code's are single runs of the same fixture rather than a matched comparison.

All four produced a correct fix. The gap is behavioural rather than model-related: OpenCode
spent turns running `go build` and `go vet` where Pi went straight to the edit. That is
0010's context-frugality question showing up before 0010 runs, and it is one task, so it
is a signal and not a result.

## Reproducing

    # Pi
    LOCAL_OPENAI_API_KEY=local pi -p -e harness/pi/local-provider.js \
      --provider local --model 'bartowski/Qwen3.8-27B-GGUF:Q4_K_M' --tools read,edit,write "<task>"

    # OpenCode — needs opencode.json in the working directory
    cp harness/opencode/opencode.json <workdir>/
    opencode run -m 'local/bartowski/Qwen3.8-27B-GGUF:Q4_K_M' "<task>"

    # Hermes — global config only, and the server must serve >= 64k
    hermes --yolo --cli --in <workdir> -z "<task>"

## What each spends before the user's first word

System prompt plus tool definitions, from the request each harness actually sends, rendered
through the served template and counted by the served tokeniser. Rows in
[`docs/data/2026-08-19-m2max-32gb-0008-harness-overhead.jsonl`](../docs/data/2026-08-19-m2max-32gb-0008-harness-overhead.jsonl).

| harness | tools | system | tool defs | fixed | of a 32k context |
|---|---|---|---|---|---|
| Claude Code, defaults | 21 | 2,845 | 15,304 | **18,388** | 56% |
| Hermes | 17 | 7,415 | 10,040 | **17,465** | 53% |
| OpenCode | 10 | 4,477 | 5,321 | **9,810** | 30% |
| Pi | 4 | 2,991 | 921 | **3,922** | 12% |
| Claude Code, `--tools Bash,Edit,Read,Write` | 4 | 1,662 | 1,810 | **3,711** | 11% |

**Tool definitions are the cost, and they are configuration.** The same harness is both the
most and the least expensive row here, so a comparison that takes each harness's defaults
ranks their tool inventories rather than the harnesses. `Workflow` alone is 5,316 tokens.

This is a floor, not a per-turn price: llama.cpp reuses the prefix, so a session pays it
once and each later turn pays for what changed. It is also what a cold session waits
through — 18,388 tokens at the ~110 tok/s this machine ingests at low depth is about three
minutes before the first token of the answer.
