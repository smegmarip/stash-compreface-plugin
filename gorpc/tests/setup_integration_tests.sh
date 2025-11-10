#!/bin/bash
#
# Integration Test Setup Script
# This script helps configure environment variables for running integration tests.
#
# Usage:
#   1. Obtain API keys from Compreface web UI (http://localhost:8000)
#   2. Edit this script and replace the placeholder keys
#   3. Source this script: source ./setup_integration_tests.sh
#   4. Run tests: go test -tags=integration ./tests/integration/... -v
#

# ============================================================================
# Compreface API Keys
# ============================================================================
# To obtain API keys:
# 1. Open Compreface web UI: http://localhost:8000
# 2. Navigate to each service (Recognition, Detection, Verification)
# 3. Create an application if one doesn't exist
# 4. Copy the API keys from the application page
# 5. Replace the values below with your actual API keys

export COMPREFACE_URL="http://localhost:8000"
export COMPREFACE_RECOGNITION_KEY="35228992-5b8f-45c7-9fd9-37c1456ada37"
export COMPREFACE_DETECTION_KEY="c79708e7-2a0c-4377-b29c-4aea90e74730"
export COMPREFACE_VERIFICATION_KEY=""  # Optional - not currently used in tests

# ============================================================================
# Stash Configuration
# ============================================================================
export STASH_URL="http://localhost:9999"

# ============================================================================
# Vision Service Configuration
# ============================================================================
# Note: Vision API runs on port 5010 (mapped from container port 5000)
export VISION_SERVICE_URL="http://localhost:5010"

# ============================================================================
# Quality Service Configuration
# ============================================================================
export QUALITY_SERVICE_URL="http://localhost:6001"

# ============================================================================
# Validation
# ============================================================================
echo "Environment variables set:"
echo "  COMPREFACE_URL: $COMPREFACE_URL"
echo "  COMPREFACE_RECOGNITION_KEY: ${COMPREFACE_RECOGNITION_KEY:0:8}... (${#COMPREFACE_RECOGNITION_KEY} chars)"
echo "  COMPREFACE_DETECTION_KEY: ${COMPREFACE_DETECTION_KEY:0:8}... (${#COMPREFACE_DETECTION_KEY} chars)"
echo "  STASH_URL: $STASH_URL"
echo "  VISION_SERVICE_URL: $VISION_SERVICE_URL"
echo "  QUALITY_SERVICE_URL: $QUALITY_SERVICE_URL"
echo ""

# Verify keys are set
if [[ -z "$COMPREFACE_RECOGNITION_KEY" ]] || [[ -z "$COMPREFACE_DETECTION_KEY" ]]; then
    echo "⚠️  WARNING: Compreface API keys are not configured!"
    echo "   Please edit this script and set the API keys."
    echo "   Get keys from: http://localhost:8000"
    echo ""
else
    echo "✓ Compreface API keys configured"
fi

# Test connectivity
echo "Testing service connectivity..."

# Test Stash
if curl -s -f "$STASH_URL/graphql" -H "Content-Type: application/json" -d '{"query":"{version{version}}"}' > /dev/null 2>&1; then
    echo "  ✓ Stash is accessible at $STASH_URL"
else
    echo "  ✗ Stash is NOT accessible at $STASH_URL"
fi

# Test Compreface
if curl -s -f "$COMPREFACE_URL/" > /dev/null 2>&1; then
    echo "  ✓ Compreface is accessible at $COMPREFACE_URL"
else
    echo "  ✗ Compreface is NOT accessible at $COMPREFACE_URL"
fi

# Test Vision Service
if curl -s -f "$VISION_SERVICE_URL/health" > /dev/null 2>&1; then
    echo "  ✓ Vision Service is accessible at $VISION_SERVICE_URL"
else
    echo "  ⚠  Vision Service may not be accessible at $VISION_SERVICE_URL"
fi

# Test Quality Service
if curl -s -f "$QUALITY_SERVICE_URL/health" > /dev/null 2>&1; then
    echo "  ✓ Quality Service is accessible at $QUALITY_SERVICE_URL"
else
    echo "  ⚠  Quality Service may not be accessible at $QUALITY_SERVICE_URL"
fi

echo ""
echo "To run integration tests:"
echo "  go test -tags=integration ./tests/integration/... -v"
echo ""
