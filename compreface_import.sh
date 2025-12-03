#!/bin/bash
#
# Compreface Import Script
#
# Imports subjects and face images to Compreface with ArcFace embeddings.
# Used for migrating from FaceNet to ArcFace embedding models.
#
# Usage: ./compreface_import.sh <NEW_RECOGNITION_API_KEY>
#

set -e  # Exit on error

# Check arguments
if [ $# -ne 1 ]; then
    echo "Usage: $0 <NEW_RECOGNITION_API_KEY>"
    echo ""
    echo "Example:"
    echo "  $0 abcd1234-5678-90ef-ghij-klmnopqrstuv"
    exit 1
fi

# Configuration
COMPREFACE_URL="http://localhost:8000"
RECOGNITION_API_KEY="$1"
INPUT_DIR="./compreface_backup"
IMAGES_DIR="${INPUT_DIR}/images"
SUBJECTS_FILE="${INPUT_DIR}/subjects.json"

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

# Verify input directory exists
if [ ! -d "${INPUT_DIR}" ]; then
    log_error "Input directory not found: ${INPUT_DIR}"
    log_error "Please run compreface_export.sh first"
    exit 1
fi

if [ ! -f "${SUBJECTS_FILE}" ]; then
    log_error "Subjects metadata file not found: ${SUBJECTS_FILE}"
    exit 1
fi

# Read subjects metadata
log_info "Reading subjects from ${SUBJECTS_FILE}"
SUBJECTS_JSON=$(cat "${SUBJECTS_FILE}")
SUBJECT_COUNT=$(echo "${SUBJECTS_JSON}" | jq '.subjects | length')

log_info "Found ${SUBJECT_COUNT} subjects to import"

# Process each subject
CURRENT=0
TOTAL_SUBJECTS_CREATED=0
FAILED_SUBJECTS=0
TOTAL_IMAGES_UPLOADED=0
FAILED_IMAGES=0

while [ ${CURRENT} -lt ${SUBJECT_COUNT} ]; do
    # Extract subject data
    SUBJECT_NAME=$(echo "${SUBJECTS_JSON}" | jq -r ".subjects[${CURRENT}].subject")
    IMAGE_IDS_JSON=$(echo "${SUBJECTS_JSON}" | jq -r ".subjects[${CURRENT}].image_ids[]")

    CURRENT=$((CURRENT + 1))
    log_info "[${CURRENT}/${SUBJECT_COUNT}] Processing subject: ${SUBJECT_NAME}"

    # Create subject
    CREATE_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
        "${COMPREFACE_URL}/api/v1/recognition/subjects" \
        -H "Content-Type: application/json" \
        -H "x-api-key: ${RECOGNITION_API_KEY}" \
        -d "{\"subject\": \"${SUBJECT_NAME}\"}")

    CREATE_HTTP_CODE=$(echo "${CREATE_RESPONSE}" | tail -n 1)
    CREATE_BODY=$(echo "${CREATE_RESPONSE}" | sed '$d')

    if [ "${CREATE_HTTP_CODE}" == "201" ] || [ "${CREATE_HTTP_CODE}" == "200" ]; then
        log_info "  Subject created successfully"
        TOTAL_SUBJECTS_CREATED=$((TOTAL_SUBJECTS_CREATED + 1))
    else
        log_warn "  Failed to create subject (HTTP ${CREATE_HTTP_CODE}): ${CREATE_BODY}"
        FAILED_SUBJECTS=$((FAILED_SUBJECTS + 1))
        continue
    fi

    # Upload images for this subject
    SUBJECT_DIR="${IMAGES_DIR}/${SUBJECT_NAME}"
    if [ ! -d "${SUBJECT_DIR}" ]; then
        log_warn "  Image directory not found: ${SUBJECT_DIR}"
        continue
    fi

    IMAGE_COUNT=0
    while IFS= read -r IMAGE_ID; do
        IMAGE_PATH="${SUBJECT_DIR}/${IMAGE_ID}.jpg"

        if [ ! -f "${IMAGE_PATH}" ]; then
            log_warn "  Image file not found: ${IMAGE_PATH}"
            FAILED_IMAGES=$((FAILED_IMAGES + 1))
            continue
        fi

        # Upload image
        UPLOAD_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
            "${COMPREFACE_URL}/api/v1/recognition/faces?subject=$(printf %s "${SUBJECT_NAME}" | jq -sRr @uri)" \
            -H "x-api-key: ${RECOGNITION_API_KEY}" \
            -F "file=@${IMAGE_PATH}")

        UPLOAD_HTTP_CODE=$(echo "${UPLOAD_RESPONSE}" | tail -n 1)

        if [ "${UPLOAD_HTTP_CODE}" == "201" ] || [ "${UPLOAD_HTTP_CODE}" == "200" ]; then
            IMAGE_COUNT=$((IMAGE_COUNT + 1))
            TOTAL_IMAGES_UPLOADED=$((TOTAL_IMAGES_UPLOADED + 1))
        else
            UPLOAD_BODY=$(echo "${UPLOAD_RESPONSE}" | sed '$d')
            log_warn "  Failed to upload image ${IMAGE_ID} (HTTP ${UPLOAD_HTTP_CODE})"
            FAILED_IMAGES=$((FAILED_IMAGES + 1))
        fi
    done <<< "${IMAGE_IDS_JSON}"

    log_info "  Uploaded ${IMAGE_COUNT} images"
done

# Summary
log_info ""
log_info "=============== Import Complete ==============="
log_info "Subjects created: ${TOTAL_SUBJECTS_CREATED}/${SUBJECT_COUNT}"
if [ ${FAILED_SUBJECTS} -gt 0 ]; then
    log_warn "Failed subjects: ${FAILED_SUBJECTS}"
fi
log_info "Images uploaded: ${TOTAL_IMAGES_UPLOADED}"
if [ ${FAILED_IMAGES} -gt 0 ]; then
    log_warn "Failed image uploads: ${FAILED_IMAGES}"
fi
log_info ""
log_info "Next steps:"
log_info "1. Update Stash plugin config with new recognition API key:"
log_info "   recognitionApiKey: ${RECOGNITION_API_KEY}"
log_info "2. Test embedding recognition with identifyImagesNew"
log_info "3. Verify similarity scores are ~0.92 (not 0.10)"
log_info "=============================================="
