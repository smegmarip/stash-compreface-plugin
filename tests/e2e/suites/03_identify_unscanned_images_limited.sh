#!/bin/bash
# Suite 3 (Limited): Identify Unscanned Images - Test with limit=20

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../utils/common.sh"

SUITE_NAME="Suite 3 (Limited)"
SUITE_DESCRIPTION="Identify Unscanned Images (Limit: 20)"

log_suite_start "$SUITE_NAME" "$SUITE_DESCRIPTION"

# Check prerequisites
log "Checking prerequisites..."
if ! check_stash_running; then
    error "Stash is not running"
    exit 1
fi

if ! check_compreface_running; then
    error "Compreface is not running"
    exit 1
fi

# Get plugin configuration
log "Fetching plugin configuration..."
PLUGIN_ID=$(get_plugin_id "Compreface")
if [ -z "$PLUGIN_ID" ]; then
    error "Compreface plugin not found"
    exit 1
fi
log "Plugin ID: $PLUGIN_ID"

# Run task: Identify Unscanned Images (limit=20)
log "Running task: Identify Unscanned Images (limit=20)..."

TASK_NAME="Identify Unscanned Images"
ARGS='[
  {
    "key": "limit",
    "value": {"i": 20}
  }
]'

JOB_ID=$(run_plugin_task "$PLUGIN_ID" "$TASK_NAME" "$ARGS")
if [ -z "$JOB_ID" ]; then
    error "Failed to start task"
    exit 1
fi

log "Task started with job ID: $JOB_ID"
log "Waiting for task to complete (timeout: 600 seconds)..."

# Wait for job completion with timeout
TIMEOUT=600
ELAPSED=0
POLL_INTERVAL=5

while [ $ELAPSED -lt $TIMEOUT ]; do
    STATUS=$(get_job_status "$JOB_ID")

    case "$STATUS" in
        "FINISHED")
            log "✓ Task completed successfully"
            break
            ;;
        "FAILED")
            error "Task failed"
            get_job_logs "$JOB_ID"
            exit 1
            ;;
        "CANCELLED")
            error "Task was cancelled"
            exit 1
            ;;
        "RUNNING"|"READY")
            # Get progress if available
            PROGRESS=$(get_job_progress "$JOB_ID")
            if [ -n "$PROGRESS" ]; then
                log "Task in progress: ${PROGRESS}%"
            else
                log "Task status: $STATUS"
            fi
            ;;
        *)
            log "Unknown status: $STATUS"
            ;;
    esac

    sleep $POLL_INTERVAL
    ELAPSED=$((ELAPSED + POLL_INTERVAL))
done

if [ $ELAPSED -ge $TIMEOUT ]; then
    error "Task timed out after $TIMEOUT seconds"
    get_job_logs "$JOB_ID"
    exit 1
fi

# Get job logs
log "Task logs:"
get_job_logs "$JOB_ID"

# Verify results
log "Verifying results..."

# Count images with "Compreface Matched" tag
MATCHED_TAG_ID=$(get_tag_id "Compreface Matched")
if [ -z "$MATCHED_TAG_ID" ]; then
    warning "Compreface Matched tag not found"
    MATCHED_COUNT=0
else
    MATCHED_COUNT=$(count_images_with_tag "$MATCHED_TAG_ID")
    log "Images with Compreface Matched tag: $MATCHED_COUNT"
fi

# Count images with "Compreface Scanned" tag
SCANNED_TAG_ID=$(get_tag_id "Compreface Scanned")
if [ -z "$SCANNED_TAG_ID" ]; then
    error "Compreface Scanned tag not found"
    exit 1
fi
SCANNED_COUNT=$(count_images_with_tag "$SCANNED_TAG_ID")
log "Images with Compreface Scanned tag: $SCANNED_COUNT"

# Verify limit was respected (should process at most 20 images)
if [ $SCANNED_COUNT -gt 20 ]; then
    error "Limit not respected: processed $SCANNED_COUNT images (expected max 20)"
    exit 1
fi

log "✓ Limit respected: processed $SCANNED_COUNT images (max 20)"

# Check if any matched images have performers associated
log "Checking if matched images have performers..."
check_matched_images_have_performers "$MATCHED_TAG_ID"

log_suite_end "$SUITE_NAME"
