#!/usr/bin/env bash
#
# End-to-end integration test for Marginalia.
#
# Expects Marginalia running at BASE_URL (default: http://localhost:8080)
# with mock-readeck providing fixture data.
#
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
TOKEN="${TOKEN:-test-token-e2e}"
PASS=0
FAIL=0

# ── Helpers ─────────────────────────────────────────────────────────

header() {
  printf "\n\033[1;34m══ %s\033[0m\n" "$1"
}
pass() {
  printf "  \033[32m✓ %s\033[0m\n" "$1"
  PASS=$((PASS + 1))
}
fail() {
  printf "  \033[31m✗ %s\033[0m\n" "$1"
  if [[ -n "${2:-}" ]]; then
    printf "    \033[31m%s\033[0m\n" "$2"
  fi
  FAIL=$((FAIL + 1))
}

api() {
  local method="$1" path="$2"
  shift 2
  curl -sf -X "$method" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    "$BASE_URL$path" "$@"
}

assert_eq() {
  local label="$1" expected="$2" actual="$3"
  if [[ "$expected" == "$actual" ]]; then
    pass "$label"
  else
    fail "$label" "expected: $expected, got: $actual"
  fi
}

assert_contains() {
  local label="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    pass "$label"
  else
    fail "$label" "expected to contain: $needle"
  fi
}

assert_gt() {
  local label="$1" threshold="$2" actual="$3"
  if [[ "$actual" -gt "$threshold" ]]; then
    pass "$label"
  else
    fail "$label" "expected > $threshold, got: $actual"
  fi
}

# ── Wait for service ────────────────────────────────────────────────

header "Waiting for Marginalia"
for i in $(seq 1 30); do
  if curl -sf "$BASE_URL/healthz" > /dev/null 2>&1; then
    pass "service healthy"
    break
  fi
  if [[ "$i" -eq 30 ]]; then
    fail "service did not become healthy after 30s"
    exit 1
  fi
  sleep 1
done

# ── 1. Health check ─────────────────────────────────────────────────

header "1. Health check"
HEALTH=$(curl -sf "$BASE_URL/healthz")
STATUS=$(echo "$HEALTH" | jq -r '.status')
assert_eq "healthz returns ok" "ok" "$STATUS"

# ── 2. Auth ─────────────────────────────────────────────────────────

header "2. Authentication"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/v1/sources")
assert_eq "unauthenticated request returns 401" "401" "$HTTP_CODE"

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer wrong-token" "$BASE_URL/api/v1/sources")
assert_eq "wrong token returns 401" "401" "$HTTP_CODE"

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $TOKEN" "$BASE_URL/api/v1/sources")
assert_eq "correct token returns 200" "200" "$HTTP_CODE"

# ── 3. Sources ──────────────────────────────────────────────────────

header "3. Sources"
SOURCES=$(api GET /api/v1/sources)
SOURCE_COUNT=$(echo "$SOURCES" | jq 'length')
assert_gt "at least one source configured" 0 "$SOURCE_COUNT"

# Sources are per-user now (id like "readeck-<uid>"); match on the stable type.
READECK_TYPE=$(echo "$SOURCES" | jq -r '.[] | select(.type == "readeck") | .type')
assert_eq "readeck source exists" "readeck" "$READECK_TYPE"

# ── 4. Readeck sync ────────────────────────────────────────────────

header "4. Readeck sync"
SYNC_RESULT=$(api POST /api/v1/sync)
READECK_STATUS=$(echo "$SYNC_RESULT" | jq -r '.readeck.status')
assert_eq "readeck sync completed" "completed" "$READECK_STATUS"

DOCS_SYNCED=$(echo "$SYNC_RESULT" | jq -r '.readeck.documents')
assert_eq "synced 2 documents from mock readeck" "2" "$DOCS_SYNCED"

HLS_SYNCED=$(echo "$SYNC_RESULT" | jq -r '.readeck.highlights')
assert_eq "synced 3 highlights from mock readeck" "3" "$HLS_SYNCED"

# ── 5. Verify documents ────────────────────────────────────────────

