#!/bin/bash
# Quick runner for Reset Unmatched Images test

set -euo pipefail

cd "$(dirname "$0")"

export STASH_URL="${STASH_URL:-http://localhost:9999}"
export COMPREFACE_URL="${COMPREFACE_URL:-http://localhost:8000}"
export COMPREFACE_RECOGNITION_KEY="${COMPREFACE_RECOGNITION_KEY:-35228992-5b8f-45c7-9fd9-37c1456ada37}"
export COMPREFACE_DETECTION_KEY="${COMPREFACE_DETECTION_KEY:-c79708e7-2a0c-4377-b29c-4aea90e74730}"
TEST_MODE="${TEST_MODE:-all}" 

echo "Running Reset Unmatched Images Test..."
echo ""

if [ "$TEST_MODE" == "all" ] || [ "$TEST_MODE" == "images" ]; then
    ./suites/07_reset_unmatched.sh
fi
if [ "$TEST_MODE" == "all" ] || [ "$TEST_MODE" == "scenes" ]; then
    ./suites/08_reset_unmatched_scenes.sh
fi
if [ "$TEST_MODE" != "all" ] && [ "$TEST_MODE" != "images" ] && [ "$TEST_MODE" != "scenes" ]; then
    echo "Unknown TEST_MODE: $TEST_MODE. Valid options are: all, images, scenes."
    echo "Skipping all tests."
    exit 1
fi