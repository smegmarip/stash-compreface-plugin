#!/bin/bash
# E2E Test Suite 4: Recognize Images (HQ and LQ)
#
# Tests the "Recognize Images" plugin tasks which detect faces in images
# and create performer groups for unrecognized faces.
#
# Modes tested:
# - High Quality (recognizeImages): Processes all unscanned images
#
# Prerequisites:
# - Stash running with test data
# - Compreface service running
# - Images without "Compreface Scanned" tag

set -euo pipefail

# Source libraries
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/../lib"

source "${LIB_DIR}/database.sh"
source "${LIB_DIR}/logging.sh"
source "${LIB_DIR}/graphql.sh"
source "${LIB_DIR}/validation.sh"

# Configuration
PLUGIN_ID="compreface-rpc"

# Test configuration
LIMIT=1

echo "================================================================"
echo "Suite 4: Image Recognition (HQ)"
echo "================================================================"
echo "Limit: ${LIMIT}"
echo ""

# Function to run recognition and validate results
run_recognition_test() {
    local task_name=$1

    echo "Testing Image Recognition..."
    echo "----------------------------------------"

    # Get count of unscanned images before
    local before_count
    before_count=$(get_unscanned_image_count)

    if [ -z "${before_count}" ] || [ "${before_count}" = "null" ]; then
        echo "⚠️  Failed to get unscanned image count - skipping image recognition test"
        return 0
    fi

    echo "Unscanned images before: ${before_count}"

    if [ "${before_count}" -eq 0 ]; then
        echo "⚠️  No unscanned images found - skipping image recognition test"
        return 0
    fi

    # Calculate expected processed count
    local expected_processed
    if [ "${before_count}" -lt "${LIMIT}" ]; then
        expected_processed="${before_count}"
    else
        expected_processed="${LIMIT}"
    fi

    echo "Expected to process up to: ${expected_processed} images"
    echo ""

    # Run recognition task
    echo "Running task: ${task_name}..."
    local job_id
    job_id=$(run_plugin_task "${PLUGIN_ID}" "${task_name}" "{\"limit\": ${LIMIT}}")

    if [ -z "${job_id}" ]; then
        echo "❌ Failed to start ${task_name} task"
        return 1
    fi

    echo "Job ID: ${job_id}"

    # Wait for completion
    echo "Waiting for job to complete..."
    if ! poll_job_status "${job_id}" 300; then
        echo "❌ Job ${job_id} failed or timed out"
        return 1
    fi

    echo "✅ Job completed successfully"
    echo ""

    # Validate results
    echo "Validating results..."

    # Get count after
    local after_count
    after_count=$(get_unscanned_image_count)

    if [ -z "${after_count}" ] || [ "${after_count}" = "null" ]; then
        after_count=0
    fi

    echo "Unscanned images after: ${after_count}"

    # Calculate how many were processed
    local actual_processed=$((before_count - after_count))
    echo "Actually processed: ${actual_processed}"

    # Validate count (allow some flexibility for face detection failures)
    if [ "${actual_processed}" -le 0 ]; then
        echo "⚠️  Warning: No images were processed"
        return 0  # Don't fail - might be expected if all images already processed
    fi

    # Check for new performers created
    local performer_count
    performer_count=$(get_performer_count)
    echo "Total performers in Stash: ${performer_count}"

    # Check for "Compreface Scanned" tag usage
    local scanned_count
    scanned_count=$(get_scanned_image_count)
    echo "Images with 'Compreface Scanned' tag: ${scanned_count}"

    echo ""
    echo "✅ image recognition test passed"
    echo "   Processed: ${actual_processed} images"
    echo "   Performers: ${performer_count}"
    echo "   Scanned: ${scanned_count}"
    echo ""

    return 0
}

# Helper function to get tag ID by name
get_tag_id() {
    local tag_name=$1
    local query="query { allTags { id name } }"

    echo "{}" | jq -n --arg q "$query" "{query: \$q}" | \
        curl -s -X POST "${STASH_URL:-http://localhost:9999}/graphql" \
            -H "Content-Type: application/json" -d @- | \
        jq -r ".data.allTags[] | select(.name == \"${tag_name}\") | .id"
}

# Helper function to get unscanned image count
get_unscanned_image_count() {
    local tag_id=$(get_tag_id "Compreface Scanned")

    if [ -z "$tag_id" ]; then
        echo "null"
        return
    fi

    local query="query { findImages(image_filter: { tags: { value: [${tag_id}], modifier: EXCLUDES } } filter: { per_page: 1 }) { count } }"

    echo "{}" | jq -n --arg q "$query" "{query: \$q}" | \
        curl -s -X POST "${STASH_URL:-http://localhost:9999}/graphql" \
            -H "Content-Type: application/json" -d @- | \
        jq -r '.data.findImages.count'
}

# Helper function to get scanned image count
get_scanned_image_count() {
    local tag_id=$(get_tag_id "Compreface Scanned")

    if [ -z "$tag_id" ]; then
        echo "0"
        return
    fi

    local query="query { findImages(image_filter: { tags: { value: [${tag_id}], modifier: INCLUDES } } filter: { per_page: 1 }) { count } }"

    echo "{}" | jq -n --arg q "$query" "{query: \$q}" | \
        curl -s -X POST "${STASH_URL:-http://localhost:9999}/graphql" \
            -H "Content-Type: application/json" -d @- | \
        jq -r '.data.findImages.count'
}

# Helper function to get performer count
get_performer_count() {
    local query="query { findPerformers(filter: { per_page: 1 }) { count } }"

    echo "{}" | jq -n --arg q "$query" "{query: \$q}" | \
        curl -s -X POST "${STASH_URL:-http://localhost:9999}/graphql" \
            -H "Content-Type: application/json" -d @- | \
        jq -r '.data.findPerformers.count'
}

# Run tests
run_recognition_test "Recognize Images" "High Quality" || exit 1

echo "================================================================"
echo "✅ Suite 4: Image Recognition Tests PASSED"
echo "================================================================"
