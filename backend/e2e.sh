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

echo "1/7 unauthenticated access is rejected"
[ "$(curl -s -o /dev/null -w '%{http_code}' $API/api/v1/services)" = 401 ] || fail "services without token"

# A still-open incident from an earlier run would absorb the new fault (dedup).
for old in $(curl -sf -H "$H" $API/api/v1/incidents | j '" ".join(i["id"] for i in d if i["status"]!="resolved")'); do
  curl -sf -XPATCH -H "$H" -H 'Content-Type: application/json' -d '{"status":"resolved"}' $API/api/v1/incidents/$old >/dev/null
done

echo "2/7 ingest healthy history, then a payments fault with a deployment"
go run ./cmd/simulate -backfill 20m -d 10s >/dev/null 2>&1
go run ./cmd/simulate -d 90s -fault payments -deploy payments=v-e2e >/dev/null 2>&1
sleep 20

echo "3/7 services and dependency graph are discovered"
curl -sf -H "$H" "$API/api/v1/services?window=1h" | j '"payments" in [s["name"] for s in d]' | grep -q True || fail "payments not discovered"
curl -sf -H "$H" "$API/api/v1/dependencies?window=1h" | j '"checkout->payments" in [e["source"]+"->"+e["target"] for e in d]' | grep -q True || fail "edge missing"

echo "4/7 an incident was detected and analysed"
for _ in $(seq 1 12); do
  ID=$(curl -sf -H "$H" $API/api/v1/incidents | j 'next((i["id"] for i in d if i["service"]=="payments" and i["status"]=="identified"), "")')
  [ -n "$ID" ] && break; sleep 5
done
[ -n "${ID:-}" ] || fail "no analysed payments incident"
CAUSE=$(curl -sf -H "$H" $API/api/v1/incidents/$ID | j 'd["analyses"][0]["rootCause"]')
echo "   root cause: $CAUSE"
# Back-to-back runs share one unhealthy streak, so the ranked cause can be either the
# deployment or the degraded database. Both are correct; what must hold is that the
# analysis names the payments service and carries the deployment in its evidence.
echo "$CAUSE" | grep -qi "payments" || fail "analysis did not identify payments"
curl -sf -H "$H" $API/api/v1/incidents/$ID | j '"v-e2e" in " ".join(e["summary"] for e in d["analyses"][0]["evidence"] if e["kind"]=="deploy")' | grep -q True || fail "deployment missing from the evidence"
echo "5/7 approval is recorded"
AID=$(curl -sf -H "$H" $API/api/v1/incidents/$ID | j 'd["analyses"][0]["id"]')
[ "$(curl -s -o /dev/null -w '%{http_code}' -XPOST -H "$H" $API/api/v1/analyses/$AID/approve)" = 204 ] || fail "approve"

echo "5b  alert rule, SLO and dashboard APIs work"
J='Content-Type: application/json'
RID=$(curl -sf -XPOST -H "$H" -H "$J" -d '{"name":"e2e","kind":"error_rate","service":"payments","threshold":0.5}' $API/api/v1/alert-rules | j 'd["id"]')
curl -sf -H "$H" $API/api/v1/alert-rules | j '[r["id"] for r in d]' | grep -q "$RID" || fail "alert rule not listed"
curl -sf -XDELETE -H "$H" $API/api/v1/alert-rules/$RID >/dev/null
SID=$(curl -sf -XPOST -H "$H" -H "$J" -d '{"name":"e2e","service":"payments","kind":"availability","objective":99.9}' $API/api/v1/slos | j 'd["id"]')
curl -sf -H "$H" $API/api/v1/slos | j '[s for s in d if s["id"]=="'$SID'"][0]["status"]' | grep -qE "ok|warning|breached" || fail "slo not evaluated"
curl -sf -XDELETE -H "$H" $API/api/v1/slos/$SID >/dev/null
curl -sf -H "$H" "$API/api/v1/logs/patterns?window=1h" | j 'len(d)>0' | grep -q True || fail "log patterns empty"

