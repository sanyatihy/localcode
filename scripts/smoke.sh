#!/usr/bin/env bash
# Assert a served endpoint is actually usable: it answers, and it round-trips a
# tool call as valid JSON matching the schema it was given. Malformed tool calls
# are the dominant local-model failure, so a server that chats but cannot call a
# tool is broken for this project's purpose and must fail here.
#
# Thinking is disabled for these two requests to keep the gate fast and its output
# deterministic. Whether thinking helps or hurts is 0005's axis, not a smoke test's.
set -euo pipefail

ENDPOINT="${ENDPOINT:-http://127.0.0.1:8080}"

fail() { echo "SMOKE FAIL: $*" >&2; exit 1; }

curl -s -m 10 "$ENDPOINT/health" | grep -q '"ok"' || fail "no healthy server at $ENDPOINT"
echo "ok   health"

chat=$(curl -s -m 300 "$ENDPOINT/v1/chat/completions" \
  -H 'Content-Type: application/json' -d '{
    "messages":[{"role":"user","content":"Reply with exactly: pong"}],
    "chat_template_kwargs":{"enable_thinking":false},
    "max_tokens":32,"temperature":0}') || fail "chat request failed"

python3 -c "
import json,sys
d=json.loads(sys.argv[1])
if 'error' in d: sys.exit('server error: %s' % str(d['error'])[:200])
c=(d['choices'][0]['message'].get('content') or '').strip()
if not c: sys.exit('empty content — model produced no answer')
print('ok   chat completion: %r' % c[:40])
" "$chat" || fail "chat completion assertion failed"

tool=$(curl -s -m 300 "$ENDPOINT/v1/chat/completions" \
  -H 'Content-Type: application/json' -d '{
    "messages":[
      {"role":"system","content":"You are a coding agent. Use the tool. Do not explain."},
      {"role":"user","content":"Read the file src/auth/session.go so you can inspect it."}],
    "tools":[{"type":"function","function":{
      "name":"read_file","description":"Read a file from the repository",
      "parameters":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}}}],
    "tool_choice":"auto",
    "chat_template_kwargs":{"enable_thinking":false},
    "max_tokens":256,"temperature":0}') || fail "tool request failed"

python3 -c "
import json,sys
d=json.loads(sys.argv[1])
if 'error' in d: sys.exit('server error: %s' % str(d['error'])[:200])
m=d['choices'][0]['message']
tc=m.get('tool_calls')
if not tc: sys.exit('no tool_calls returned — model answered in prose instead of calling the tool')
f=tc[0]['function']
if f['name'] != 'read_file': sys.exit('wrong tool called: %s' % f['name'])
try: args=json.loads(f['arguments'])
except Exception as e: sys.exit('tool arguments are not valid JSON: %s' % e)
if 'path' not in args: sys.exit('tool arguments miss the required field \'path\': %r' % args)
print('ok   tool round-trip: %s(%s)' % (f['name'], json.dumps(args)))
" "$tool" || fail "tool-call assertion failed"

echo "smoke passed"
