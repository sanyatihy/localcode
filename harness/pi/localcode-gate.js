// Holds a Pi session to the budget `localcode` gave it, by refusing tool calls.
//
// Loaded beside the provider with a second `-e`. Nothing is decided here: the payload is
// filled in from Pi's vocabulary, `localcode hook gate` answers, and a refusal is returned
// as a blocked call. The gate is the same binary and the same code the Claude Code hooks
// run, so the two harnesses cannot drift into two answers about what a session may spend.
import { spawnSync } from "node:child_process";
import { resolve } from "node:path";

// Pi's tool names against the gate's. Only three matter to it — `Read` takes the result
// cap, `Write` and `Edit` are how a handoff is written — and every other name is a call
// like any other, so an unmapped tool is passed through under its own name.
const NAMES = { read: "Read", write: "Write", edit: "Edit", bash: "Bash" };

// The gate names a file `file_path` and Pi names it `path`, and the gate compares it
// against the one absolute path the session may write out of turn. Resolved here, because
// a relative `HANDOFF.md` is the same file and would not match.
function toolInput(input, cwd) {
  const out = { ...input };
  if (typeof input?.path === "string") {
    out.file_path = resolve(cwd, input.path);
    delete out.path;
  }
  return out;
}

// What the session has spent, which under Pi is the harness's own number: `getContextUsage`
// reports the last assistant usage plus an estimate of what follows it. It is the peak as
// well as the current reading, because a session that cannot compact only grows.
function peakTokens(ctx) {
  return ctx.getContextUsage()?.tokens ?? 0;
}

// One process per call, which is what the Claude Code hooks already cost. Exit 2 is the
// refusal and stderr is the reason the model is given; anything else stands aside, because
// a gate that cannot answer must be able to stop a session overrunning and must not be
// able to stop it working.
function ask(payload, cwd) {
  const bin = process.env.LOCALCODE_BIN || "localcode";
  const run = spawnSync(bin, ["hook", "gate"], {
    input: JSON.stringify(payload),
    encoding: "utf8",
    cwd,
  });
  if (run.status !== 2) return null;
  return run.stderr.trim() || "localcode: this session's budget is spent.";
}

export default function (pi) {
  pi.on("tool_call", (event, ctx) => {
    const reason = ask(
      {
        session_id: ctx.sessionManager.getSessionId(),
        tool_name: NAMES[event.toolName] ?? event.toolName,
        tool_input: toolInput(event.input, ctx.cwd),
        peak_tokens: peakTokens(ctx),
      },
      ctx.cwd,
    );
    // Not terminating: a session at its ceiling still has a handoff to write, and ending
    // the run here would take that away from it.
    if (reason) return { block: true, reason };
  });
}
