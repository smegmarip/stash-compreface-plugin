#!/bin/bash
#
# Compreface Export Script
#
# Exports all subjects and their face images from Compreface.
# Used for migrating from FaceNet to ArcFace embedding models.
#
# Usage: ./compreface_export.sh
#

set -e  # Exit on error

# Configuration
COMPREFACE_URL="http://localhost:8000"
RECOGNITION_API_KEY="35228992-5b8f-45c7-9fd9-37c1456ada37"
OUTPUT_DIR="./compreface_backup"
IMAGES_DIR="${OUTPUT_DIR}/images"
SUBJECTS_FILE="${OUTPUT_DIR}/subjects.json"

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

# Create output directory structure
log_info "Creating output directory: ${OUTPUT_DIR}"
mkdir -p "${OUTPUT_DIR}"
mkdir -p "${IMAGES_DIR}"

# Fetch all subjects
log_info "Fetching subjects from Compreface..."
SUBJECTS_RESPONSE=$(curl -s -w "\n%{http_code}" -X GET \
    "${COMPREFACE_URL}/api/v1/recognition/subjects?page=0&size=1000" \
    -H "x-api-key: ${RECOGNITION_API_KEY}")

HTTP_CODE=$(echo "${SUBJECTS_RESPONSE}" | tail -n 1)
SUBJECTS_BODY=$(echo "${SUBJECTS_RESPONSE}" | sed '$d')

if [ "${HTTP_CODE}" != "200" ]; then
    log_error "Failed to fetch subjects (HTTP ${HTTP_CODE})"
    log_error "Response: ${SUBJECTS_BODY}"
    exit 1
fi

# Parse subject names from response
SUBJECT_NAMES=$(echo "${SUBJECTS_BODY}" | jq -r '.subjects[]' 2>/dev/null)

if [ -z "${SUBJECT_NAMES}" ]; then
    log_warn "No subjects found in Compreface"
    echo '{"subjects":[]}' > "${SUBJECTS_FILE}"
    exit 0
fi

SUBJECT_COUNT=$(echo "${SUBJECT_NAMES}" | wc -l | tr -d ' ')
log_info "Found ${SUBJECT_COUNT} subjects"

# Initialize subjects metadata
echo '{"subjects":[]}' > "${SUBJECTS_FILE}"

# Process each subject
CURRENT=0
TOTAL_IMAGES=0
FAILED_IMAGES=0

while IFS= read -r SUBJECT_NAME; do
    CURRENT=$((CURRENT + 1))
    log_info "[${CURRENT}/${SUBJECT_COUNT}] Processing subject: ${SUBJECT_NAME}"

    # Create subject directory
    SUBJECT_DIR="${IMAGES_DIR}/${SUBJECT_NAME}"
    mkdir -p "${SUBJECT_DIR}"

    # Fetch face examples for this subject
    FACES_RESPONSE=$(curl -s -w "\n%{http_code}" -X GET \
        "${COMPREFACE_URL}/api/v1/recognition/faces?page=0&size=100&subject=$(printf %s "${SUBJECT_NAME}" | jq -sRr @uri)" \
        -H "x-api-key: ${RECOGNITION_API_KEY}")

    FACES_HTTP_CODE=$(echo "${FACES_RESPONSE}" | tail -n 1)
    FACES_BODY=$(echo "${FACES_RESPONSE}" | sed '$d')

    if [ "${FACES_HTTP_CODE}" != "200" ]; then
        log_warn "Failed to fetch faces for subject '${SUBJECT_NAME}' (HTTP ${FACES_HTTP_CODE})"
        continue
    fi

    # Parse image IDs
    IMAGE_IDS=$(echo "${FACES_BODY}" | jq -r '.faces[].image_id' 2>/dev/null)

    if [ -z "${IMAGE_IDS}" ]; then
        log_warn "No face images found for subject '${SUBJECT_NAME}'"
        continue
    fi

    IMAGE_COUNT=$(echo "${IMAGE_IDS}" | wc -l | tr -d ' ')
    log_info "  Found ${IMAGE_COUNT} face images"

    # Download each image
    IMAGE_IDS_ARRAY=()
    while IFS= read -r IMAGE_ID; do
        IMAGE_PATH="${SUBJECT_DIR}/${IMAGE_ID}.jpg"

        curl -s -f -X GET \
            "${COMPREFACE_URL}/api/v1/recognition/faces/${IMAGE_ID}/img" \
            -H "x-api-key: ${RECOGNITION_API_KEY}" \
            -o "${IMAGE_PATH}"

        if [ $? -eq 0 ]; then
            IMAGE_IDS_ARRAY+=("\"${IMAGE_ID}\"")
            TOTAL_IMAGES=$((TOTAL_IMAGES + 1))
        else
            log_warn "  Failed to download image ${IMAGE_ID}"
            FAILED_IMAGES=$((FAILED_IMAGES + 1))
        fi
    done <<< "${IMAGE_IDS}"

    # Add subject to metadata JSON
    IMAGE_IDS_JSON=$(IFS=,; echo "${IMAGE_IDS_ARRAY[*]}")
    SUBJECT_ENTRY=$(cat <<EOF
{
  "subject": "${SUBJECT_NAME}",
  "image_ids": [${IMAGE_IDS_JSON}]
}
EOF
)

    # Update subjects.json
    TMP_FILE=$(mktemp)
    jq ".subjects += [${SUBJECT_ENTRY}]" "${SUBJECTS_FILE}" > "${TMP_FILE}"
    mv "${TMP_FILE}" "${SUBJECTS_FILE}"

done <<< "${SUBJECT_NAMES}"

# Summary
log_info ""
log_info "=============== Export Complete ==============="
log_info "Subjects exported: ${SUBJECT_COUNT}"
log_info "Images downloaded: ${TOTAL_IMAGES}"
if [ ${FAILED_IMAGES} -gt 0 ]; then
    log_warn "Failed downloads: ${FAILED_IMAGES}"
fi
log_info "Output directory: ${OUTPUT_DIR}"
log_info "Metadata file: ${SUBJECTS_FILE}"
log_info "=============================================="
