// Registers the local llama-server as a Pi provider.
//
// Kept in the repo and loaded with `pi -e harness/pi/local-provider.js` rather than
// installed into ~/.pi/extensions, so a run is reproducible from a checkout and does
// not depend on a machine having been set up by hand.
//
// Pi ignores OPENAI_BASE_URL: pointing it at a local server without this file sends
// the request to api.openai.com and returns 401.
export default function (pi) {
  pi.registerProvider("local", {
    baseUrl: "http://127.0.0.1:8080/v1",
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
        contextWindow: 32768,
        maxTokens: 4096,
      },
    ],
  });
}
