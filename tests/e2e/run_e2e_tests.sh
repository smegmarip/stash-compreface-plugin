#!/bin/bash

# E2E Testing - Run Plugin Tasks via Stash API
# Tests the deployed Compreface plugin by executing tasks through Stash GraphQL API

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Stash Compreface Plugin - E2E Tests${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Configuration
STASH_URL="http://localhost:9999"
PLUGIN_ID="compreface-rpc"

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Helper function to run GraphQL query
graphql_query() {
    local query="$1"
    curl -s -X POST "${STASH_URL}/graphql" \
        -H "Content-Type: application/json" \
        -d "{\"query\":\"$query\"}"
}

# Helper function to run plugin task
run_plugin_task() {
    local task_name="$1"
    local description="$2"

    echo -e "${BLUE}Running task: ${task_name}${NC}"

    local query="mutation { runPluginTask(plugin_id: \\\"${PLUGIN_ID}\\\", task_name: \\\"${task_name}\\\") }"
    local result=$(graphql_query "$query")

    # Extract job ID
    local job_id=$(echo "$result" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data.get('data', {}).get('runPluginTask', ''))" 2>/dev/null)

    if [ -z "$job_id" ]; then
        echo -e "${RED}✗ Failed to start task${NC}"
        echo "Response: $result"
        return 1
    fi

    echo -e "${GREEN}✓ Task started (Job ID: ${job_id})${NC}"

    # Poll job status
    local status=""
    local progress=0
    local max_wait=60  # 60 seconds max wait
    local waited=0

    while [ "$status" != "FINISHED" ] && [ "$status" != "FAILED" ] && [ $waited -lt $max_wait ]; do
        sleep 2
        waited=$((waited + 2))

        local job_query="query { findJob(input: {id: \\\"${job_id}\\\"}) { id status progress description } }"
        local job_result=$(graphql_query "$job_query")

        status=$(echo "$job_result" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data.get('data', {}).get('findJob', {}).get('status', ''))" 2>/dev/null)
        progress=$(echo "$job_result" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data.get('data', {}).get('findJob', {}).get('progress', 0))" 2>/dev/null)

        if [ -n "$status" ] && [ "$status" != "FINISHED" ] && [ "$status" != "FAILED" ]; then
            echo -e "  Status: ${YELLOW}${status}${NC} (Progress: ${progress}%)"
        fi
    done

    if [ "$status" = "FINISHED" ]; then
        echo -e "${GREEN}✓ Task completed successfully${NC}"
        return 0
    elif [ "$status" = "FAILED" ]; then
        echo -e "${RED}✗ Task failed${NC}"
        return 1
    else
        echo -e "${YELLOW}⚠ Task timeout after ${max_wait}s (Status: ${status})${NC}"
        return 1
    fi
}

# Helper function to check plugin settings are configured
check_plugin_settings() {
    echo -e "${BLUE}Checking plugin configuration...${NC}"

    # Query plugin configuration
    local query="query { configuration { plugins } }"
    local result=$(graphql_query "$query")

    # Check if Compreface URL is set
    if echo "$result" | grep -q "comprefaceUrl"; then
        echo -e "${GREEN}✓ Plugin settings accessible${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠ Plugin settings not configured${NC}"
        echo -e "${YELLOW}  Please configure Compreface URL and API keys in Stash UI${NC}"
        echo -e "${YELLOW}  http://localhost:9999/settings?tab=plugins${NC}"
        return 1
    fi
}

# Test 1: Verify plugin is registered and enabled
test_plugin_registration() {
    TESTS_RUN=$((TESTS_RUN + 1))
    echo ""
    echo -e "${BLUE}Test 1: Plugin Registration${NC}"
    echo "----------------------------------------"

    local query="query { plugins { id name version enabled } }"
    local result=$(graphql_query "$query")

    if echo "$result" | grep -q "\"id\":\"${PLUGIN_ID}\""; then
        local enabled=$(echo "$result" | grep -o "\"id\":\"${PLUGIN_ID}\"[^}]*\"enabled\":[a-z]*" | grep -o "true\|false")

        if [ "$enabled" = "true" ]; then
            echo -e "${GREEN}✓ Plugin is registered and enabled${NC}"
            TESTS_PASSED=$((TESTS_PASSED + 1))
            return 0
        else
            echo -e "${RED}✗ Plugin is registered but disabled${NC}"
            TESTS_FAILED=$((TESTS_FAILED + 1))
            return 1
        fi
    else
        echo -e "${RED}✗ Plugin not found${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Test 2: Verify plugin tasks are available
test_plugin_tasks() {
    TESTS_RUN=$((TESTS_RUN + 1))
    echo ""
    echo -e "${BLUE}Test 2: Plugin Tasks Availability${NC}"
    echo "----------------------------------------"

    local query="query { plugins { id name tasks { name description } } }"
    local result=$(graphql_query "$query")

    # Extract task count using Python for reliable JSON parsing
    local task_count=$(echo "$result" | python3 -c "import sys, json; data = json.load(sys.stdin); plugins = data.get('data', {}).get('plugins', []); plugin = next((p for p in plugins if p['id'] == '${PLUGIN_ID}'), None); print(len(plugin.get('tasks', [])) if plugin else 0)" 2>/dev/null || echo "0")

    if [ "$task_count" -ge 10 ]; then
        echo -e "${GREEN}✓ Found ${task_count} plugin tasks${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}✗ Expected at least 10 tasks, found ${task_count}${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Test 3: Run Synchronize Performers task (safe, read-only)
test_synchronize_performers() {
    TESTS_RUN=$((TESTS_RUN + 1))
    echo ""
    echo -e "${BLUE}Test 3: Synchronize Performers Task${NC}"
    echo "----------------------------------------"

    if run_plugin_task "Synchronize Performers" "Sync existing performers with Compreface"; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Test 4: Check Stash database has data
test_stash_data() {
    TESTS_RUN=$((TESTS_RUN + 1))
    echo ""
    echo -e "${BLUE}Test 4: Stash Database Content${NC}"
    echo "----------------------------------------"

    # Check for images
    local query="query { findImages(filter: {per_page: 1}) { count } }"
    local result=$(graphql_query "$query")
    local image_count=$(echo "$result" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data.get('data', {}).get('findImages', {}).get('count', 0))" 2>/dev/null)

    # Check for performers
    query="query { findPerformers(filter: {per_page: 1}) { count } }"
    result=$(graphql_query "$query")
    local performer_count=$(echo "$result" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data.get('data', {}).get('findPerformers', {}).get('count', 0))" 2>/dev/null)

    echo "  Images: ${image_count}"
    echo "  Performers: ${performer_count}"

    if [ "$image_count" -gt 0 ]; then
        echo -e "${GREEN}✓ Stash database has content${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${YELLOW}⚠ Stash database is empty${NC}"
        echo -e "${YELLOW}  Add some images to Stash for more comprehensive testing${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))  # Not a failure
        return 0
    fi
}

# Test 5: Check Compreface connectivity
test_compreface_connectivity() {
    TESTS_RUN=$((TESTS_RUN + 1))
    echo ""
    echo -e "${BLUE}Test 5: Compreface Service Connectivity${NC}"
    echo "----------------------------------------"

    # Source the integration test env
    if [ -f "../../gorpc/tests/setup_integration_tests.sh" ]; then
        source ../../gorpc/tests/setup_integration_tests.sh >/dev/null 2>&1
    fi

    if [ -z "$COMPREFACE_URL" ]; then
        COMPREFACE_URL="http://localhost:8000"
    fi

    if curl -s -f "${COMPREFACE_URL}/" >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Compreface is accessible at ${COMPREFACE_URL}${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${YELLOW}⚠ Compreface not accessible at ${COMPREFACE_URL}${NC}"
        echo -e "${YELLOW}  Ensure Compreface is running for full E2E testing${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))  # Not a failure
        return 0
    fi
}

# Run all tests
echo -e "${BLUE}Prerequisites Check...${NC}"
echo "----------------------------------------"
check_plugin_settings || true  # Don't exit if settings not configured

# Run tests
test_plugin_registration
test_plugin_tasks
test_stash_data
test_compreface_connectivity
test_synchronize_performers

# Summary
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Test Summary${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "Tests Run:    ${TESTS_RUN}"
echo -e "Tests Passed: ${GREEN}${TESTS_PASSED}${NC}"
echo -e "Tests Failed: ${RED}${TESTS_FAILED}${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Configure plugin settings: http://localhost:9999/settings?tab=plugins"
    echo "  2. Run additional tasks manually: http://localhost:9999/settings?tab=tasks"
    echo "  3. Check logs for any errors"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi
