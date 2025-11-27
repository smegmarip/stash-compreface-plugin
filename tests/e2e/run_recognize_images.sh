#!/bin/bash
# Quick runner for Low Quality recognition test only

set -euo pipefail

cd "$(dirname "$0")"

export STASH_URL="${STASH_URL:-http://localhost:9999}"
export COMPREFACE_URL="${COMPREFACE_URL:-http://localhost:8000}"
export COMPREFACE_RECOGNITION_KEY="${COMPREFACE_RECOGNITION_KEY:-35228992-5b8f-45c7-9fd9-37c1456ada37}"
export COMPREFACE_DETECTION_KEY="${COMPREFACE_DETECTION_KEY:-c79708e7-2a0c-4377-b29c-4aea90e74730}"

echo "Running Image Recognition Test..."
echo ""

./suites/04_recognize_images.sh
