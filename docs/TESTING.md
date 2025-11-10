# Stash Compreface Plugin - Testing Guide

**Version:** 2.0.0
**Last Updated:** 2025-11-08

---

## Table of Contents

1. [Test Environment Setup](#test-environment-setup)
2. [Unit Testing](#unit-testing)
3. [Integration Testing](#integration-testing)
4. [End-to-End Testing](#end-to-end-testing)
5. [Performance Testing](#performance-testing)
6. [Error Scenario Testing](#error-scenario-testing)
7. [Test Checklist](#test-checklist)

---

## Test Environment Setup

### Prerequisites

1. **Stash Instance**
   - Version: v0.20.0+
   - Running at: `http://localhost:9999`
   - Test database with sample data

2. **Compreface Instance**
   - Version: v1.0.0+
   - Running at: `http://localhost:8000`
   - Services configured (Recognition + Detection)

3. **Test Data**
   - 50+ test images with clear faces
   - 10+ test performers
   - 5+ test galleries
   - Sample videos (optional - for Vision Service testing)

### Environment Variables

```bash
export STASH_URL="http://localhost:9999"
export COMPREFACE_URL="http://localhost:8000"
export COMPREFACE_RECOGNITION_KEY="your_recognition_key"
export COMPREFACE_DETECTION_KEY="your_detection_key"
```

---

## Unit Testing

### Go Unit Tests

Run all unit tests:
```bash
cd gorpc
TMPDIR=/Users/x/tmp GOTMPDIR=/Users/x/tmp go test ./... -v
```

Run with coverage:
```bash
TMPDIR=/Users/x/tmp GOTMPDIR=/Users/x/tmp go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Critical Test Modules

#### 1. Subject Naming Tests
**File:** `utils_test.go`
**Tests:**
- `TestRandomSubject` - Verify random string generation
- `TestCreateSubjectName` - Verify subject name format
- `TestPersonAliasPattern` - Verify regex matching

**Expected Results:**
- Subject names match format: `Person {id} {16-char-random}`
- Random strings use only uppercase letters and digits
- Alias pattern correctly identifies "Person ..." names

#### 2. Configuration Tests
**File:** `config_test.go`
**Tests:**
- `TestLoadPluginConfig` - Configuration loading
- `TestResolveServiceURL` - DNS resolution logic
- `TestSettingDefaults` - Default value application

**Expected Results:**
- Configuration loads successfully
- DNS resolution works for localhost, IP, hostname, container names
- Defaults apply when settings not provided

#### 3. Compreface Client Tests
**File:** `compreface_client_test.go`
**Tests:**
- `TestDetectFaces` - Face detection API
- `TestRecognizeFace` - Face recognition API
- `TestAddSubject` - Subject creation API
- `TestListSubjects` - Subject listing API

**Expected Results:**
- HTTP client initializes correctly
- API requests formatted properly
- Responses parsed correctly
- Error handling works for HTTP errors

---

## Integration Testing

### Test Against Local Compreface

#### Test 1: Performer Synchronization

**Setup:**
1. Create 5 test performers in Stash with profile images
2. Add aliases in format "Person {id} {random}"

**Execute:**
```bash
# Via Stash UI: Settings → Tasks → Synchronize Performers
```

**Verify:**
- All performers with images are processed
- Compreface subjects created for each performer
- Subject names match performer aliases
- No duplicate subjects created

**Expected Output:**
```
✓ Processed 5 performers
✓ Created 5 Compreface subjects
✓ 0 errors
```

#### Test 2: Image Recognition (High Quality)

**Setup:**
1. Upload 20 high-quality images with clear faces
2. Ensure images don't have "Compreface Scanned" tag

**Execute:**
```bash
# Via Stash UI: Settings → Tasks → Recognize Images (High Quality)
```

**Verify:**
- All faces detected in images
- New performers created for unknown faces
- Images tagged as "Compreface Scanned"
- Progress updates visible in logs

**Expected Output:**
```
✓ Processed 20 images
✓ Detected 35 faces
✓ Created 12 new performers
✓ 0 errors
```

#### Test 3: Image Identification

**Setup:**
1. Have existing performers with Compreface subjects
2. Upload new images of the same people

**Execute:**
```bash
# Via Stash UI: Settings → Tasks → Identify Unscanned Images
```

**Verify:**
- Faces matched to existing performers
- Images updated with performer associations
- No duplicate performers created
- Similarity scores above threshold (0.89)

**Expected Output:**
```
✓ Processed 15 images
✓ Matched 28 faces to 8 performers
✓ Updated 15 images
✓ 0 errors
```

---

## End-to-End Testing

### Workflow Test: Full Recognition Pipeline

**Scenario:** New user with no existing data

**Steps:**

1. **Initial Setup**
   ```
   - Fresh Stash instance
   - Empty Compreface
   - 50 test images uploaded
   ```

2. **Create Sample Performers** (Manual)
   ```
   - Create 5 performers manually
   - Add profile images
   - Add aliases: "Person 1 ABC", "Person 2 DEF", etc.
   ```

3. **Synchronize Performers**
   ```
   Task: Synchronize Performers
   Expected: 5 Compreface subjects created
   ```

4. **Recognize All Images**
   ```
   Task: Recognize Images (High Quality)
   Expected:
   - 50 images processed
   - 80+ faces detected
   - 10-15 new performers created
   - All images tagged
   ```

5. **Identify Matches**
   ```
   Task: Identify All Images
   Expected:
   - Faces matched to existing performers
   - Images updated with performers
   - Matched tag applied
   ```

6. **Gallery Processing**
   ```
   Task: Identify Gallery
   Args: galleryId={test_gallery_id}
   Expected:
   - All gallery images processed
   - Performers added to images
   ```

7. **Reset Unmatched**
   ```
   Task: Reset Unmatched Images
   Expected:
   - Scan tags removed from unmatched images
   - Allows reprocessing
   ```

**Success Criteria:**
- ✅ All tasks complete without errors
- ✅ Progress updates visible
- ✅ All performers correctly identified
- ✅ No duplicate subjects/performers
- ✅ Tags applied correctly

---

## Performance Testing

### Batch Processing Test

**Objective:** Verify batching and cooldown work correctly

**Setup:**
- 200 test images
- Batch size: 20
- Cooldown: 5 seconds

**Execute:**
```bash
# Configure plugin:
# - maxBatchSize: 20
# - cooldownSeconds: 5

# Run: Recognize Images (High Quality)
```

**Monitor:**
- Batch processing occurs (20 images at a time)
- Cooldown periods applied between batches
- Memory usage stable
- No goroutine leaks

**Expected Metrics:**
- Processing time: ~10 batches × (processing + 5s cooldown)
- Memory: Stable (no continuous growth)
- CPU: Peaks during processing, drops during cooldown
- Progress: Updates smoothly from 0% to 100%

### Large Dataset Test

**Objective:** Test performance with large number of images

**Setup:**
- 1000+ images
- Batch size: 50
- Cooldown: 10 seconds

**Execute:**
- Run recognition on all images
- Monitor resource usage
- Check for timeouts or errors

**Expected Results:**
- Completes successfully (may take 30+ minutes)
- Memory remains stable
- No crashes or hangs
- Progress reporting accurate

---

## Error Scenario Testing

### Test 1: Compreface Unavailable

**Setup:**
1. Stop Compreface service

**Execute:**
```bash
# Run any task (e.g., Synchronize Performers)
```

**Expected Behavior:**
- Task fails gracefully
- Error message: "Compreface unavailable"
- No data corruption
- Task can be retried after service restart

### Test 2: Invalid API Keys

**Setup:**
1. Configure invalid API keys

**Execute:**
```bash
# Run Recognize Images
```

**Expected Behavior:**
- Task fails with authentication error
- Clear error message about API keys
- No partial data created

### Test 3: Malformed Images

**Setup:**
1. Upload corrupted/invalid image files

**Execute:**
```bash
# Run Recognize Images
```

**Expected Behavior:**
- Invalid images skipped
- Error logged for each bad image
- Processing continues with valid images
- Images tagged with error tag

### Test 4: Network Interruption

**Setup:**
1. Start recognition task
2. Disconnect network mid-process

**Execute:**
```bash
# Simulate network failure during task
```

**Expected Behavior:**
- Task fails with network error
- Partial progress saved (completed images remain tagged)
- Task can be resumed (skips already-processed images)

### Test 5: Task Cancellation

**Setup:**
1. Start long-running task (e.g., 200 images)

**Execute:**
```bash
# Cancel task via Stash UI
```

**Expected Behavior:**
- Task stops gracefully
- Partial progress saved
- No data corruption
- Task can be restarted

---

## Test Checklist

### Pre-Deployment Checklist

#### Build & Installation
- [ ] Binary compiles without errors (`./build.sh`)
- [ ] Binary is executable
- [ ] Plugin loads in Stash UI
- [ ] All 11 tasks appear in task list
- [ ] Settings page displays correctly

#### Configuration
- [ ] Default settings load correctly
- [ ] Compreface URL auto-detection works
- [ ] Service URL resolution works (DNS)
- [ ] API keys validated
- [ ] Settings persist after reload

#### Core Functionality
- [ ] Synchronize Performers - Works
- [ ] Recognize Images (HQ) - Works
- [ ] Recognize Images (LQ) - Works
- [ ] Identify All Images - Works
- [ ] Identify Unscanned Images - Works
- [ ] Identify Single Image - Works
- [ ] Create Performer from Image - Works
- [ ] Identify Gallery - Works
- [ ] Reset Unmatched Images - Works

#### Scene Recognition (If Vision Service Available)
- [ ] Recognize Scenes - Works
- [ ] Recognize Scene Sprites - Works
- [ ] Vision Service health check - Works
- [ ] Job submission and polling - Works
- [ ] Face results processed correctly

#### Performance
- [ ] Batching works (configurable size)
- [ ] Cooldown periods applied
- [ ] Progress reporting accurate
- [ ] Memory usage stable
- [ ] Task cancellation works

#### Error Handling
- [ ] Compreface unavailable - Fails gracefully
- [ ] Invalid API keys - Clear error message
- [ ] Malformed images - Skipped, logged
- [ ] Network errors - Recoverable
- [ ] Duplicate subjects - Prevented

#### Data Integrity
- [ ] Subject naming format preserved
- [ ] No duplicate Compreface subjects
- [ ] No duplicate performers in Stash
- [ ] Tags applied correctly
- [ ] Performer associations correct

### Post-Deployment Monitoring

#### Week 1
- [ ] Monitor task success rates
- [ ] Check for unexpected errors
- [ ] Verify performance metrics
- [ ] Review user feedback

#### Week 2-4
- [ ] Long-term stability check
- [ ] Resource usage trends
- [ ] Performance degradation check
- [ ] Compreface database growth

---

## Test Data Preparation

### Creating Test Images

```bash
# Download sample face dataset
wget https://example.com/face_dataset.zip
unzip face_dataset.zip -d test_images/

# Or use Stash scraping to get test data
```

### Creating Test Performers

```graphql
mutation CreatePerformer {
  performerCreate(input: {
    name: "Test Performer 1"
    alias_list: ["Person 1001 ABCD1234EFGH5678"]
  }) {
    id
    name
  }
}
```

### Resetting Test Environment

```bash
# Remove all Compreface subjects
curl -X DELETE http://localhost:8000/api/v1/recognition/subjects \
  -H "x-api-key: $COMPREFACE_RECOGNITION_KEY"

# Remove tags from images (via Stash GraphQL)
# Remove test performers
```

---

## Continuous Integration

### GitHub Actions Workflow

```yaml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: 1.21
      - name: Run tests
        run: |
          cd gorpc
          go test ./... -v -cover
      - name: Build binary
        run: ./build.sh
```

---

## Troubleshooting Tests

### Test Failures

**Issue:** Tests fail with "permission denied" errors
**Solution:** Set `TMPDIR` and `GOTMPDIR`:
```bash
export TMPDIR=/tmp
export GOTMPDIR=/tmp
go test ./...
```

**Issue:** Compreface connection timeout
**Solution:**
1. Verify Compreface is running: `curl http://localhost:8000/`
2. Check network connectivity
3. Verify API keys are correct

**Issue:** Test data not found
**Solution:**
1. Ensure test images exist in expected location
2. Check file permissions
3. Verify Stash can access image files

---

## Performance Benchmarks

### Expected Performance (Reference Hardware)

**Hardware:**
- CPU: Intel i7 / AMD Ryzen 7
- RAM: 16GB
- GPU: NVIDIA GTX 1060 or better (for Compreface)

**Metrics:**
- Face Detection: ~5-10 images/second
- Face Recognition: ~10-15 faces/second
- Batch Processing (20 images): ~10-20 seconds
- Performer Sync (100 performers): ~2-3 minutes

**Note:** Performance varies based on:
- Image resolution
- Number of faces per image
- Compreface hardware
- Network latency

---

## Contact

For test failures or issues:
- **Issues:** https://github.com/smegmarip/stash-compreface-plugin/issues
- **Discussions:** https://github.com/smegmarip/stash-compreface-plugin/discussions
