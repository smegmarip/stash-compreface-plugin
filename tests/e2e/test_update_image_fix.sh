#!/bin/bash

# Quick test to verify UpdateImage fix for performer association
# This tests the critical bug found in Suite 3

set -e

# Source libraries
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

source "${LIB_DIR}/graphql.sh"
source "${LIB_DIR}/logging.sh"
source "${LIB_DIR}/database.sh"

# Configuration
PLUGIN_ID="compreface-rpc"

echo ""
echo "=========================================="
echo "  UpdateImage Fix Verification Test"
echo "=========================================="
echo ""

# Test 1: Get a test image
echo "[INFO] Step 1: Finding test image..."
IMAGE_RESULT=$(graphql_query 'query { findImages(filter: {per_page: 1}) { images { id } } }')
IMAGE_ID=$(echo "$IMAGE_RESULT" | python3 -c "
import sys, json
data = json.load(sys.stdin)
images = data.get('data', {}).get('findImages', {}).get('images', [])
if images:
    print(images[0]['id'])
else:
    sys.exit(1)
" 2>/dev/null)

if [ -z "$IMAGE_ID" ]; then
    echo "[ERROR] Failed to find test image"
    exit 1
fi

echo "[INFO] Using image ID: $IMAGE_ID"

# Test 2: Get a performer
echo "[INFO] Step 2: Finding performer..."
PERFORMER_RESULT=$(graphql_query 'query { findPerformers(filter: {per_page: 1}) { performers { id name } } }')
PERFORMER_ID=$(echo "$PERFORMER_RESULT" | python3 -c "
import sys, json
data = json.load(sys.stdin)
performers = data.get('data', {}).get('findPerformers', {}).get('performers', [])
if performers:
    print(performers[0]['id'])
else:
    sys.exit(1)
" 2>/dev/null)

if [ -z "$PERFORMER_ID" ]; then
    echo "[ERROR] Failed to find performer"
    exit 1
fi

echo "[INFO] Using performer ID: $PERFORMER_ID"

# Test 3: Run identification on the single image
echo "[INFO] Step 3: Running identification task..."
clear_log

JOB_ID=$(run_plugin_task "$PLUGIN_ID" "Identify Unscanned Images")

if [ -z "$JOB_ID" ]; then
    echo "[ERROR] Failed to start task"
    exit 1
fi

echo "[INFO] Task started: Job ID $JOB_ID"
echo "[INFO] Polling for completion..."

if ! poll_job_status "$JOB_ID" 300; then
    echo "[ERROR] Task did not complete"
    exit 1
fi

echo "[INFO] Task completed"

# Test 4: Check logs for performer association and UpdateImage success
echo "[INFO] Step 4: Checking logs for successful performer association..."

sleep 2  # Give time for logs to flush

# Check for face matching
MATCHED=$(docker logs stash 2>&1 | tail -100 | grep -c "Matched subject" || true)
echo "[INFO] Found $MATCHED face matches in logs"

# Check for performer association
ASSOCIATED=$(docker logs stash 2>&1 | tail -100 | grep -c "Associated with performer" || true)
echo "[INFO] Found $ASSOCIATED performer associations in logs"

# Check for UpdateImage calls
UPDATES=$(docker logs stash 2>&1 | tail -100 | grep -c "Updating image .* with .* performer" || true)
echo "[INFO] Found $UPDATES UpdateImage calls in logs"

# Check for 422 errors (should be ZERO now)
ERRORS=$(docker logs stash 2>&1 | tail -100 | grep -c "422 Unprocessable Entity" || true)
echo "[INFO] Found $ERRORS GraphQL 422 errors in logs"

# Test 5: Verify images actually have performers associated
echo "[INFO] Step 5: Verifying images have performer associations in database..."

IMAGES_WITH_PERFORMERS=$(graphql_query 'query { findImages(image_filter: {performers: {modifier: NOT_NULL}}, filter: {per_page: 1}) { count } }' | \
    python3 -c "import sys, json; data = json.load(sys.stdin); print(data.get('data', {}).get('findImages', {}).get('count', 0))" 2>/dev/null)

echo "[INFO] Images with performers: $IMAGES_WITH_PERFORMERS"

# Summary
echo ""
echo "=========================================="
echo "  Test Results"
echo "=========================================="
echo ""
echo "Face matches found:       $MATCHED"
echo "Performers associated:    $ASSOCIATED"
echo "UpdateImage calls:        $UPDATES"
echo "422 Errors (should be 0): $ERRORS"
echo "Images with performers:   $IMAGES_WITH_PERFORMERS"
echo ""

if [ "$ERRORS" -eq 0 ] && [ "$IMAGES_WITH_PERFORMERS" -gt 0 ]; then
    echo -e "${GREEN}✓ UpdateImage fix verified - performers are being associated!${NC}"
    exit 0
else
    echo -e "${RED}✗ UpdateImage fix verification failed${NC}"
    if [ "$ERRORS" -gt 0 ]; then
        echo "[ERROR] Still seeing 422 errors - fix did not work"
    fi
    if [ "$IMAGES_WITH_PERFORMERS" -eq 0 ]; then
        echo "[ERROR] No images have performers - association still failing"
    fi
    exit 1
fi
