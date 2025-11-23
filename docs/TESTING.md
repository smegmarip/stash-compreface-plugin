# Testing Guide - Stash Compreface Plugin

**Last Updated:** 2025-11-22
**Test Coverage:** 12/13 tasks (92%)

---

## Overview

This document describes the testing strategy, procedures, and results for the Stash Compreface plugin. Tests validate face recognition accuracy, batch processing, performance, and integration with external services (Compreface, Vision Service, Quality Service).

### Test Categories

1. **Unit Tests** - Component-level validation with mocks
2. **Integration Tests** - Live service interactions
3. **End-to-End Tests** - Complete task workflows from initiation to completion
4. **Performance Tests** - Batching, cooldown, memory stability

---

## Test Strategy

### Unit Testing Philosophy

**Coverage Target:** 80%+ for all packages

**Approach:**

- Test packages in isolation using mocks for external dependencies
- Focus on business logic, error handling, and edge cases
- Each test file mirrors the source file structure
- Shared test utilities and fixtures in `tests/testutil/`

**Key Areas:**

- Configuration loading and validation (`internal/config/`)
- Compreface API client operations (`internal/compreface/`)
- GraphQL mutations and queries (`internal/stash/`)
- Vision Service integration (`internal/vision/`)
- Quality assessment routing (`internal/quality/`)
- Business logic layer (`internal/rpc/`)

### Integration Testing Philosophy

**Purpose:** Validate plugin interactions with live external services

**Prerequisites:**

- Stash instance running at `http://localhost:9999`
- Compreface service at `http://localhost:8000`
- Vision Service at `http://localhost:5010` (optional, for scene tests)
- Quality Service at `http://localhost:8001` (optional)

**Test Data:**

- Test database: 1,175 images, 6 performers, 139 scenes
- Sample face images: 7 performer photos in `samples/stash/`
- Backup database: `/Users/x/dev/resources/docker/stash/config/stash-go.sqlite`

**Validation Approach:**

- Verify API responses match expected formats
- Confirm database state changes
- Check Compreface subject creation
- Validate tag application
- Monitor logs for errors

### End-to-End Testing Philosophy

**Purpose:** Validate complete task workflows as users would experience them

**Execution Strategy:**

- Run each of 11 plugin tasks in realistic scenarios
- Test with limited datasets (avoid processing all 1,175 images)
- Use `limit` parameter to cap processing (e.g., limit=50)
- Validate results in Stash UI and Compreface API
- Check for proper tag application and performer linking

**Test Isolation:**

- Database backup before each suite
- Independent test suites for each task
- Restore database if needed for reproducibility
- Clear logs between test runs

---

## Test Scenarios

### Scenario 1: Performer Synchronization

**Task:** Synchronize Performers

**Objective:** Sync existing Stash performers with Compreface by creating subjects with face images

**Prerequisites:**

- Performers exist in Stash database
- Performers have face images
- Performers have names starting with "Person" or aliases matching pattern

**Test Flow:**

1. Query Stash for performers with "Person" name/alias pattern
2. For each performer with image, create Compreface subject
3. Use subject naming format: `Person {id} {random_16_chars}`
4. Tag synchronized performers with "Compreface Synced"
5. Verify subject creation in Compreface API

**Expected Results:**

- 6 performers synchronized
- 6 Compreface subjects created
- Subject names match pattern `Person \d+ [A-Z0-9]{16}`
- Performer aliases updated
- Zero errors

**Validation:**

- Query `http://localhost:8000/api/v1/recognition/subjects`
- Check performer aliases in Stash GraphQL
- Verify "Compreface Synced" tag applied

### Scenario 2: Image Recognition (High Quality)

**Task:** Recognize Images (High Quality)

**Objective:** Detect faces in images, create subjects for new faces, match to existing performers

**Prerequisites:**

- Images without "Compreface Scanned" tag
- Compreface service available
- Quality assessment configured

**Test Flow:**

1. Query unscanned images (limit=50)
2. For each image, detect faces using Compreface Detection API
3. Filter faces by quality (min confidence, size)
4. Try to recognize faces against existing subjects
5. If match found (similarity >= 0.81), link to performer
6. If no match, create new subject
7. Tag image as "Compreface Scanned"
8. If matched, add "Compreface Matched" tag
9. Apply cooldown (10s) after each batch

**Expected Results:**

- 50 images processed
- ~37 images with faces detected (74% success rate)
- ~13 images without faces (expected)
- New subjects created for unrecognized faces
- Existing performers matched where applicable
- Tags applied correctly
- Cooldown periods observed

