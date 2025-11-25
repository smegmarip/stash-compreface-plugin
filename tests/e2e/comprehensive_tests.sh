#!/bin/bash

# Comprehensive E2E Test Orchestrator
# Manages test suite execution with database state management

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Directories
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"
SUITES_DIR="${SCRIPT_DIR}/suites"
DATA_DIR="${SCRIPT_DIR}/data"

# Source libraries
source "${LIB_DIR}/database.sh"
source "${LIB_DIR}/logging.sh"
source "${LIB_DIR}/graphql.sh"
source "${LIB_DIR}/validation.sh"

# Set environment variables directly (avoid interactive setup script)
export STASH_URL="${STASH_URL:-http://localhost:9999}"
export COMPREFACE_URL="${COMPREFACE_URL:-http://localhost:8000}"
export COMPREFACE_RECOGNITION_KEY="${COMPREFACE_RECOGNITION_KEY:-35228992-5b8f-45c7-9fd9-37c1456ada37}"
export COMPREFACE_DETECTION_KEY="${COMPREFACE_DETECTION_KEY:-c79708e7-2a0c-4377-b29c-4aea90e74730}"
export VISION_SERVICE_URL="${VISION_SERVICE_URL:-http://localhost:5010}"
export QUALITY_SERVICE_URL="${QUALITY_SERVICE_URL:-http://localhost:6001}"

# Configuration
PLUGIN_ID="compreface-rpc"

# Test state
TOTAL_SUITES=0
PASSED_SUITES=0
FAILED_SUITES=0
SUITE_RESULTS=()

# Banner
echo ""
echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                                                            ║${NC}"
echo -e "${CYAN}║       Stash Compreface Plugin - E2E Test Suite            ║${NC}"
echo -e "${CYAN}║                Comprehensive Testing                       ║${NC}"
echo -e "${CYAN}║                                                            ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Check prerequisites
echo -e "${BLUE}[1/6] Checking Prerequisites...${NC}"
echo "----------------------------------------"

# Validate services
if ! validate_stash_connectivity; then
    echo -e "${RED}✗ Stash is not accessible${NC}"
    exit 1
fi

if ! validate_compreface_connectivity; then
    echo -e "${RED}✗ Compreface is not accessible${NC}"
    exit 1
fi

# Validate environment variables
if [ -z "$COMPREFACE_RECOGNITION_KEY" ]; then
    echo -e "${RED}✗ COMPREFACE_RECOGNITION_KEY not set${NC}"
    exit 1
fi

if [ -z "$COMPREFACE_DETECTION_KEY" ]; then
    echo -e "${RED}✗ COMPREFACE_DETECTION_KEY not set${NC}"
    exit 1
fi

echo -e "${GREEN}✓ All prerequisites met${NC}"
echo ""

# Create baseline backup (including WAL files)
echo -e "${BLUE}[2/6] Creating Baseline Backup...${NC}"
echo "----------------------------------------"

BASELINE_FILE=$(create_baseline_backup)
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Baseline backup created${NC}"
    echo "  Location: $BASELINE_FILE"
    echo "  Size: $(get_database_size)"
else
    echo -e "${YELLOW}⚠ Using existing baseline backup${NC}"
fi

# Count initial state
INITIAL_IMAGES=$(count_images)
INITIAL_PERFORMERS=$(count_performers)
INITIAL_GALLERIES=$(count_galleries)
INITIAL_SCENES=$(count_scenes)
INITIAL_SUBJECTS=$(count_compreface_subjects)

echo ""
echo "Initial State:"
echo "  Images:      $INITIAL_IMAGES"
echo "  Performers:  $INITIAL_PERFORMERS"
echo "  Galleries:   $INITIAL_GALLERIES"
echo "  Scenes:      $INITIAL_SCENES"
echo "  Subjects:    $INITIAL_SUBJECTS"
echo ""

# Clear logs
echo -e "${BLUE}[3/6] Preparing Test Environment...${NC}"
echo "----------------------------------------"

clear_log
echo -e "${GREEN}✓ Stash log cleared${NC}"

# Optionally clear Compreface subjects (auto-clear if any exist)
if [ "$INITIAL_SUBJECTS" -gt 0 ]; then
    echo "[INFO] Found $INITIAL_SUBJECTS existing subjects, clearing them..."
    delete_all_subjects
    echo -e "${GREEN}✓ Compreface subjects cleared${NC}"
fi

echo ""

# Run test suites
echo -e "${BLUE}[4/6] Executing Test Suites...${NC}"
echo "=========================================="
echo ""

# Function to run a test suite
run_suite() {
    local suite_file="$1"
    local suite_name="$2"

    TOTAL_SUITES=$((TOTAL_SUITES + 1))

    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║  Suite ${TOTAL_SUITES}: ${suite_name}${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Backup database before suite (includes WAL files for consistency)
    local suite_backup=$(backup_database "${suite_name}")
    echo "[INFO] Database backed up for suite: $suite_name"

    # Clear log before suite
    clear_log

    # Run suite
    if bash "${suite_file}"; then
        PASSED_SUITES=$((PASSED_SUITES + 1))
        SUITE_RESULTS+=("${GREEN}✓${NC} ${suite_name}")
        echo ""
        echo -e "${GREEN}✓ Suite passed: ${suite_name}${NC}"
    else
        FAILED_SUITES=$((FAILED_SUITES + 1))
        SUITE_RESULTS+=("${RED}✗${NC} ${suite_name}")
        echo ""
        echo -e "${RED}✗ Suite failed: ${suite_name}${NC}"

        # Backup failed log
        backup_log "${suite_name}_FAILED"

        # Continue automatically on failure (can stop with Ctrl+C)
        echo -e "${YELLOW}Continuing with remaining suites...${NC}"

        # NOTE: Database restoration disabled - cannot restore while Stash is running
        # Would need to stop/start Stash which disrupts testing
        echo "[WARN] Database NOT restored (would crash Stash) - test state persists"
    fi

    # Wait for Stash to settle
    sleep 2

    return 0
}

# Test Suite Execution Order (logical dependency order)

# Suite 1: Foundation (Plugin registration - already tested, but verify)
if [ -f "${SUITES_DIR}/01_foundation.sh" ]; then
    run_suite "${SUITES_DIR}/01_foundation.sh" "Foundation Tests" || exit 1
fi

# Suite 2: Synchronize Performers (establishes performer-subject links)
if [ -f "${SUITES_DIR}/02_sync_performers.sh" ]; then
    run_suite "${SUITES_DIR}/02_sync_performers.sh" "Synchronize Performers" || exit 1
fi

# Suite 3: Single Image Operations (test specific operations)
if [ -f "${SUITES_DIR}/03_single_image.sh" ]; then
    run_suite "${SUITES_DIR}/03_single_image.sh" "Single Image Operations" || exit 1
fi

# Suite 4: Image Recognition (create new subjects)
if [ -f "${SUITES_DIR}/04_recognize_images.sh" ]; then
    run_suite "${SUITES_DIR}/04_recognize_images.sh" "Image Recognition" || exit 1
fi

# Suite 5: Image Identification (match to existing performers)
if [ -f "${SUITES_DIR}/05_identify_images.sh" ]; then
    run_suite "${SUITES_DIR}/05_identify_images.sh" "Image Identification" || exit 1
fi

# Suite 6: Gallery Operations
if [ -f "${SUITES_DIR}/06_gallery.sh" ]; then
    run_suite "${SUITES_DIR}/06_gallery.sh" "Gallery Operations" || exit 1
fi

# Suite 7: Cleanup Operations
if [ -f "${SUITES_DIR}/07_cleanup.sh" ]; then
    run_suite "${SUITES_DIR}/07_cleanup.sh" "Cleanup Operations" || exit 1
fi

# Suite 8: Scene Operations (skip by default - requires Vision Service)
if [ -f "${SUITES_DIR}/08_scenes.sh" ] && [ "${RUN_SCENE_TESTS:-false}" = "true" ]; then
    run_suite "${SUITES_DIR}/08_scenes.sh" "Scene Operations" || exit 1
else
    echo "[INFO] Skipping Scene Operations tests (set RUN_SCENE_TESTS=true to enable)"
fi

echo ""
echo ""

# Validate final state
echo -e "${BLUE}[5/6] Validating Final State...${NC}"
echo "----------------------------------------"

FINAL_IMAGES=$(count_images)
FINAL_PERFORMERS=$(count_performers)
FINAL_GALLERIES=$(count_galleries)
FINAL_SCENES=$(count_scenes)
FINAL_SUBJECTS=$(count_compreface_subjects)

echo ""
echo "Final State:"
echo "  Images:      $FINAL_IMAGES (Δ: $((FINAL_IMAGES - INITIAL_IMAGES)))"
echo "  Performers:  $FINAL_PERFORMERS (Δ: $((FINAL_PERFORMERS - INITIAL_PERFORMERS)))"
echo "  Galleries:   $FINAL_GALLERIES (Δ: $((FINAL_GALLERIES - INITIAL_GALLERIES)))"
echo "  Scenes:      $FINAL_SCENES (Δ: $((FINAL_SCENES - INITIAL_SCENES)))"
echo "  Subjects:    $FINAL_SUBJECTS (Δ: $((FINAL_SUBJECTS - INITIAL_SUBJECTS)))"
echo ""

# Check for errors in final log
if ! check_log_errors; then
    echo -e "${YELLOW}⚠ Errors found in Stash log${NC}"
    echo "  Review: /Users/x/dev/resources/docker/stash/config/stash.log"
fi

echo ""

# Restore to baseline (skip by default to preserve test results)
echo -e "${BLUE}[6/6] Cleanup...${NC}"
echo "----------------------------------------"

if [ "${RESTORE_BASELINE:-false}" = "true" ]; then
    restore_to_baseline
    echo -e "${GREEN}✓ Database restored to baseline${NC}"
else
    echo "[INFO] Database left in final test state (set RESTORE_BASELINE=true to restore)"
fi

# Cleanup old backups
cleanup_backups 10
echo ""

# Summary
echo ""
echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                    Test Summary                            ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

echo "Suites Executed: $TOTAL_SUITES"
echo -e "Suites Passed:   ${GREEN}$PASSED_SUITES${NC}"
echo -e "Suites Failed:   ${RED}$FAILED_SUITES${NC}"
echo ""

echo "Results by Suite:"
for result in "${SUITE_RESULTS[@]}"; do
    echo -e "  $result"
done

echo ""

# Final result
if [ $FAILED_SUITES -eq 0 ]; then
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                  ✓ ALL TESTS PASSED                        ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Review test logs in: ${DATA_DIR}/logs/"
    echo "  2. Check database backups in: ${DATA_DIR}/backup/"
    echo "  3. Verify Compreface subjects via API"
    echo ""
    exit 0
else
    echo -e "${RED}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║                  ✗ SOME TESTS FAILED                       ║${NC}"
    echo -e "${RED}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Review failed test logs:"
    ls -lh "${DATA_DIR}/logs/"*_FAILED*.log 2>/dev/null || echo "  No failed logs found"
    echo ""
    exit 1
fi