header "5. Documents"
DOCS=$(api GET /api/v1/documents)
DOC_COUNT=$(echo "$DOCS" | jq 'length')
assert_eq "2 documents in database" "2" "$DOC_COUNT"

DOC1_TITLE=$(echo "$DOCS" | jq -r '.[] | select(.source_document_id == "bm-001") | .title')
assert_eq "document 1 title" "The Art of Focused Reading" "$DOC1_TITLE"

DOC2_TITLE=$(echo "$DOCS" | jq -r '.[] | select(.source_document_id == "bm-002") | .title')
assert_eq "document 2 title" "Why Self-Hosting Matters" "$DOC2_TITLE"

# Get doc1 with highlights
DOC1_ID=$(echo "$DOCS" | jq -r '.[] | select(.source_document_id == "bm-001") | .id')
DOC1_FULL=$(api GET "/api/v1/documents/$DOC1_ID")
DOC1_HL_COUNT=$(echo "$DOC1_FULL" | jq '.highlights | length')
assert_eq "document 1 has 2 highlights" "2" "$DOC1_HL_COUNT"

DOC1_HL_TEXT=$(echo "$DOC1_FULL" | jq -r '.highlights[0].text')
assert_contains "highlight text present" "distractions" "$DOC1_HL_TEXT"

# ── 6. KOReader push ───────────────────────────────────────────────

header "6. KOReader highlight push"

# Push book highlights via Readwise-compatible endpoint
KOREADER_RESULT=$(api POST /api/v2/highlights -d '{
  "highlights": [
    {
      "text": "The most rewarding things in life take years",
      "title": "How to Live",
      "author": "Derek Sivers",
      "source_type": "koreader",
      "category": "books",
      "note": "Patience matters",
      "location": 42,
      "location_type": "order",
      "highlighted_at": "2025-03-15T14:30:00Z"
    },
    {
      "text": "Be useful to others",
      "title": "How to Live",
      "author": "Derek Sivers",
      "source_type": "koreader",
      "category": "books",
      "location": 88,
      "location_type": "order",
      "highlighted_at": "2025-03-15T15:00:00Z"
    }
  ]
}')
KO_DOCS=$(echo "$KOREADER_RESULT" | jq -r '.documents')
KO_HLS=$(echo "$KOREADER_RESULT" | jq -r '.highlights')
assert_eq "koreader: 1 document ingested" "1" "$KO_DOCS"
assert_eq "koreader: 2 highlights ingested" "2" "$KO_HLS"

# Verify total documents now = 3 (2 readeck + 1 koreader)
ALL_DOCS=$(api GET /api/v1/documents)
ALL_DOC_COUNT=$(echo "$ALL_DOCS" | jq 'length')
assert_eq "total documents now 3" "3" "$ALL_DOC_COUNT"

# Verify the book was stored correctly
BOOK_TITLE=$(echo "$ALL_DOCS" | jq -r '.[] | select(.type == "book") | .title')
assert_eq "koreader book title" "How to Live" "$BOOK_TITLE"

# ── 7. KOReader deduplication ───────────────────────────────────────

header "7. KOReader deduplication"

# Push same highlights again
api POST /api/v2/highlights -d '{
  "highlights": [
    {
      "text": "The most rewarding things in life take years",
      "title": "How to Live",
      "author": "Derek Sivers",
      "source_type": "koreader",
      "category": "books",
      "note": "Updated note",
      "location": 42,
      "location_type": "order",
      "highlighted_at": "2025-03-15T14:30:00Z"
    }
  ]
}' > /dev/null

# Still 3 documents total
ALL_DOCS2=$(api GET /api/v1/documents)
ALL_DOC_COUNT2=$(echo "$ALL_DOCS2" | jq 'length')
assert_eq "no duplicate documents after re-push" "3" "$ALL_DOC_COUNT2"

