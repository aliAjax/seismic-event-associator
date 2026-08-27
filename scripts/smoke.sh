#!/bin/sh
set -eu
BASE_URL="${BASE_URL:-http://127.0.0.1:18336}"
API_KEY="${SEISMIC_API_KEY:-seismic-dev}"
curl -fsS "$BASE_URL/healthz"
curl -fsS "$BASE_URL/readyz"
curl -fsS -H "X-API-Key: $API_KEY" "$BASE_URL/v1/stations"
go run ./cmd/wave-simulator -url "$BASE_URL" -api-key "$API_KEY"
curl -fsS -H "X-API-Key: $API_KEY" "$BASE_URL/v1/picks"
curl -fsS -H "X-API-Key: $API_KEY" "$BASE_URL/v1/events"
curl -fsS "$BASE_URL/metrics" | grep seismic_events_total

