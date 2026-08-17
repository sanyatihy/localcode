# Harness configuration

How each candidate harness is pointed at the local endpoint. Two work, one is blocked.

| harness | mechanism | where it lives | status |
|---|---|---|---|
| **Pi** | extension registering a provider | [`pi/local-provider.js`](pi/local-provider.js), loaded with `pi -e` | works |
| **OpenCode** | `provider` block using `@ai-sdk/openai-compatible` | [`opencode/opencode.json`](opencode/opencode.json), copied into the working dir | works |
| **Hermes** | top-level `model:` block with `provider: custom` | global, not repo-local | [works](hermes/README.md), needs 64k |

None of them takes a local endpoint through an environment variable. Pi ignores
`OPENAI_BASE_URL` and calls `api.openai.com` (401); OpenCode ignores `LOCAL_ENDPOINT` —
its model list is byte-identical with and without it. Both facts appear in write-ups
online and neither is true.

## Verified, not assumed

Each "works" above means the harness completed the `patch-nil-check` fixture in a scratch
Go module and the **unseen test passed afterwards** — not that it started, and not that it
claimed success.

    Pi        38.5 s     at 32k
    OpenCode  3 m 06 s    at 32k
    Hermes    4 m 45 s    at 64k — it refuses anything under 64k

All three produced a correct fix. The gap is behavioural rather than model-related: OpenCode
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
