// Registers the local llama-server as a Pi provider.
//
// Kept in the repo and loaded with `pi -e harness/pi/local-provider.js` rather than
// installed into ~/.pi/extensions, so a run is reproducible from a checkout and does
// not depend on a machine having been set up by hand.
//
// Pi ignores OPENAI_BASE_URL: pointing it at a local server without this file sends
// the request to api.openai.com and returns 401.
const SERVER = "http://127.0.0.1:8081";

// The context is read from the server rather than written down here, because no literal
// is right for both configs this repo serves: the harness comparison runs at 65,536 and
// the agent flow at 49,152. `default_generation_settings.n_ctx` is the per-slot context —
// what one session may actually use — and is the field `localcode` already declares
// Claude Code's window from.
async function servedContext() {
  let props;
  try {
    const resp = await fetch(`${SERVER}/props`, { signal: AbortSignal.timeout(3000) });
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    props = await resp.json();
  } catch (cause) {
    throw new Error(
      `could not read ${SERVER}/props: ${cause.message}` +
        " — start one with `make serve CONFIG=config/agent.env`",
    );
  }
  const nCtx = props?.default_generation_settings?.n_ctx;
  if (!(nCtx > 0)) throw new Error(`${SERVER}/props reports no n_ctx, so the served context is unknown`);
  return nCtx;
}

// Pi awaits the factory before startup continues, so the fetched window is in place for
// an interactive session and for `pi --list-models` alike.
export default async function (pi) {
  pi.registerProvider("local", {
    baseUrl: `${SERVER}/v1`,
    // llama-server does not check the key, but Pi expects the field. The env var is
    // named so a real gateway could be dropped in without editing this file.
    apiKey: "$LOCAL_OPENAI_API_KEY",
    api: "openai-completions",
    models: [
      {
        // Matches what the server reports at /v1/models, so the id in a result row
        // and the id on the wire are the same string.
        id: "bartowski/Qwen3.8-27B-GGUF:Q4_K_M",
        name: "Qwen3.8-27B Q4_K_M (local)",
        reasoning: true,
        input: ["text"],
        cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
        // The whole of it: prompt and reply share the served context, and what a session
        // may spend inside it is 0038's gate rather than a slice withheld here.
        contextWindow: await servedContext(),
        maxTokens: 4096,
      },
    ],
  });
}
