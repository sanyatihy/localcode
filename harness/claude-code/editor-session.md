# The editor flow, recorded

Three sessions on 2026-08-19, same prompt, same model, same server — the third the same
harness driven from a terminal instead. Taken from Claude Code's own session files, which
is why the model string is the served one rather than a label anybody typed.

## It refuses before it starts, if the context is the scorer's

    12:11:37  user       Who are you?
    12:11:48  assistant  API Error: 400 request (36297 tokens) exceeds the available
                         context size (32768 tokens), try increasing it

Nothing was typed beyond the question. The 36,297 tokens are the extension's fixed
preamble: 21 tool definitions, the project's own instructions, and the editor's context.

## It works at 49,152, and costs seven and a half minutes

    12:14:15  user       Who are you?
    12:21:57  assistant  I'm Claude Code — Anthropic's CLI for software engineering
                         tasks. In this environment I'm running inside the VSCode
                         extension, powered by a Qwen3.8-27B model, and working in your
                         `localcode-0008` repo on branch
                         `0008-wire-the-winning-config-into-the-coding-agent`.

    model   bartowski/Qwen3.8-27B-GGUF:Q4_K_M
    input   36,309 tokens        ingest 434 s at 83.7 tok/s
    output  82 tokens            5.54 tok/s
    total   462 s

The answer is correct about its own situation, which is the point: the editor, the agent
and the repo are the flow that was already in use, and only the backend changed.

## The same harness in a terminal, twelve minutes later

    12:26:36  user       Who are you?
    12:27:21  assistant  I'm Claude Code, Anthropic's official CLI for Claude … In this
                         session I'm running on a local model (a quantized GGUF), but I
                         operate as the Claude Code harness …

    invocation  claude --tools Read,Edit,Write,Bash
    input       3,767 tokens
    output      116 tokens
    total       45 s

**One flag is the difference between 45 seconds and 462.** Same model, same server, same
question; the extension has no equivalent of `--tools`, so its preamble is not reducible
from the client side. That is the finding this feature ends on, and it is a fact about
configuration rather than about either harness.
