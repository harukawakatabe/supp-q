#!/bin/sh
set -eu

BASE_URL=${SUPPQ_BASE_URL:-http://127.0.0.1:3000}
COMPOSE_FILE=${SUPPQ_COMPOSE_FILE:-deploy/compose.dev.yml}
DOCKER_COMPOSE_BIN=${DOCKER_COMPOSE_BIN:-docker-compose}

cookie_file=$(mktemp)
response_file=$(mktemp)
trap 'rm -f "$cookie_file" "$response_file"' EXIT HUP INT TERM

curl_json() {
  curl --fail --silent --show-error \
    --cookie "$cookie_file" \
    --cookie-jar "$cookie_file" \
    -H 'Content-Type: application/json' \
    "$@"
}

curl_json "$BASE_URL/api/v1/session" >"$response_file"
user_id=$(jq -er '.actor.userId' "$response_file")
curl_json "$BASE_URL/api/v1/products" >"$response_file"
product_id=$(jq -er '.items[0].id' "$response_file")
before_quantity=$(jq -er '.items[0].currentQuantity' "$response_file")
today=$(date +%F)
idempotency_key="restart-persistence:${user_id}:${today}:$$"

payload=$(jq -nc \
  --arg productId "$product_id" \
  --arg date "$today" \
  '{productId:$productId,date:$date,time:"12:00",quantity:1,source:"ad_hoc",note:"restart persistence acceptance"}')
curl_json \
  -X POST \
  -H "Idempotency-Key: $idempotency_key" \
  --data "$payload" \
  "$BASE_URL/api/v1/intakes" >"$response_file"
intake_id=$(jq -er '.intake.id' "$response_file")
persisted_quantity=$(jq -er '.product.currentQuantity' "$response_file")

"$DOCKER_COMPOSE_BIN" -f "$COMPOSE_FILE" restart postgres api worker >/dev/null

attempt=0
until curl --fail --silent "$BASE_URL/api/v1/health/ready" >"$response_file"; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 40 ]; then
    printf '%s\n' 'readiness did not recover within 80 seconds' >&2
    exit 1
  fi
  sleep 2
done

curl_json "$BASE_URL/api/v1/session" >"$response_file"
after_user_id=$(jq -er '.actor.userId' "$response_file")
test "$after_user_id" = "$user_id"

curl_json "$BASE_URL/api/v1/products" >"$response_file"
after_quantity=$(jq -er --arg id "$product_id" '.items[] | select(.id == $id) | .currentQuantity' "$response_file")
test "$after_quantity" = "$persisted_quantity"

curl_json -X DELETE "$BASE_URL/api/v1/intakes/$intake_id" >"$response_file"
restored_quantity=$(jq -er '.product.currentQuantity' "$response_file")
test "$restored_quantity" = "$before_quantity"

printf 'restart persistence passed: user=%s product=%s quantity=%s restored=%s\n' \
  "$user_id" "$product_id" "$after_quantity" "$restored_quantity"
