#!/bin/bash
set -e

# Default URL, can be overridden by environment variable
BASE_URL=${BASE_URL:-"https://nevup.apnadomain.qzz.io"}
USER_ID="f412f236-4edc-47a2-8f54-8763a6ed2ce8"
OTHER_USER_ID="a1b2c3d4-e5f6-7890-abcd-ef1234567890"

echo "=========================================="
echo " NevUp Track 1 - Remote Verification Test"
echo " Target: $BASE_URL"
echo "=========================================="

echo "1. Generating JWT Token for User: $USER_ID"
JWT_TOKEN=$(go run cmd/jwtgen/main.go "$USER_ID")
echo "Token generated: ${JWT_TOKEN:0:15}..."

echo -e "\n2. Verifying Health Endpoint (No Auth)"
# The PS requires /health to return 200 without auth
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health")
if [ "$HTTP_CODE" -eq 200 ]; then 
  echo "✅ Health check passed (200 OK)"
else 
  echo "❌ Health check failed (Got $HTTP_CODE, expected 200)"
fi

echo -e "\n3. Testing Tenancy Rules (Cross-Tenant Access)"
# The PS requires a strict 403 (never 404/200) when accessing another user's data
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $JWT_TOKEN" "$BASE_URL/users/$OTHER_USER_ID/profile")
if [ "$HTTP_CODE" -eq 403 ]; then 
  echo "✅ Tenancy check passed (403 Forbidden for cross-tenant)"
else 
  echo "❌ Tenancy check failed (Got $HTTP_CODE, expected 403)"
fi

echo -e "\n4. Testing Metrics Endpoint (Valid Access)"
# Querying metrics endpoint with required query params
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $JWT_TOKEN" "$BASE_URL/users/$USER_ID/metrics?from=2025-01-01T00:00:00Z&to=2025-03-31T23:59:59Z&granularity=daily")
if [ "$HTTP_CODE" -eq 200 ]; then 
  echo "✅ Metrics successfully fetched (200 OK)"
else 
  echo "❌ Metrics fetch failed (Got $HTTP_CODE, expected 200)"
fi

echo -e "\n5. Running k6 Load Tests against Remote..."
# Runs the loadtest script matching latency constraints (<150ms)
k6 run --env BASE_URL="$BASE_URL" --env JWT_TOKEN="$JWT_TOKEN" loadtest/trade.js

echo -e "\n✅ Verification Complete!"
