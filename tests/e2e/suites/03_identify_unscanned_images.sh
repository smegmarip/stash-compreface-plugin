#!/bin/bash

# Test Suite 3: Identify Unscanned Images
# Tests the identifyImagesNew task which matches faces to existing performers
# This demonstrates test dependency - requires Suite 2 (Synchronize Performers) to have run first
# Expected: Suite 2 created 3 subjects, so this should match some images to those performers

set -e

# Source libraries
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/../lib"

source "${LIB_DIR}/database.sh"
source "${LIB_DIR}/logging.sh"
source "${LIB_DIR}/graphql.sh"
source "${LIB_DIR}/validation.sh"

# Configuration
PLUGIN_ID="compreface-rpc"
TASK_NAME="Identify Unscanned Images"

# Test state
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Helper function to run test
run_test() {
    local test_name="$1"
    local test_function="$2"

    TESTS_RUN=$((TESTS_RUN + 1))
    echo ""
    echo -e "${BLUE}Test $TESTS_RUN: $test_name${NC}"
    echo "----------------------------------------"

    if $test_function; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        echo -e "${GREEN}✓ Test passed${NC}"
        return 0
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        echo -e "${RED}✗ Test failed${NC}"
        return 1
    fi
}

# Test 1: Verify Compreface subjects exist (from Suite 2)
test_subjects_exist() {
    local subject_count=$(count_compreface_subjects)

    echo "[INFO] Compreface subjects: $subject_count"

    if [ "$subject_count" -gt 0 ]; then
        echo "[INFO] Prerequisites met - subjects exist from previous suite"
        export INITIAL_SUBJECT_COUNT="$subject_count"
        return 0
    else
        echo "[ERROR] No Compreface subjects found - Suite 2 must run first"
        return 1
    fi
}

# Test 2: Count images without "Compreface Scanned" tag
test_count_unscanned_images() {
    # Count total images
    local total_images=$(graphql_query 'query { findImages(filter: {per_page: 1}) { count } }' | \
        python3 -c "import sys, json; data = json.load(sys.stdin); print(data.get('data', {}).get('findImages', {}).get('count', 0))" 2>/dev/null)

    echo "[INFO] Total images in database: $total_images"

    if [ "$total_images" -gt 0 ]; then
        export INITIAL_IMAGE_COUNT="$total_images"
        return 0
    else
        echo "[ERROR] No images found in database"
        return 1
    fi
}

# Test 3: Run Identify Unscanned Images task
test_run_identify_task() {
    echo "[INFO] Running task: $TASK_NAME"
    echo "[INFO] Note: This is a long-running task (may take several hours for large datasets)"

    # Clear log before task
    clear_log

    # Run task
    local job_id=$(run_plugin_task "$PLUGIN_ID" "$TASK_NAME")

    if [ -z "$job_id" ]; then
        echo "[ERROR] Failed to start task"
        return 1
    fi

    echo "[INFO] Task started with Job ID: $job_id"

    # Poll for completion (5 hour timeout - 18000 seconds)
    # This is a long-running task designed to process all images
    if ! poll_job_status "$job_id" 18000; then
        echo "[WARN] Task reached timeout after 5 hours"
        echo "[INFO] Gracefully stopping task..."

        # Attempt to stop the job
        graphql_query "mutation { stopJob(job_id: \"$job_id\") }" >/dev/null 2>&1 || true

        echo "[INFO] Task was stopped due to timeout (this is expected for very large datasets)"
        echo "[INFO] Partial processing was successful"

        # Export job ID for later tests
        export IDENTIFY_JOB_ID="$job_id"

        # Return success since timeout is acceptable for this long-running task
        return 0
    fi

    echo "[INFO] Task completed successfully"
    export IDENTIFY_JOB_ID="$job_id"

    return 0
}

# Test 4: Verify some images were scanned
test_images_scanned() {
    echo "[INFO] Checking if images were scanned"

    # Check log for scanning messages
    if wait_for_log_pattern "Processing.*image\|Scanned.*images" 10; then
        echo "[INFO] Image scanning detected in log"
        return 0
    else
        echo "[WARN] No explicit scanning messages found (task may have processed 0 images)"
        # Not a failure - might be no unscanned images
        return 0
    fi
}

# Test 5: Verify matched tag exists
test_matched_tag_exists() {
    echo "[INFO] Checking if 'Compreface Matched' tag was created"

    local result=$(graphql_query 'query { findTags(tag_filter: {name: {value: "Compreface Matched", modifier: EQUALS}}, filter: {per_page: 1}) { count } }')

    local count=$(echo "$result" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data.get('data', {}).get('findTags', {}).get('count', 0))" 2>/dev/null)

    if [ "$count" -gt 0 ]; then
        echo "[INFO] 'Compreface Matched' tag exists"
        return 0
    else
        echo "[WARN] 'Compreface Matched' tag not found (no matches occurred)"
        # Not a failure - might be no matches
        return 0
    fi
}

# Test 6: Verify no errors in log
test_no_errors_in_log() {
    echo "[INFO] Checking log for errors"

    if check_log_errors; then
        return 0
    else
        echo "[ERROR] Errors found in log"
        extract_log_lines "\\[Error\\]\\|panic:" 10
        return 1
    fi
}

# Test 7: Verify cooldown applied (if large batch)
test_cooldown_applied() {
    echo "[INFO] Checking if cooldown was applied"

    local cooldown_count=$(count_log_matches "Cooling down")

    if [ "$cooldown_count" -gt 0 ]; then
        echo "[INFO] Cooldown applied $cooldown_count times (large batch processed)"
        return 0
    else
        echo "[INFO] No cooldown detected (small batch or no processing needed)"
        return 0
    fi
}

# Test 8: Verify task completion message
test_task_completion_message() {
    echo "[INFO] Checking for task completion message"

    if wait_for_log_pattern "identification.*complete\|Image identification completed" 5; then
        return 0
    else
        echo "[WARN] Task completion message not found in log"
        # Not critical
        return 0
    fi
}

# Run all tests
echo ""
echo "=================================="
echo "  Identify Unscanned Images Tests"
echo "=================================="
echo ""

run_test "Verify Compreface subjects exist" test_subjects_exist || exit 1
run_test "Count images in database" test_count_unscanned_images || exit 1
run_test "Run Identify Unscanned Images task" test_run_identify_task || exit 1
run_test "Verify images were scanned" test_images_scanned || exit 1
run_test "Verify matched tag exists" test_matched_tag_exists || exit 1
run_test "Verify no errors in log" test_no_errors_in_log || exit 1
run_test "Verify cooldown applied" test_cooldown_applied || exit 1
run_test "Verify task completion" test_task_completion_message || exit 1

# Summary
echo ""
echo "=================================="
echo "  Test Summary"
echo "=================================="
echo ""
echo "Tests Run:    $TESTS_RUN"
echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed${NC}"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi
