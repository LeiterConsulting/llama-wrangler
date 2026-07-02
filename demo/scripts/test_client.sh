#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:11435}"

curl -s "$BASE_URL/api/chat" \
  -H 'Content-Type: application/json' \
  -d '{"model":"local-fast","stream":false,"messages":[{"role":"user","content":"Say hello from Llama Wrangler."}]}'

curl -s "$BASE_URL/v1/chat/completions" \
  -H 'Content-Type: application/json' \
  -H 'X-Llama-Wrangler-Session: demo-code-task' \
  -d '{"model":"local-code","stream":false,"messages":[{"role":"user","content":"Write a tiny Go function that adds two ints."}]}'

curl -s "$BASE_URL/v1/chat/completions" \
  -H 'Content-Type: application/json' \
  -H 'X-Llama-Wrangler-Session: demo-consensus-task' \
  -d '{"model":"local-consensus","stream":false,"messages":[{"role":"user","content":"Return JSON with one key named status and value ok."}]}'