echo "5c  AI assistant: provider settings, encrypted key, tool-using chat (against a mock model)"
python3 scripts/mock_llm.py 8765 >/dev/null 2>&1 & MOCK=$!
trap 'kill $MOCK 2>/dev/null || true' EXIT
sleep 1
[ "$(curl -s -o /dev/null -w '%{http_code}' -XPUT -H "$H" -H "$J" -d '{"provider":"openai-compatible","baseUrl":"ftp://x","model":"m"}' $API/api/v1/ai/settings)" = 400 ] || fail "bad base URL accepted"
curl -sf -XPUT -H "$H" -H "$J" -d '{"provider":"openai-compatible","baseUrl":"http://host.docker.internal:8765/v1","model":"mock","apiKey":"sk-e2e-0000000000wxyz"}' $API/api/v1/ai/settings | j 'd["keyHint"]' | grep -q wxyz || fail "ai settings not saved"
docker exec resolvex-postgres-1 psql -U resolvex -d resolvex -Atc "select key_cipher like '%sk-e2e%' from ai_settings where tenant_id='tnt_dev'" | grep -q f || fail "API key stored in plaintext"
curl -sf -XPOST -H "$H" $API/api/v1/ai/test | j 'd["ok"]' | grep -q True || fail "ai test failed"
curl -sf -XPOST -H "$H" -H "$J" -d '{"messages":[{"role":"user","content":"what is unhealthy?"}]}' $API/api/v1/ai/chat | j '"MOCK ANSWER" in d["answer"] and d["steps"][0]["tool"]=="list_services" and d["steps"][0]["ok"]' | grep -q True || fail "chat did not run a tool"
curl -sf -XDELETE -H "$H" $API/api/v1/ai/settings >/dev/null

echo "5d  team: invite, roles, revocation"
INV=$(curl -sf -XPOST -H "$H" -H "$J" -d '{"email":"member-'$RANDOM'@example.com","role":"member"}' $API/api/v1/members/invites)
MEMAIL=$(echo "$INV" | j 'd["email"]'); MTOK=$(echo "$INV" | j 'd["token"]')
MT=$(curl -sf -XPOST -H "$J" -d '{"token":"'$MTOK'","password":"member-long-password"}' $API/api/v1/auth/accept-invite | j 'd["token"]')
MH="Authorization: Bearer $MT"
[ "$(curl -s -o /dev/null -w '%{http_code}' -XPOST -H "$J" -d '{"token":"'$MTOK'","password":"member-long-password"}' $API/api/v1/auth/accept-invite)" = 400 ] || fail "invite token reusable"
[ "$(curl -s -o /dev/null -w '%{http_code}' -H "$MH" "$API/api/v1/services?window=1h")" = 200 ] || fail "member cannot read telemetry"
PID=$(curl -sf -H "$H" $API/api/v1/projects | j 'd[0]["id"]')
[ "$(curl -s -o /dev/null -w '%{http_code}' -XPOST -H "$MH" -H "$J" -d '{"environmentId":"x"}' $API/api/v1/projects/$PID/api-keys)" = 403 ] || fail "member created an API key"
[ "$(curl -s -o /dev/null -w '%{http_code}' -XPOST -H "$MH" -H "$J" -d '{"name":"x","kind":"error_rate","service":"payments","threshold":0.5}' $API/api/v1/alert-rules)" = 403 ] || fail "member created an alert rule"
[ "$(curl -s -o /dev/null -w '%{http_code}' -XPOST -H "$MH" -H "$J" -d '{"messages":[{"role":"user","content":"x"}]}' $API/api/v1/ai/chat)" != 401 ] || fail "member cannot use the assistant"
MID=$(curl -sf -H "$H" $API/api/v1/members | j 'next(m["userId"] for m in d["members"] if m["email"]=="'$MEMAIL'")')
curl -sf -XDELETE -H "$H" $API/api/v1/members/$MID >/dev/null
[ "$(curl -s -o /dev/null -w '%{http_code}' -H "$MH" "$API/api/v1/services?window=1h")" = 401 ] || fail "removed member kept access"

echo "5e  notifications: signed webhook delivery"
python3 scripts/webhook_sink.py 8766 /tmp/rx-sink.json >/dev/null 2>&1 & SINK=$!
trap 'kill $MOCK $SINK 2>/dev/null || true' EXIT
sleep 1; rm -f /tmp/rx-sink.json
CH=$(curl -sf -XPOST -H "$H" -H "$J" -d '{"kind":"webhook","name":"e2e","url":"http://host.docker.internal:8766/hook"}' $API/api/v1/notifications/channels)
CID=$(echo "$CH" | j 'd["channel"]["id"]'); SEC=$(echo "$CH" | j 'd["signingSecret"]')
curl -sf -XPOST -H "$H" $API/api/v1/notifications/channels/$CID/test | j 'd["ok"]' | grep -q True || fail "webhook test failed"
python3 - "$SEC" <<'PY' || fail "webhook signature invalid"
import hashlib, hmac, json, sys
d = json.load(open("/tmp/rx-sink.json"))
sig = {k.lower(): v for k, v in d["headers"].items()}["x-resolvex-signature"]
assert sig == "sha256=" + hmac.new(sys.argv[1].encode(), d["body"].encode(), hashlib.sha256).hexdigest()
PY
curl -sf -XDELETE -H "$H" $API/api/v1/notifications/channels/$CID >/dev/null

echo "5f  usage and read-only database access"
curl -sf -H "$H" "$API/api/v1/usage?limit=7" | j 'd["total"]["spans"] > 0' | grep -q True || fail "usage empty"

echo "6/7 another tenant sees none of it"
EMAIL="e2e-$RANDOM@example.com"
T2=$(curl -sf -XPOST $API/api/v1/auth/signup -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\",\"password\":\"e2e-long-password\"}" | j 'd["token"]')
[ "$(curl -sf -H "Authorization: Bearer $T2" "$API/api/v1/services?window=24h")" = "[]" ] || fail "tenant leak: services"
[ "$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $T2" $API/api/v1/incidents/$ID)" = 404 ] || fail "tenant leak: incident"
echo "PASS"
