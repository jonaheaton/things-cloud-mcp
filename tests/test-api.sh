#!/usr/bin/env bash
set -euo pipefail

# REST API endpoint tests — verifies /api/* endpoints with auth.
# Usage: ./test-api.sh [base_url] [api_key]
# The base URL can also be set via BASE_URL env var.
# The api_key can also be set via API_KEY env var.

BASE="${1:-${BASE_URL:-}}"
API_KEY="${2:-${API_KEY:-}}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LOG_FILE="${SCRIPT_DIR}/test-results.log"

if [ -z "$BASE" ]; then
  echo "Error: base URL is required. Pass as the first arg or set BASE_URL env var."
  exit 1
fi

if [ -z "$API_KEY" ]; then
  echo "Error: API_KEY is required. Pass as second arg or set API_KEY env var."
  exit 1
fi

AUTH_HEADER="Authorization: Bearer ${API_KEY}"

PASS=0
FAIL=0
FAILURES=""
CREATED_UUID=""

# --- helpers ---

api_get() {
  local path="$1"
  sleep 1
  curl -s --max-time 30 "${BASE}${path}" -H "$AUTH_HEADER"
}

api_get_no_auth() {
  local path="$1"
  sleep 1
  curl -s --max-time 30 -o /dev/null -w "%{http_code}" "${BASE}${path}"
}

api_get_status() {
  local path="$1"
  sleep 1
  curl -s --max-time 30 -o /dev/null -w "%{http_code}" "${BASE}${path}" -H "$AUTH_HEADER"
}

api_post() {
  local path="$1" body="$2"
  sleep 1
  curl -s --max-time 30 "${BASE}${path}" \
    -X POST \
    -H "Content-Type: application/json" \
    -H "$AUTH_HEADER" \
    -d "$body"
}

api_post_status() {
  local path="$1" body="$2"
  sleep 1
  curl -s --max-time 30 -o /dev/null -w "%{http_code}" "${BASE}${path}" \
    -X POST \
    -H "Content-Type: application/json" \
    -H "$AUTH_HEADER" \
    -d "$body"
}

api_method_status() {
  local method="$1" path="$2"
  sleep 1
  curl -s --max-time 30 -o /dev/null -w "%{http_code}" "${BASE}${path}" \
    -X "$method" \
    -H "$AUTH_HEADER"
}

json_field() {
  local json="$1" key="$2"
  echo "$json" | python3 -c "import sys,json; print(json.loads(sys.stdin.read()).get('$key',''))" 2>/dev/null || echo ""
}

is_array() {
  local json="$1"
  echo "$json" | python3 -c "import sys,json; print('true' if isinstance(json.loads(sys.stdin.read()), list) else 'false')" 2>/dev/null || echo "false"
}

check() {
  local label="$1" got="$2" want="$3"
  if [ "$got" = "$want" ]; then
    printf "  \033[32mPASS\033[0m  %s\n" "$label"
    PASS=$((PASS + 1))
  else
    printf "  \033[31mFAIL\033[0m  %s (got: %s, want: %s)\n" "$label" "$got" "$want"
    FAIL=$((FAIL + 1))
    FAILURES="${FAILURES}    FAIL  ${label} (got: ${got}, want: ${want})\n"
  fi
}

# --- tests ---

echo ""
echo "=== REST API Tests ==="
echo "    Base: ${BASE}"
echo ""

# 1. Health check
echo "--- Health ---"
HEALTH=$(curl -s --max-time 10 "${BASE}/")
check "service status" "$(json_field "$HEALTH" status)" "ok"
check "service name" "$(json_field "$HEALTH" service)" "things-cloud-api"

# 2-3. Auth
echo "--- Auth ---"
check "no auth returns 401" "$(api_get_no_auth /api/verify)" "401"
check "with auth returns 200" "$(api_get_status /api/verify)" "200"

# 4-9. Read endpoints
echo "--- Read Endpoints ---"
RESP=$(api_get "/api/tasks/today")
check "today returns array" "$(is_array "$RESP")" "true"

RESP=$(api_get "/api/tasks/inbox")
check "inbox returns array" "$(is_array "$RESP")" "true"

RESP=$(api_get "/api/projects")
check "projects returns array" "$(is_array "$RESP")" "true"

RESP=$(api_get "/api/areas")
check "areas returns array" "$(is_array "$RESP")" "true"

RESP=$(api_get "/api/tags")
check "tags returns array" "$(is_array "$RESP")" "true"

RESP=$(api_get "/api/sync")
check "sync returns changes_count" "$([ -n "$(json_field "$RESP" changes_count)" ] && echo ok || echo '')" "ok"

# 10-13. Write endpoints
echo "--- Write Endpoints ---"
RESP=$(api_post "/api/tasks/create" '{"title":"[api-test] Task"}')
check "create returns status" "$(json_field "$RESP" status)" "created"
CREATED_UUID=$(json_field "$RESP" uuid)
check "create returns uuid" "$([ -n "$CREATED_UUID" ] && echo ok || echo '')" "ok"

RESP=$(api_post "/api/tasks/edit" "{\"uuid\":\"${CREATED_UUID}\",\"title\":\"[api-test] Task (edited)\"}")
check "edit returns status" "$(json_field "$RESP" status)" "updated"

RESP=$(api_post "/api/tasks/complete" "{\"uuid\":\"${CREATED_UUID}\"}")
check "complete returns status" "$(json_field "$RESP" status)" "completed"

RESP=$(api_post "/api/tasks/trash" "{\"uuid\":\"${CREATED_UUID}\"}")
check "trash returns status" "$(json_field "$RESP" status)" "trashed"

# 14-16. Validation
echo "--- Validation ---"
RESP=$(api_post "/api/tasks/create" '{}')
check "create no title returns error" "$(json_field "$RESP" error)" "title is required"

RESP=$(api_post "/api/tasks/complete" '{}')
check "complete no uuid returns error" "$(json_field "$RESP" error)" "uuid is required"

check "GET on create returns 405" "$(api_method_status GET /api/tasks/create)" "405"

# --- summary & log ---

RESULT="PASS"
if [ "$FAIL" -gt 0 ]; then
  RESULT="FAIL"
fi

echo ""
echo "=== Results: ${PASS} passed, ${FAIL} failed ==="
echo ""

{
  echo "$(date -u '+%Y-%m-%d %H:%M:%S UTC')  test-api  ${RESULT}  ${PASS} passed, ${FAIL} failed"
  if [ -n "$FAILURES" ]; then
    printf "%b" "$FAILURES"
  fi
} >> "$LOG_FILE"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
