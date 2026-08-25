// Holds a Pi session to the budget `localcode` gave it: a tool call refused at the
// ceiling, a read narrowed to the result cap, a compaction cancelled, and an end refused
// while the session has handed nothing on.
//
// Loaded beside the provider with a second `-e`. Nothing is decided here: the payload is
// filled in from Pi's vocabulary, `localcode hook gate` and `localcode hook stop` answer,
// and the verdict is returned in Pi's own terms. Those are the binary and the code the
// Claude Code hooks run, so the two harnesses cannot drift into two answers about what a
// session may spend.
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

// The inverse of `toolInput`: a call the gate narrowed comes back in the gate's names.
function fromGate(updated) {
  const out = { ...updated };
  if (typeof updated.file_path === "string") {
    out.path = updated.file_path;
    delete out.file_path;
  }
  return out;
}

// One process per decision, which is what the Claude Code hooks already cost.
function hook(name, payload, cwd) {
  const bin = process.env.LOCALCODE_BIN || "localcode";
  return spawnSync(bin, ["hook", name], { input: JSON.stringify(payload), encoding: "utf8", cwd });
}

// Exit 2 is the refusal and stderr is the reason the model is given; a permitted call the
// gate held to the result cap comes back on stdout. Anything else stands aside, because a
// gate that cannot answer must be able to stop a session overrunning and must not be able
// to stop it working.
function ask(payload, cwd) {
  const run = hook("gate", payload, cwd);
  if (run.status === 2) {
    return { reason: run.stderr.trim() || "localcode: this session's budget is spent." };
  }
  if (run.status !== 0 || !run.stdout.trim()) return {};
  try {
    const updated = JSON.parse(run.stdout).hookSpecificOutput?.updatedInput;
    return updated ? { input: fromGate(updated) } : {};
  } catch {
    return {}; // an answer this cannot read is one it did not get
  }
}

export default function (pi) {
  // A chain hands off; it does not compact. Cancelling is a refusal rather than a
  // threshold, so it holds whatever `compaction.reserveTokens` the session was served —
  // and it covers the manual and overflow triggers, which no reserve does.
  pi.on("session_before_compact", () => ({ cancel: true }));

  // A session that has handed nothing on is not finished. The same `chain.Stop` the Stop
  // hook asks answers here, refusals and grace included, and a refusal is delivered as a
  // follow-up message: Pi continues on what an `agent_end` handler queues, which is the
  // only thing here that keeps a session going.
  pi.on("agent_end", (_event, ctx) => {
    const run = hook("stop", { session_id: ctx.sessionManager.getSessionId() }, ctx.cwd);
    if (run.status !== 2) return;
    const reason = run.stderr.trim() || "localcode: this session has handed nothing on.";
    pi.sendUserMessage(reason, { deliverAs: "followUp" });
  });

  pi.on("tool_call", (event, ctx) => {
    const verdict = ask(
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
    if (verdict.reason) return { block: true, reason: verdict.reason };
    // Narrowed rather than refused: the session reads what it asked for, up to what the
    // reserve holds for one call. Pi takes the patched arguments as they stand here.
    if (verdict.input) Object.assign(event.input, verdict.input);
  });
}
