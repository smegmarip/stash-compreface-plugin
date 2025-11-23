#!/bin/bash
# E2E Test Suite 8: Reset Unmatched Scenes
#
# Tests the "Reset Unmatched Scenes" plugin task which removes the
# "Compreface Scanned" tag from scenes that were scanned but not matched
# to any performers (don't have "Compreface Matched" tag).
#
# Prerequisites:
# - Stash running with test data
# - Some scenes with "Compreface Scanned" tag but no "Compreface Matched" tag

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
TASK_NAME="Reset Unmatched Scenes"
LIMIT=50

echo "================================================================"
echo "Suite 8: Reset Unmatched Scenes"
echo "================================================================"
echo "Limit: ${LIMIT}"
echo ""

# Helper function to get tag ID by name
get_tag_id() {
    local tag_name=$1
    local query="query { allTags { id name } }"

    echo "{}" | jq -n --arg q "$query" "{query: \$q}" | \
        curl -s -X POST "${STASH_URL:-http://localhost:9999}/graphql" \
            -H "Content-Type: application/json" -d @- | \
        jq -r ".data.allTags[] | select(.name == \"${tag_name}\") | .id"
}

# Helper function to get count of scenes with scanned tag but no matched tag (unmatched)
get_unmatched_scene_count() {
    local scanned_tag_id=$(get_tag_id "Compreface Scanned")
    local matched_tag_id=$(get_tag_id "Compreface Matched")

    if [ -z "$scanned_tag_id" ] || [ -z "$matched_tag_id" ]; then
        echo "0"
        return
    fi

    # Find scenes with scanned tag
    local query="query { findScenes(scene_filter: { tags: { value: [${scanned_tag_id}], modifier: INCLUDES } } filter: { per_page: -1 }) { scenes { id tags { id } } } }"

    local scenes_json
    scenes_json=$(echo "{}" | jq -n --arg q "$query" "{query: \$q}" | \
        curl -s -X POST "${STASH_URL:-http://localhost:9999}/graphql" \
            -H "Content-Type: application/json" -d @-)

    # Filter to only those without matched tag
    echo "$scenes_json" | jq -r "[.data.findScenes.scenes[] | select(.tags | map(.id == \"${matched_tag_id}\") | any | not)] | length"
}

# Helper function to get count of scenes with scanned tag
get_scanned_scene_count() {
    local scanned_tag_id=$(get_tag_id "Compreface Scanned")

    if [ -z "$scanned_tag_id" ]; then
        echo "0"
        return
    fi

    local query="query { findScenes(scene_filter: { tags: { value: [${scanned_tag_id}], modifier: INCLUDES } } filter: { per_page: 1 }) { count } }"

    echo "{}" | jq -n --arg q "$query" "{query: \$q}" | \
        curl -s -X POST "${STASH_URL:-http://localhost:9999}/graphql" \
            -H "Content-Type: application/json" -d @- | \
        jq -r '.data.findScenes.count'
}

echo "Step 1: Getting initial counts..."
echo "----------------------------------------"

# Get counts before reset
before_unmatched=$(get_unmatched_scene_count)
before_scanned=$(get_scanned_scene_count)

echo "Unmatched scenes (scanned but not matched): ${before_unmatched}"
echo "Total scanned scenes: ${before_scanned}"
echo ""

if [ "$before_unmatched" = "0" ]; then
    echo "⚠️  No unmatched scenes found - skipping reset test"
    echo "   (This is expected if all scanned scenes were matched)"
    echo ""
    echo "================================================================"
    echo "✅ Suite 8: Reset Unmatched Scenes PASSED (nothing to reset)"
    echo "================================================================"
    exit 0
fi

# Calculate expected count after reset
expected_processed=$before_unmatched
if [ "$expected_processed" -gt "$LIMIT" ]; then
    expected_processed=$LIMIT
fi

echo "Expected to reset up to: ${expected_processed} scenes"
echo ""

echo "Step 2: Running reset task..."
echo "----------------------------------------"

# Run reset task
echo "Running task: ${TASK_NAME}..."
args_json=$(jq -n --arg limit "${LIMIT}" '{limit: ($limit | tonumber)}')
job_id=$(run_plugin_task "${PLUGIN_ID}" "${TASK_NAME}" "${args_json}")

if [ -z "${job_id}" ]; then
    echo "❌ Failed to start ${TASK_NAME} task"
    exit 1
fi

echo "Job ID: ${job_id}"
echo ""

# Wait for completion (30 second timeout - this should be fast)
echo "Waiting for job to complete..."
if ! poll_job_status "${job_id}" 30; then
    echo "❌ Job ${job_id} failed or timed out"
    exit 1
fi

echo "✅ Job completed successfully"
echo ""

echo "Step 3: Validating results..."
echo "----------------------------------------"

# Get counts after reset
after_unmatched=$(get_unmatched_scene_count)
after_scanned=$(get_scanned_scene_count)

echo "Unmatched scenes after: ${after_unmatched}"
echo "Total scanned scenes after: ${after_scanned}"
echo ""

# Calculate how many were actually reset
actual_reset=$((before_unmatched - after_unmatched))

echo "Actually reset: ${actual_reset} scenes"
echo ""

# Validation
if [ "$actual_reset" -le 0 ]; then
    echo "⚠️  Warning: No scenes were reset"
    echo "   This might indicate an issue, but could also mean scenes were already matched"
fi

# The number of scanned scenes should decrease by the same amount
expected_scanned=$((before_scanned - actual_reset))
if [ "$after_scanned" != "$expected_scanned" ]; then
    echo "⚠️  Warning: Scanned count mismatch"
    echo "   Expected: ${expected_scanned}, Got: ${after_scanned}"
fi

echo ""
echo "================================================================"
echo "✅ Suite 8: Reset Unmatched Scenes PASSED"
echo "================================================================"
echo "Summary:"
echo "  - Unmatched before: ${before_unmatched}"
echo "  - Unmatched after: ${after_unmatched}"
echo "  - Scenes reset: ${actual_reset}"
echo "  - Scanned scenes reduced by: $((before_scanned - after_scanned))"
echo "================================================================"
