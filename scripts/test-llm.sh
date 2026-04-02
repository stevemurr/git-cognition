#!/usr/bin/env bash
set -euo pipefail

# Test the LLM reasoning extraction pipeline against a live endpoint.
# Builds a fake session, sends it through the extraction, and validates
# the structured output.
#
# Usage: test-llm.sh [ENDPOINT] [MODEL]
#
# Environment:
#   GC_LLM_ENDPOINT  LLM endpoint URL (required)
#   GC_LLM_API_KEY   Bearer token (required)
#   GC_LLM_MODEL     Model name (default: nemotron3-nano)

ENDPOINT="${1:-${GC_LLM_ENDPOINT:-}}"
MODEL="${2:-${GC_LLM_MODEL:-nemotron3-nano}}"
API_KEY="${GC_LLM_API_KEY:-}"

if [ -z "$ENDPOINT" ] || [ -z "$API_KEY" ]; then
    echo "Error: GC_LLM_ENDPOINT and GC_LLM_API_KEY must be set" >&2
    echo "Usage: GC_LLM_ENDPOINT=http://... GC_LLM_API_KEY=sk-... $0 [MODEL]" >&2
    exit 1
fi

PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== LLM Extraction Tests ==="
echo "  endpoint: $ENDPOINT"
echo "  model:    $MODEL"
echo ""

# --- Test 1: Endpoint reachable ---
echo "--- Test 1: Endpoint reachable ---"
# Try /health (litellm), then /v1/models (vllm/openai), then bare GET
if curl -4sf --max-time 5 "$ENDPOINT/health" > /dev/null 2>&1; then
    pass "endpoint responds (/health)"
elif curl -4sf --max-time 5 "$ENDPOINT/v1/models" > /dev/null 2>&1; then
    pass "endpoint responds (/v1/models)"
elif curl -4sf --max-time 5 "$ENDPOINT/" > /dev/null 2>&1; then
    pass "endpoint responds (/)"
else
    fail "endpoint unreachable at $ENDPOINT"
    echo "  tried: /health, /v1/models, /"
    echo ""
    echo "Cannot continue without a reachable endpoint."
    echo "Results: $PASS passed, $FAIL failed"
    exit 1
fi

# --- Test 2: Chat completions basic request ---
echo "--- Test 2: Basic chat completion ---"
BASIC_RESP=$(curl -4sf --max-time 30 \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $API_KEY" \
    "$ENDPOINT/v1/chat/completions" \
    -d "{
        \"model\": \"$MODEL\",
        \"messages\": [{\"role\": \"user\", \"content\": \"Say hello in one word.\"}],
        \"temperature\": 0.1
    }" 2>&1) || true

if echo "$BASIC_RESP" | grep -q '"choices"'; then
    pass "chat completions returns choices"
else
    fail "chat completions failed: $BASIC_RESP"
fi

# --- Test 3: Structured output with json_schema ---
echo "--- Test 3: Structured output (json_schema) ---"
SCHEMA_RESP=$(curl -4sf --max-time 60 \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $API_KEY" \
    "$ENDPOINT/v1/chat/completions" \
    -d '{
        "model": "'"$MODEL"'",
        "messages": [
            {"role": "system", "content": "You are a code reasoning extractor. Produce structured JSON."},
            {"role": "user", "content": "## Task\nAdd a hello world function\n\n## Commits\n- abc1234 feat: add hello (files: hello.go)\n\n## Tool Sequence\n1. Write hello.go\n2. Bash: go build\n\n## Final Message\nI created hello.go with a simple Hello function.\n\nExtract the reasoning for this session."}
        ],
        "temperature": 0.1,
        "response_format": {
            "type": "json_schema",
            "json_schema": {
                "name": "reasoning_extraction",
                "strict": true,
                "schema": {
                    "type": "object",
                    "properties": {
                        "summary": {"type": "string"},
                        "file_annotations": {
                            "type": "array",
                            "items": {
                                "type": "object",
                                "properties": {
                                    "path": {"type": "string"},
                                    "what": {"type": "string"},
                                    "why": {"type": "string"}
                                },
                                "required": ["path", "what", "why"],
                                "additionalProperties": false
                            }
                        },
                        "rejected_approaches": {"type": "array", "items": {"type": "string"}},
                        "key_decisions": {"type": "array", "items": {"type": "string"}}
                    },
                    "required": ["summary", "file_annotations", "rejected_approaches", "key_decisions"],
                    "additionalProperties": false
                }
            }
        }
    }' 2>&1) || true

# Extract the content from the response
CONTENT=$(echo "$SCHEMA_RESP" | python3 -c "
import sys, json
try:
    r = json.load(sys.stdin)
    print(r['choices'][0]['message']['content'])
except Exception as e:
    print(f'PARSE_ERROR: {e}', file=sys.stderr)
    sys.exit(1)
" 2>/dev/null) || CONTENT=""

if [ -z "$CONTENT" ]; then
    fail "no content in structured output response"
    echo "  raw response: $(echo "$SCHEMA_RESP" | head -5)"
else
    pass "structured output returned content"
fi

# --- Test 4: Validate JSON structure ---
echo "--- Test 4: Validate JSON structure ---"
if [ -n "$CONTENT" ]; then
    VALID=$(echo "$CONTENT" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    required = ['summary', 'file_annotations', 'rejected_approaches', 'key_decisions']
    missing = [k for k in required if k not in d]
    if missing:
        print(f'MISSING: {missing}')
        sys.exit(1)
    if not isinstance(d['summary'], str) or len(d['summary']) == 0:
        print('BAD_SUMMARY')
        sys.exit(1)
    if not isinstance(d['file_annotations'], list):
        print('BAD_FILE_ANNOTATIONS')
        sys.exit(1)
    for ann in d['file_annotations']:
        for k in ['path', 'what', 'why']:
            if k not in ann:
                print(f'MISSING_ANNOTATION_FIELD: {k}')
                sys.exit(1)
    print('OK')
except json.JSONDecodeError as e:
    print(f'INVALID_JSON: {e}')
    sys.exit(1)
" 2>/dev/null) || VALID="ERROR"

    if [ "$VALID" = "OK" ]; then
        pass "JSON has all required fields with correct types"
    else
        fail "JSON validation: $VALID"
    fi
else
    fail "skipped (no content from previous test)"
fi

# --- Test 5: Content quality ---
echo "--- Test 5: Content quality ---"
if [ -n "$CONTENT" ]; then
    echo "$CONTENT" | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(f'  summary:    {d[\"summary\"][:80]}')
print(f'  files:      {len(d[\"file_annotations\"])} annotations')
for a in d['file_annotations']:
    print(f'              {a[\"path\"]}: {a[\"what\"][:60]}')
print(f'  rejected:   {len(d[\"rejected_approaches\"])} approaches')
print(f'  decisions:  {len(d[\"key_decisions\"])} decisions')
" 2>/dev/null && pass "content looks reasonable" || fail "content inspection failed"
else
    fail "skipped (no content)"
fi

# --- Test 6: Go unit tests ---
echo "--- Test 6: Go unit tests (internal/llm) ---"
cd "$(dirname "$0")/.."
if go test ./internal/llm/ -v -count=1 2>&1 | tail -5; then
    pass "Go unit tests pass"
else
    fail "Go unit tests failed"
fi

# --- Summary ---
echo ""
echo "══════════════════════════════════════"
echo "  Results: $PASS passed, $FAIL failed"
echo "══════════════════════════════════════"

[ "$FAIL" -eq 0 ] || exit 1