# Book still has 2 highlights (not 3)
BOOK_ID=$(echo "$ALL_DOCS2" | jq -r '.[] | select(.type == "book") | .id')
BOOK_FULL=$(api GET "/api/v1/documents/$BOOK_ID")
BOOK_HL_COUNT=$(echo "$BOOK_FULL" | jq '.highlights | length')
assert_eq "no duplicate highlights after re-push" "2" "$BOOK_HL_COUNT"

# Note was updated
UPDATED_NOTE=$(echo "$BOOK_FULL" | jq -r '.highlights[] | select(.location == "42") | .note')
assert_eq "note updated on re-push" "Updated note" "$UPDATED_NOTE"

# ── 8. Export API ───────────────────────────────────────────────────

header "8. Export API"
EXPORT=$(api GET /api/v1/export)
EXPORT_COUNT=$(echo "$EXPORT" | jq 'length')
assert_eq "export returns 3 documents" "3" "$EXPORT_COUNT"

# Verify rendered content contains expected fields
ARTICLE_EXPORT=$(echo "$EXPORT" | jq -r '.[] | select(.title == "The Art of Focused Reading") | .content')
assert_contains "export has title property" "title:: The Art of Focused Reading" "$ARTICLE_EXPORT"
assert_contains "export has author" "Alice Writer" "$ARTICLE_EXPORT"
assert_contains "export has highlight text" "Deep reading requires removing all distractions" "$ARTICLE_EXPORT"

BOOK_EXPORT=$(echo "$EXPORT" | jq -r '.[] | select(.title == "How to Live") | .content')
assert_contains "book export has title" "title:: How to Live" "$BOOK_EXPORT"
assert_contains "book export has author" "Derek Sivers" "$BOOK_EXPORT"
assert_contains "book export has highlight" "most rewarding things" "$BOOK_EXPORT"

# Verify checksums are present
CHECKSUMS=$(echo "$EXPORT" | jq -r '.[].checksum' | grep -c 'sha256:')
assert_eq "all exports have sha256 checksums" "3" "$CHECKSUMS"

# ── 9. Export since filter ──────────────────────────────────────────

header "9. Export since filter"

# Use a timestamp in the past — should return all docs
EXPORT_ALL=$(api GET "/api/v1/export?since=2020-01-01T00:00:00Z")
EXPORT_ALL_COUNT=$(echo "$EXPORT_ALL" | jq 'length')
assert_eq "export since past returns all 3" "3" "$EXPORT_ALL_COUNT"

# Use a timestamp in the future — should return 0
EXPORT_NONE=$(api GET "/api/v1/export?since=2099-01-01T00:00:00Z")
EXPORT_NONE_COUNT=$(echo "$EXPORT_NONE" | jq 'length')
assert_eq "export since future returns 0" "0" "$EXPORT_NONE_COUNT"

# ── 10. Readeck re-sync (idempotency) ──────────────────────────────

header "10. Readeck re-sync (idempotency)"
SYNC2=$(api POST /api/v1/sync/readeck)
READECK_STATUS2=$(echo "$SYNC2" | jq -r '.status')
assert_eq "re-sync completed" "completed" "$READECK_STATUS2"

# Still same number of documents and highlights
DOCS_AFTER=$(api GET /api/v1/documents)
DOC_COUNT_AFTER=$(echo "$DOCS_AFTER" | jq 'length')
assert_eq "no duplicate documents after re-sync" "3" "$DOC_COUNT_AFTER"

HIGHLIGHTS_AFTER=$(api GET /api/v1/highlights)
HL_COUNT_AFTER=$(echo "$HIGHLIGHTS_AFTER" | jq 'length')
assert_eq "no duplicate highlights after re-sync" "5" "$HL_COUNT_AFTER"

# ── 11. Template management ────────────────────────────────────────

header "11. Template management"

# List templates (should be empty — defaults are built-in, not stored)
TEMPLATES=$(api GET /api/v1/templates)
TPL_COUNT=$(echo "$TEMPLATES" | jq 'length')
assert_eq "no custom templates initially" "0" "$TPL_COUNT"

