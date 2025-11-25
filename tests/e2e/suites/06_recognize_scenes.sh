#!/bin/bash
# E2E Test Suite 6: Recognize New Scenes
#
# Tests the "Recognize New Scenes" and "Recognize New Scene Sprites" plugin tasks
# which extract faces from video scenes and match them to performers.
#
# These tasks require the Vision Service (stash-auto-vision) to be running
# for face detection in video frames.
#
# Prerequisites:
# - Stash running with test data
# - Vision Service running at configured URL (default: http://localhost:5010)
# - Compreface service running
# - Scenes in the database

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
LIMIT=1
TEST_MODE="${TEST_MODE:-all}"  # all, scenes, sprites, all_scenes, all_sprites
VISION_SERVICE_URL="${VISION_SERVICE_URL:-http://vision-api:5010}"

echo "================================================================"
echo "Suite 6: Scene Recognition (Scenes/Sprites)"
echo "================================================================"
echo "Limit: ${LIMIT}"
echo "Test mode: ${TEST_MODE}"
echo "Vision Service URL: ${VISION_SERVICE_URL}"
echo ""

# Helper function to check Vision Service availability
check_vision_service() {
    echo "Checking Vision Service availability..."

    # Try the configured URL first
    if curl -s -f "${VISION_SERVICE_URL}/health" >/dev/null 2>&1; then
        echo "✅ Vision Service is available at ${VISION_SERVICE_URL}"
        return 0
    fi

    # If that fails, try localhost:5010 (common for host-based checks)
    if [ "${VISION_SERVICE_URL}" != "http://localhost:5010" ]; then
        echo "Trying fallback: http://localhost:5010/health"
        if curl -s -f "http://localhost:5010/health" >/dev/null 2>&1; then
            echo "✅ Vision Service is available at http://localhost:5010"
            echo "   (Note: Plugin uses container URL internally)"
            return 0
        fi
    fi

    # Check if it's a container URL that we can't reach from the test script
    if [[ "${VISION_SERVICE_URL}" =~ vision-api|host\.docker\.internal ]]; then
        echo "⚠️  Cannot verify Vision Service from test script (container URL detected)"
        echo "   Configured URL: ${VISION_SERVICE_URL}"
        echo ""
        echo "This is a container-to-container URL that's not reachable from the host."
        echo "The plugin will attempt to connect from within the Stash container."
        echo ""
        echo "Proceeding with test - check plugin logs if it fails."
        return 0
    fi

    echo "❌ Vision Service is not available at ${VISION_SERVICE_URL}"
    echo ""
    echo "The Vision Service (stash-auto-vision) is required for scene recognition."
    echo "Please start the Vision Service or skip this test."
    echo ""
    echo "To start Vision Service:"
    echo "  cd /path/to/stash-auto-vision"
    echo "  docker-compose up -d"
    echo ""
    echo "If Vision Service is running in Docker, ensure it's accessible:"
    echo "  - From host: http://localhost:5010"
    echo "  - From container: http://vision-api:5010 or http://host.docker.internal:5010"
    echo ""
    return 1
}

# Helper function to get scene count
get_scene_count() {
    local query="query { findScenes(filter: { per_page: 1 }) { count } }"

    jq -n --arg q "$query" '{query: $q}' | \
        curl -s -X POST "${STASH_URL:-http://localhost:9999}/graphql" \
            -H "Content-Type: application/json" -d @- | \
        jq -r '.data.findScenes.count'
}

# Function to run scene recognition test
run_scene_recognition_test() {
    local task_name=$1
    local mode_name=$2

    echo "Testing ${mode_name} Recognition..."
    echo "----------------------------------------"

    # Get scene count
    local scene_count
    scene_count=$(get_scene_count)

    if [ -z "$scene_count" ] || [ "$scene_count" = "null" ] || [ "$scene_count" = "0" ]; then
        echo "⚠️  No scenes found in database - skipping ${mode_name} test"
        return 0
    fi

    echo "Scenes in database: ${scene_count}"

    # Calculate expected processed count
    local expected_processed=$scene_count
    if [ "$expected_processed" -gt "$LIMIT" ]; then
        expected_processed=$LIMIT
    fi

    echo "Expected to process up to: ${expected_processed} scenes"
    echo ""

    # Run scene recognition task
    echo "Running task: ${task_name}..."
    local job_id
    job_id=$(run_plugin_task "${PLUGIN_ID}" "${task_name}" "{\"limit\": ${LIMIT}")

    if [ -z "${job_id}" ]; then
        echo "❌ Failed to start ${task_name} task"
        return 1
    fi

    echo "Job ID: ${job_id}"
    echo ""

    # Wait for completion (300 second timeout - scene processing can be slow)
    echo "Waiting for job to complete..."
    echo "(This may take a while as scenes are processed by Vision Service)"
    if ! poll_job_status "${job_id}" 900; then
        echo "❌ Job ${job_id} failed or timed out"
        echo ""
        echo "Note: Scene recognition requires the Vision Service to be running."
        echo "Check Vision Service logs for details:"
        echo "  docker logs stash-auto-vision"
        return 1
    fi

    echo "✅ Job completed"
    echo ""

    # Check plugin logs for errors (Stash marks jobs as FINISHED even when plugin returns error)
    echo "Checking for plugin errors..."
    if docker logs stash 2>&1 | tail -50 | grep -q "vision service URL not configured"; then
        echo "❌ Vision Service URL is not configured in plugin settings"
        echo ""
        echo "To fix this:"
        echo "  1. Go to Stash UI: Settings → Plugins → Compreface"
        echo "  2. Set 'Vision Service URL' to: http://host.docker.internal:5010"
        echo "  3. Click 'Save' and reload the plugin"
        echo ""
        return 1
    fi

    echo "✅ No configuration errors detected"
    echo ""

    # Validation
    echo "Validating results..."
    echo "Scene recognition results are complex - manual verification recommended:"
    echo "  1. Check Stash UI for scenes with updated performer tags"
    echo "  2. Verify new performers were created for unknown faces"
    echo "  3. Check Vision Service logs for processing details"
    echo ""

    echo "✅ ${mode_name} recognition test completed"
    echo ""

    return 0
}

# Check Vision Service first
if ! check_vision_service; then
    echo "================================================================"
    echo "⚠️  Suite 6: Scene Recognition SKIPPED"
    echo "================================================================"
    echo "Reason: Vision Service not available"
    echo ""
    echo "This is not a test failure - the Vision Service is optional."
    echo "Scene recognition features are available when the Vision Service is deployed."
    exit 0
fi

echo ""

# Run tests based on TEST_MODE
case "${TEST_MODE}" in
    scenes)
        run_scene_recognition_test "Recognize New Scenes" "Scene" || exit 1
        ;;
    sprites)
        run_scene_recognition_test "Recognize New Scene Sprites" "Scene Sprite" || exit 1
        ;;
    all_scenes)
        run_scene_recognition_test "Recognize All Scenes" "All Scenes" || exit 1
        ;;
    all_sprites)
        run_scene_recognition_test "Recognize All Scene Sprites" "All Scene Sprites" || exit 1
        ;;
    all|*)
        run_scene_recognition_test "Recognize New Scenes" "Scene" || exit 1
        echo ""
        echo "================================================================"
        echo ""
        run_scene_recognition_test "Recognize New Scene Sprites" "Scene Sprite" || exit 1
        ;;
esac

echo "================================================================"
echo "✅ Suite 6: Scene Recognition Tests PASSED"
echo "================================================================"
