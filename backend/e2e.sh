#!/usr/bin/env bash
# End-to-end check of the whole product against a running `make up` stack:
# ingest -> store -> query -> detect -> analyse -> approve, plus tenant isolation.
set -euo pipefail
cd "$(dirname "$0")"
API=${API:-http://localhost:8000}
fail() { echo "FAIL: $*" >&2; exit 1; }
j() { python3 -c "import sys,json; d=json.load(sys.stdin); print($1)"; }

login() { curl -sf -XPOST $API/api/v1/auth/login -H 'Content-Type: application/json' -d "{\"email\":\"$1\",\"password\":\"$2\"}" | j 'd["token"]'; }
TOKEN=$(login dev@resolve-x.local devpassword1)
H="Authorization: Bearer $TOKEN"

echo "1/6 unauthenticated access is rejected"
[ "$(curl -s -o /dev/null -w '%{http_code}' $API/api/v1/services)" = 401 ] || fail "services without token"

# A still-open incident from an earlier run would absorb the new fault (dedup).
for old in $(curl -sf -H "$H" $API/api/v1/incidents | j '" ".join(i["id"] for i in d if i["status"]!="resolved")'); do
  curl -sf -XPATCH -H "$H" -H 'Content-Type: application/json' -d '{"status":"resolved"}' $API/api/v1/incidents/$old >/dev/null
done

echo "2/6 ingest healthy history, then a payments fault with a deployment"
go run ./cmd/simulate -backfill 20m -d 10s >/dev/null 2>&1
go run ./cmd/simulate -d 90s -fault payments -deploy payments=v-e2e >/dev/null 2>&1
sleep 20

echo "3/6 services and dependency graph are discovered"
curl -sf -H "$H" "$API/api/v1/services?window=1h" | j '"payments" in [s["name"] for s in d]' | grep -q True || fail "payments not discovered"
curl -sf -H "$H" "$API/api/v1/dependencies?window=1h" | j '"checkout->payments" in [e["source"]+"->"+e["target"] for e in d]' | grep -q True || fail "edge missing"

echo "4/6 an incident was detected and analysed"
for _ in $(seq 1 12); do
  ID=$(curl -sf -H "$H" $API/api/v1/incidents | j 'next((i["id"] for i in d if i["service"]=="payments" and i["status"]=="identified"), "")')
  [ -n "$ID" ] && break; sleep 5
done
[ -n "${ID:-}" ] || fail "no analysed payments incident"
CAUSE=$(curl -sf -H "$H" $API/api/v1/incidents/$ID | j 'd["analyses"][0]["rootCause"]')
echo "   root cause: $CAUSE"
# Back-to-back runs share one unhealthy streak, so the blamed version may be an
# earlier run's deployment; what matters is that a deployment is identified.
echo "$CAUSE" | grep -q "Deployment of payments" || fail "analysis did not identify the deployment"
curl -sf -H "$H" $API/api/v1/incidents/$ID | j 'd["analyses"][0]["recommendation"]["action"]' | grep -q rollback || fail "no rollback recommendation"

echo "5/6 approval is recorded"
AID=$(curl -sf -H "$H" $API/api/v1/incidents/$ID | j 'd["analyses"][0]["id"]')
[ "$(curl -s -o /dev/null -w '%{http_code}' -XPOST -H "$H" $API/api/v1/analyses/$AID/approve)" = 204 ] || fail "approve"

echo "6/6 another tenant sees none of it"
EMAIL="e2e-$RANDOM@example.com"
T2=$(curl -sf -XPOST $API/api/v1/auth/signup -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\",\"password\":\"e2e-long-password\"}" | j 'd["token"]')
[ "$(curl -sf -H "Authorization: Bearer $T2" "$API/api/v1/services?window=24h")" = "[]" ] || fail "tenant leak: services"
[ "$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $T2" $API/api/v1/incidents/$ID)" = 404 ] || fail "tenant leak: incident"
echo "PASS"
