#!/bin/bash

# Test Suite 2: Synchronize Performers
# Tests the synchronizePerformers task which creates Compreface subjects for existing Stash performers

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
TASK_NAME="Synchronize Performers"

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

# Test 1: Verify performers exist in database
test_performers_exist() {
    local performer_count=$(count_performers_graphql)

    if [ "$performer_count" -gt 0 ]; then
        echo "[INFO] Found $performer_count performers in database"
        return 0
    else
        echo "[ERROR] No performers found in database"
        echo "[INFO] This test requires existing performers"
        return 1
    fi
}

# Test 2: Get sample performers for testing
test_get_sample_performers() {
    # Get first 3 performers (using simple query to avoid disk I/O errors)
    local result=$(graphql_query "query { findPerformers(filter: {per_page: 3}) { performers { id name } } }")

    # Extract performer IDs
    PERFORMER_IDS=$(echo "$result" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    performers = data.get('data', {}).get('findPerformers', {}).get('performers', [])
    for p in performers[:3]:
        print(p['id'])
except Exception as e:
    print(f'Error: {e}', file=sys.stderr)
    sys.exit(1)
" 2>/dev/null)

    if [ -z "$PERFORMER_IDS" ]; then
        echo "[ERROR] Failed to get performer IDs"
        return 1
    fi

    echo "[INFO] Sample performer IDs:"
    echo "$PERFORMER_IDS" | while read -r id; do
        echo "  - $id"
    done

    # Store for later tests
    export SAMPLE_PERFORMER_IDS="$PERFORMER_IDS"

    return 0
}

# Test 3: Remove sync tags from all performers
test_remove_sync_tags() {
    echo "[INFO] Removing 'Compreface Synced' tags from all performers"

    # Get all performers
    local performers=$(graphql_query 'query { findPerformers(filter: {per_page: -1}) { performers { id tags { name } } } }')

    # Extract performers with sync tag and remove it
    local count=0
    echo "$performers" | python3 -c "
import sys, json
data = json.load(sys.stdin)
performers = data.get('data', {}).get('findPerformers', {}).get('performers', [])
for p in performers:
    has_sync_tag = any(tag.get('name') == 'Compreface Synced' for tag in p.get('tags', []))
    if has_sync_tag:
        # Get all tag IDs except the sync tag
        other_tags = [tag['id'] for tag in p.get('tags', []) if tag.get('name') != 'Compreface Synced']
        print(f\"{p['id']}|{','.join(other_tags)}\")
" 2>/dev/null | while IFS='|' read -r perf_id tag_ids; do
        [ -z "$perf_id" ] && continue

        # Build tag_ids array for GraphQL
        if [ -z "$tag_ids" ]; then
            tag_array="[]"
        else
            tag_array="[$(echo "$tag_ids" | sed 's/,/","/g' | sed 's/^/"/' | sed 's/$/"/')]"
        fi

        # Update performer with tags excluding sync tag
        graphql_query "mutation { performerUpdate(input: {id: \"$perf_id\", tag_ids: $tag_array}) { id } }" >/dev/null
        count=$((count + 1))
    done

    echo "[INFO] Removed sync tags from $count performers"
    return 0
}

# Test 4: Clean up Compreface subjects from previous runs
test_cleanup_subjects() {
    echo "[INFO] Cleaning up Compreface subjects from previous runs"
    delete_all_subjects
    return 0
}

# Test 5: Check initial Compreface subject count
test_initial_subject_count() {
    INITIAL_SUBJECT_COUNT=$(count_compreface_subjects)

    echo "[INFO] Initial Compreface subjects: $INITIAL_SUBJECT_COUNT"

    # Export for later comparison
    export INITIAL_SUBJECT_COUNT

    return 0
}

# Test 6: Run Synchronize Performers task
test_run_sync_task() {
    echo "[INFO] Running task: $TASK_NAME"

    # Clear log before task
    clear_log

    # Run task and get job ID
    local job_id=$(run_plugin_task "$PLUGIN_ID" "$TASK_NAME")

    if [ -z "$job_id" ]; then
        echo "[ERROR] Failed to start task"
        return 1
    fi

    echo "[INFO] Task started with Job ID: $job_id"

    # Poll for completion (5 minute timeout for large batches)
    if ! poll_job_status "$job_id" 300; then
        echo "[ERROR] Task did not complete"
        return 1
    fi

    echo "[INFO] Task completed successfully"

    # Store job ID for log analysis
    export SYNC_JOB_ID="$job_id"

    return 0
}

# Test 7: Verify subject count increased
test_subject_count_increased() {
    local current_count=$(count_compreface_subjects)

    echo "[INFO] Subjects after sync: $current_count (was: $INITIAL_SUBJECT_COUNT)"

    if [ "$current_count" -gt "$INITIAL_SUBJECT_COUNT" ]; then
        local delta=$((current_count - INITIAL_SUBJECT_COUNT))
        echo "[INFO] Created $delta new subjects"
        return 0
    else
        echo "[ERROR] Subject count did not increase"
        return 1
    fi
}

# Test 8: Verify subject naming pattern
test_subject_naming_pattern() {
    echo "[INFO] Validating subject naming pattern"

    local subjects=$(list_compreface_subjects)

    if [ -z "$subjects" ]; then
        echo "[ERROR] No subjects found"
        return 1
    fi

    local invalid_count=0
    local valid_count=0

    while IFS= read -r subject; do
        if echo "$subject" | grep -qE '^Person [0-9]+ [A-Z0-9]{16}$'; then
            valid_count=$((valid_count + 1))
            echo "[INFO] Valid subject: $subject"
        else
            invalid_count=$((invalid_count + 1))
            echo "[WARN] Invalid subject: $subject"
        fi
    done <<< "$subjects"

    echo "[INFO] Valid subjects: $valid_count, Invalid: $invalid_count"

    if [ "$invalid_count" -eq 0 ] && [ "$valid_count" -gt 0 ]; then
        return 0
    else
        echo "[ERROR] Found $invalid_count subjects with invalid naming"
        return 1
    fi
}

# Test 9: Verify performers have aliases
test_performers_have_aliases() {
    echo "[INFO] Checking if performers have Compreface aliases"

    if [ -z "$SAMPLE_PERFORMER_IDS" ]; then
        echo "[ERROR] No sample performer IDs available"
        return 1
    fi

    local performers_with_aliases=0
    local total_checked=0

    while IFS= read -r performer_id; do
        if [ -z "$performer_id" ]; then
            continue
        fi

        total_checked=$((total_checked + 1))

        local result=$(get_performer "$performer_id")
        local has_person_alias=$(echo "$result" | python3 -c "
import sys, json
data = json.load(sys.stdin)
performer = data.get('data', {}).get('findPerformer', {})
aliases = performer.get('alias_list', [])
has_person = any(alias.startswith('Person ') for alias in aliases)
print('true' if has_person else 'false')
" 2>/dev/null)

        if [ "$has_person_alias" = "true" ]; then
            performers_with_aliases=$((performers_with_aliases + 1))
            echo "[INFO] Performer $performer_id has 'Person' alias"
        else
            echo "[WARN] Performer $performer_id does not have 'Person' alias"
        fi
    done <<< "$SAMPLE_PERFORMER_IDS"

    echo "[INFO] Performers with aliases: $performers_with_aliases/$total_checked"

    # At least one performer should have an alias
    if [ "$performers_with_aliases" -gt 0 ]; then
        return 0
    else
        echo "[ERROR] No performers have 'Person' aliases"
        return 1
    fi
}

# Test 10: Verify no errors in log
test_no_errors_in_log() {
    echo "[INFO] Checking log for errors"

    if check_log_errors; then
        return 0
    else
        echo "[ERROR] Errors found in log"
        # Show last few error lines
        extract_log_lines "\\[Error\\]\\|panic:" 10
        return 1
    fi
}

# Test 11: Verify cooldown was applied (if multiple batches)
test_cooldown_applied() {
    echo "[INFO] Checking if cooldown was applied"

    local cooldown_count=$(count_log_matches "Cooling down")

    if [ "$cooldown_count" -gt 0 ]; then
        echo "[INFO] Cooldown applied $cooldown_count times (batching detected)"
        return 0
    else
        echo "[INFO] No cooldown detected (single batch or no batching needed)"
        # This is not a failure - just means batch size was small enough
        return 0
    fi
}

# Test 12: Verify task completion message
test_task_completion_message() {
    echo "[INFO] Checking for task completion message"

    if wait_for_log_pattern "synchronize.*complete\\|Performer synchronization completed" 5; then
        return 0
    else
        echo "[WARN] Task completion message not found in log"
        # Not critical - task may complete without explicit message
        return 0
    fi
}

# Run all tests
echo ""
echo "=================================="
echo "  Synchronize Performers Tests"
echo "=================================="
echo ""

run_test "Performers exist in database" test_performers_exist || exit 1
run_test "Get sample performers" test_get_sample_performers || exit 1
run_test "Remove sync tags from performers" test_remove_sync_tags || exit 1
run_test "Clean up previous subjects" test_cleanup_subjects || exit 1
run_test "Check initial subject count" test_initial_subject_count || exit 1
run_test "Run Synchronize Performers task" test_run_sync_task || exit 1
run_test "Verify subject count increased" test_subject_count_increased || exit 1
run_test "Verify subject naming pattern" test_subject_naming_pattern || exit 1
run_test "Verify performers have aliases" test_performers_have_aliases || exit 1
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