# Create a custom template
CREATE_RESULT=$(api POST /api/v1/templates -d '{
  "id": "custom-article",
  "name": "Custom Article Template",
  "type": "article",
  "page_template": "# {{ title }}\nby {{ author }}\n\n{% for highlight in highlights %}- {{ highlight.text }}\n{% endfor %}",
  "highlight_template": "",
  "is_default": true
}')
CREATED_ID=$(echo "$CREATE_RESULT" | jq -r '.id')
assert_eq "created custom template" "custom-article" "$CREATED_ID"

# Export should now use the custom template for articles
EXPORT_CUSTOM=$(api GET /api/v1/export)
CUSTOM_ARTICLE=$(echo "$EXPORT_CUSTOM" | jq -r '.[] | select(.title == "The Art of Focused Reading") | .content')
assert_contains "custom template applied: heading" "# The Art of Focused Reading" "$CUSTOM_ARTICLE"
assert_contains "custom template applied: author" "by Alice Writer" "$CUSTOM_ARTICLE"

# Save the checksum for later comparison
ARTICLE_CHECKSUM_1=$(echo "$EXPORT_CUSTOM" | jq -r '.[] | select(.title == "The Art of Focused Reading") | .checksum')

# Update the template
api PUT "/api/v1/templates/custom-article" -d '{
  "name": "Custom Article Template v2",
  "type": "article",
  "page_template": "## {{ title }} — {{ author }}\n{% for highlight in highlights %}- {{ highlight.text }}\n{% endfor %}",
  "highlight_template": "",
  "is_default": true
}' > /dev/null

# Export again — content and checksum should change
EXPORT_UPDATED=$(api GET /api/v1/export)
UPDATED_ARTICLE=$(echo "$EXPORT_UPDATED" | jq -r '.[] | select(.title == "The Art of Focused Reading") | .content')
assert_contains "updated template applied" "## The Art of Focused Reading — Alice Writer" "$UPDATED_ARTICLE"

ARTICLE_CHECKSUM_2=$(echo "$EXPORT_UPDATED" | jq -r '.[] | select(.title == "The Art of Focused Reading") | .checksum')
if [[ "$ARTICLE_CHECKSUM_1" != "$ARTICLE_CHECKSUM_2" ]]; then
  pass "checksum changed after template update"
else
  fail "checksum should change after template update"
fi

# ── 12. Template preview ───────────────────────────────────────────

header "12. Template preview"
PREVIEW=$(api POST /api/v1/templates/preview -d '{
  "page_template": "Preview: {{ title }} ({{ num_highlights }} highlights)",
  "type": "book"
}')
PREVIEW_CONTENT=$(echo "$PREVIEW" | jq -r '.content')
assert_contains "preview renders sample data" "Preview:" "$PREVIEW_CONTENT"
assert_contains "preview has highlight count" "highlights)" "$PREVIEW_CONTENT"

# ── 13. Sync status / history ───────────────────────────────────────

header "13. Sync status"
SYNC_STATUS=$(api GET /api/v1/sync/status)
LOG_COUNT=$(echo "$SYNC_STATUS" | jq 'length')
assert_gt "sync logs recorded" 0 "$LOG_COUNT"

LATEST_STATUS=$(echo "$SYNC_STATUS" | jq -r '.[0].status')
assert_eq "latest sync status is completed" "completed" "$LATEST_STATUS"

# ── 14. Readwise auth endpoint ──────────────────────────────────────

header "14. Readwise auth endpoint (KOReader token check)"
AUTH_RESULT=$(api GET /api/v2/auth)
AUTH_STATUS=$(echo "$AUTH_RESULT" | jq -r '.status')
assert_eq "v2/auth returns ok" "ok" "$AUTH_STATUS"

# ── Summary ─────────────────────────────────────────────────────────

header "Summary"
TOTAL=$((PASS + FAIL))
printf "\n  %d/%d tests passed\n" "$PASS" "$TOTAL"

if [[ "$FAIL" -gt 0 ]]; then
  printf "  \033[31m%d FAILED\033[0m\n\n" "$FAIL"
  exit 1
else
  printf "  \033[32mAll tests passed!\033[0m\n\n"
  exit 0
fi
