#!/usr/bin/env bash
# Smoke-test enabled third-party models for web-search routing latency and API errors.
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:11435}"

models=(
  "provider:563c84f7526d4817|kimi-k2.6"
  "provider:be484d9aaa2b91d3|deepseek-v4-pro"
  "provider:f6559ecf2feaf319|glm-5"
  "provider:7ce777a82d062500|mimo-v2.5-pro"
  "provider:16353efc44fb277c|MiniMax-M2.5"
)

query='{"model":"MODEL","source":"SOURCE","messages":[{"role":"user","content":"今天上海天气怎么样"}],"stream":true,"web_search":{"enabled":true},"options":{"max_tokens":256,"temperature":0.1}}'

printf "%-28s %8s %s\n" "model" "seconds" "status"
printf "%-28s %8s %s\n" "----" "-------" "------"

for entry in "${models[@]}"; do
  source="${entry%%|*}"
  model="${entry##*|}"
  body="${query//MODEL/$model}"
  body="${body//SOURCE/$source}"
  start=$(date +%s.%N)
  if out=$(curl -sS -N -m 120 \
    -H 'Content-Type: application/json' \
    -H 'Accept: text/event-stream' \
    -H 'X-CSGHUB-Stream: sse' \
    -d "$body" \
    "$BASE_URL/api/chat" 2>&1); then
    end=$(date +%s.%N)
    elapsed=$(python3 - <<PY
import decimal
print(f"{decimal.Decimal('$end') - decimal.Decimal('$start'):.2f}")
PY
)
    if grep -q '"search_error"' <<<"$out"; then
      status="search_error"
    elif grep -q '"action":"search"' <<<"$out"; then
      status="ok_search"
    elif grep -q '"action":"skip"' <<<"$out"; then
      status="ok_skip"
    elif grep -q '"message"' <<<"$out"; then
      status="ok_reply"
    else
      status="unknown"
    fi
    printf "%-28s %8s %s\n" "$model" "$elapsed" "$status"
  else
    printf "%-28s %8s %s\n" "$model" "-" "curl_failed"
  fi
done
