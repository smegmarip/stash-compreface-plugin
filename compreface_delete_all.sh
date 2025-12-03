#!/bin/bash
#
# Compreface Delete All Subjects Script
#
# Deletes all subjects from Compreface recognition service.
# Use this to clear old FaceNet embeddings before re-enrollment.
#
# Usage: ./compreface_delete_all.sh
#

set -e  # Exit on error

# Configuration
COMPREFACE_URL="http://localhost:8000"
RECOGNITION_API_KEY="48400a42-bba3-4a6e-8bd3-af406595c38a"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Fetch all subjects
log_info "Fetching subjects from Compreface..."
SUBJECTS_RESPONSE=$(curl -s -w "\n%{http_code}" -X GET \
    "${COMPREFACE_URL}/api/v1/recognition/subjects" \
    -H "x-api-key: ${RECOGNITION_API_KEY}")

HTTP_CODE=$(echo "${SUBJECTS_RESPONSE}" | tail -n 1)
SUBJECTS_BODY=$(echo "${SUBJECTS_RESPONSE}" | sed '$d')

if [ "${HTTP_CODE}" != "200" ]; then
    log_error "Failed to fetch subjects (HTTP ${HTTP_CODE})"
    log_error "Response: ${SUBJECTS_BODY}"
    exit 1
fi

# Parse subject names
SUBJECTS=$(echo "${SUBJECTS_BODY}" | jq -r '.subjects[]' 2>/dev/null)

if [ -z "${SUBJECTS}" ]; then
    log_info "No subjects found in Compreface"
    exit 0
fi

SUBJECT_COUNT=$(echo "${SUBJECTS}" | wc -l | tr -d ' ')
log_info "Found ${SUBJECT_COUNT} subjects to delete"

# Confirm deletion
echo ""
log_warn "This will permanently delete all ${SUBJECT_COUNT} subjects and their face embeddings!"
read -p "Are you sure you want to continue? (yes/no): " CONFIRM

if [ "${CONFIRM}" != "yes" ]; then
    log_info "Deletion cancelled"
    exit 0
fi

echo ""
log_info "Deleting subjects..."

# Delete each subject
CURRENT=0
DELETED=0
FAILED=0

while IFS= read -r SUBJECT_NAME; do
    CURRENT=$((CURRENT + 1))

    # URL encode subject name
    ENCODED_SUBJECT=$(printf %s "${SUBJECT_NAME}" | jq -sRr @uri)

    log_info "[${CURRENT}/${SUBJECT_COUNT}] Deleting: ${SUBJECT_NAME}"

    DELETE_RESPONSE=$(curl -s -w "\n%{http_code}" -X DELETE \
        "${COMPREFACE_URL}/api/v1/recognition/subjects/${ENCODED_SUBJECT}" \
        -H "x-api-key: ${RECOGNITION_API_KEY}")

    DELETE_HTTP_CODE=$(echo "${DELETE_RESPONSE}" | tail -n 1)

    if [ "${DELETE_HTTP_CODE}" == "200" ]; then
        DELETED=$((DELETED + 1))
    else
        DELETE_BODY=$(echo "${DELETE_RESPONSE}" | sed '$d')
        log_warn "  Failed to delete (HTTP ${DELETE_HTTP_CODE}): ${DELETE_BODY}"
        FAILED=$((FAILED + 1))
    fi
done <<< "${SUBJECTS}"

# Summary
echo ""
log_info "=============== Deletion Complete ==============="
log_info "Subjects deleted: ${DELETED}/${SUBJECT_COUNT}"
if [ ${FAILED} -gt 0 ]; then
    log_warn "Failed deletions: ${FAILED}"
fi
log_info "=============================================="