**Validation:**

- Count Compreface subjects before/after
- Verify tag application in Stash
- Check batch processing in logs
- Monitor cooldown messages

### Scenario 3: Image Identification (Batch)

**Task:** Identify Unscanned Images

**Objective:** Match faces in new images to existing performers without creating new subjects

**Prerequisites:**

- Performers already synchronized (Scenario 1)
- Images without "Compreface Scanned" tag
- Compreface subjects exist

**Test Flow:**

1. Query unscanned images (limit=200)
2. For each image, detect faces
3. Recognize faces against existing subjects only
4. If match found, update image performers
5. Tag all processed images as "Compreface Scanned"
6. Tag matched images as "Compreface Matched"
7. Continue on individual failures

**Expected Results:**

- 185 images processed (92.5% of limit)
- 2 faces matched to existing performers
- Similarity scores: 0.90-0.99
- Zero UpdateImage failures
- Zero 422 GraphQL errors
- Atomic processing (failures don't block batch)

**Validation:**

- Query matched images in Stash
- Verify performer associations
- Check tag counts
- Review logs for UpdateImage calls

### Scenario 4: Single Image Identification

**Task:** Identify Single Image

**Objective:** Process a specific image and detect/match all faces

**Prerequisites:**

- Known image ID with faces
- Existing performers synced

**Test Flow:**

1. Call identifyImage task with imageId=342
2. Detect all faces in image
3. For each face, try to match to existing subjects
4. Apply similarity threshold (0.81)
5. Update image with matched performers
6. Return face detection details

**Expected Results:**

- Execution time: <5 seconds
- Multiple faces detected (e.g., 3 faces)
- Threshold filtering working (reject matches < 0.81)
- Best match selected (e.g., similarity 0.99)
- Performer linked to image
- Quick single-image processing

**Validation:**

- Check image performer associations
- Review face detection details in logs
- Verify threshold filtering
- Confirm execution time

### Scenario 5: Gallery Identification

**Task:** Identify Gallery

**Objective:** Process all images within a specific gallery

**Prerequisites:**

- Gallery exists with multiple images
- Performers synchronized

**Test Flow:**

1. Query gallery by ID
2. Retrieve all images in gallery
3. For each image, detect and match faces
4. Update image performers
5. Tag images appropriately

**Expected Results:**

- All gallery images processed
- Performers matched where applicable
- Gallery-scoped processing verified
- Batch handling for large galleries

**Validation:**

- Query gallery images
- Check performer associations
- Verify tags
- Confirm gallery isolation

### Scenario 6: Scene Recognition (BLOCKED)

**Tasks:**

- Recognize New Scenes (unscanned only)
- Recognize New Scene Sprites (unscanned only)
- Recognize All Scenes (rescan partial matches)
- Recognize All Scene Sprites (rescan partial matches)

**Objective:** Extract faces from video scenes using Vision Service

**Prerequisites:**

- Vision Service running at `http://localhost:5010`
- Scenes exist in database
- Compreface service available

**Test Flow:**

1. Query unprocessed scenes (limit=5)
2. For each scene, submit job to Vision Service
3. Poll job status until complete
4. Retrieve face detections with timestamps and embeddings
5. Extract frame at representative timestamp
6. Crop face from frame
7. Recognize face in Compreface
8. Create performer if new face
9. Update scene performers and tags

**Current Status:** ⏸️ **BLOCKED**

- Vision Service not detecting faces in test videos
- Issue in Vision Service implementation (upstream project)
- Plugin code complete and ready
- Tests pass up to Vision Service call

**Expected Results (when unblocked):**

- Vision Service jobs submitted successfully
- Face detections returned with metadata
- Frame extraction working
- Face matching/creation functioning
- Scene performers updated
- Scene tags applied

**Validation:**

- Check Vision Service job results
- Verify frame extraction
- Confirm face crop quality
- Review performer associations
- Validate scene tags

### Scenario 7: Cleanup Operations

**Task:** Reset Unmatched Images

**Objective:** Remove scan tags from images that have no performer matches

**Prerequisites:**

- Images tagged as "Compreface Scanned"
- No performers associated

**Test Flow:**

1. Query images with "Compreface Scanned" tag
2. Filter for images without performers
3. Remove "Compreface Scanned" tag
4. Leave performer data unchanged

**Expected Results:**

- Unmatched images identified
- Tags removed successfully
- Performer associations preserved
- Cleanup operation completes

**Validation:**

- Count images with/without tags
- Verify tag removal
- Confirm performers unchanged

---

## Test Procedures

### Running Unit Tests

```bash
cd /Users/x/dev/resources/repo/stash-compreface-plugin/gorpc

# Run all unit tests
go test ./internal/... -v

# Run tests with coverage
go test ./internal/... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run specific package tests
go test ./internal/config -v
go test ./internal/compreface -v
go test ./internal/stash -v
```

### Running Integration Tests

```bash
cd /Users/x/dev/resources/repo/stash-compreface-plugin/gorpc

# Run integration tests (requires services running)
go test -tags=integration ./tests/integration/... -v

# Run specific integration suite
go test -tags=integration ./tests/integration/compreface -v
go test -tags=integration ./tests/integration/stash -v
```

### Running E2E Tests

**Prerequisites:**

- Stash running at `http://localhost:9999`
- Compreface running at `http://localhost:8000`
- Database backed up

**Execution:**

```bash
cd /Users/x/dev/resources/repo/stash-compreface-plugin/tests/e2e

# Run all E2E suites
./comprehensive_tests.sh

# Run individual test suite
./suites/02_synchronize_performers.sh
./suites/03_identify_unscanned_images_limited.sh
./suites/04_recognize_images.sh
./suites/05_identify_gallery.sh
./suites/06_recognize_scenes.sh  # Currently blocked
./suites/07_reset_unmatched.sh

# Run with limit parameter
LIMIT=50 ./suites/04_recognize_images.sh
```

**Test Suite Helpers:**

- `lib/database.sh` - Database backup/restore
- `lib/logging.sh` - Log monitoring
- `lib/graphql.sh` - GraphQL query helpers
- `lib/validation.sh` - Result validation

### Backup & Restore Database

```bash
# Backup before test run
cp /Users/x/dev/resources/docker/stash/config/stash-go.sqlite \
   /Users/x/dev/resources/docker/stash/config/stash-go.sqlite.backup

# Restore after tests
cp /Users/x/dev/resources/docker/stash/config/stash-go.sqlite.backup \
   /Users/x/dev/resources/docker/stash/config/stash-go.sqlite

# Restart Stash container
docker restart stash
```

---

## Test Implementation

### Directory Structure

```
gorpc/tests/
├── fixtures/              # Test data (images, responses)
├── mocks/                 # Mock implementations
├── testutil/              # Shared test utilities
├── unit/                  # Unit tests by package
├── integration/           # Integration tests
└── performance/           # Performance tests

tests/e2e/
├── comprehensive_tests.sh # Main orchestrator
├── lib/                   # Helper libraries
├── suites/                # Individual test suites
└── data/                  # Backups and logs
```

### Mock Strategy

**Mocked Dependencies:**

- Compreface HTTP client (for unit tests)
- Stash GraphQL client (for unit tests)
- Vision Service client (for unit tests)
- Quality Service client (for unit tests)

**Mock Implementations:**

- `tests/mocks/compreface_mock.go` - Mock Compreface API responses
- `tests/mocks/stash_mock.go` - Mock GraphQL operations
- `tests/mocks/vision_mock.go` - Mock Vision Service jobs
- `tests/mocks/quality_mock.go` - Mock quality assessments

### Fixture Management

**Test Fixtures:**

- Sample face images in `tests/fixtures/images/`
- Mock API responses in `tests/fixtures/responses/`
- Performer data in `tests/fixtures/performers/`

**Loading Fixtures:**

```go
// tests/testutil/fixtures.go
func LoadTestImage(name string) ([]byte, error)
func LoadMockResponse(name string) ([]byte, error)
func CreateTestPerformer() stash.Performer
```

---

## Test Results

### Current Status

**Test Coverage:** 12/13 tasks (92%)

| Task                        | Status     | Notes                        |
| --------------------------- | ---------- | ---------------------------- |
| Synchronize Performers      | ✅ PASS    | 6 performers synced          |
| Recognize Images (HQ)       | ✅ PASS    | 74% face detection rate      |
| Recognize Images (LQ)       | ⏸️ PENDING | Awaiting Vision Service v2.0 |
| Identify All Images         | ✅ PASS    | Batch identification         |
| Identify Unscanned Images   | ✅ PASS    | 200 limit, 2 matches         |
| Reset Unmatched Images      | ✅ PASS    | Cleanup operations           |
| Recognize New Scenes        | ✅ PASS    | Vision Service v1.0.0        |
| Recognize New Scene Sprites | ✅ PASS    | Sprite extraction working    |
| Recognize All Scenes        | ✅ PASS    | Rescan all scenes            |
| Recognize All Scene Sprites | ✅ PASS    | Sprite-based rescan          |
| Reset Unmatched Scenes      | ✅ PASS    | Scene cleanup operations     |
| Identify Single Image       | ✅ PASS    | Image 342, <5s               |
| Create Performer from Image | ✅ PASS    | Working correctly            |
| Identify Gallery            | ✅ PASS    | Gallery-scoped processing    |

**Note:** Task numbering skips #13 - total 13 tasks but numbered 1-14 (no task #13).

### Known Issues

**Recognize Images (LQ) - Pending Implementation:**

- **Status:** Awaiting Vision Service support for single image analysis
- **Current:** Uses direct CompreFace (same as HQ mode)
- **Future:** Will use Vision Service with lower quality thresholds
- **Blocker:** Vision Service needs image processing capability (currently video-only)
- **Impact:** LQ mode not differentiated from HQ mode

**Resolved Issues:**

- ✅ Vision Service v1.0.0 integration complete
- ✅ Occlusion detection working (~100% TPR on hands)
- ✅ Sprite extraction functional
- ✅ All scene recognition tasks passing

**Minor Issues:**

- Some images naturally have no faces (expected ~26% no-face rate)
- Threshold filtering may reject valid low-confidence matches (working as designed)

### Performance Metrics

**Batch Processing:**

- Batch size: 20 items (configurable)
- Cooldown period: 10 seconds (configurable)
- Processing rate: ~50 images/minute (with cooldowns)
- Memory usage: Stable, no leaks observed

**Single Operations:**

- Single image: <5 seconds
- Single performer sync: <3 seconds
- Gallery processing: Depends on size

---

## Troubleshooting

### Test Failures

**Compreface Connection Errors:**

- Verify Compreface running: `curl http://localhost:8000/`
- Check API keys configured in plugin settings
- Review Compreface logs: `docker logs compreface-api`

**Stash GraphQL Errors:**

- Verify Stash running: `curl http://localhost:9999/graphql`
- Check Stash logs: `docker logs stash`
- Validate query syntax with GraphQL playground

**Vision Service Unavailable:**

- Check service health: `curl http://localhost:5010/health`
- Verify container running: `docker ps | grep vision`
- Review Vision Service logs: `docker logs vision-api`

**Test Database Issues:**

- Restore from backup if corrupted
- Clear Compreface subjects if needed
- Check disk space for database writes

### Log Analysis

**Finding Test Results:**

```bash
# Plugin logs in Stash container
docker logs stash 2>&1 | grep "Plugin / Compreface"

# Filter for specific task
docker logs stash 2>&1 | grep "Recognize Images"

# Look for errors
docker logs stash 2>&1 | grep -i error | grep Compreface

# Monitor real-time
docker logs -f stash | grep Compreface
```

**Log Patterns:**

- Task start: `Starting [task name]`
- Progress: `Processing [N]/[total]`
- Cooldown: `Cooling down for N seconds`
- Match: `Matched subject ... with similarity 0.XX`
- Error: `Failed to [operation]: [reason]`

---

## Future Testing

### Planned Improvements

**Unit Test Expansion:**

- Increase coverage to 85%+
- Add property-based testing for edge cases
- Performance regression tests

**Integration Test Additions:**

- Quality Service integration suite
- Concurrent task execution tests
- Network failure scenarios
- Rate limiting tests

**E2E Test Enhancements:**

- Automated database state validation
- Performance benchmarking in E2E suites
- Error injection testing
- Long-running stability tests

### Test Automation

**CI/CD Integration:**

- Automated test runs on commit
- Coverage reporting
- Performance tracking
- Test result notifications

**Scheduled Testing:**

- Nightly full test suite runs
- Weekly performance benchmarks
- Monthly integration verification

---

## References

**Test Files:**

- Unit tests: `gorpc/tests/unit/`
- Integration tests: `gorpc/tests/integration/`
- E2E test suites: `tests/e2e/suites/`
- Test documentation: `gorpc/tests/README.md`

**Related Documentation:**

- Architecture: `docs/ARCHITECTURE.md`
- CLAUDE.md: Development guide
- README.md: User guide
- SESSION_RESUME.md: Current status and blockers

---

**Last Updated:** 2025-11-13
**Status:** 9/11 tasks tested and passing, 2/11 blocked by Vision Service
