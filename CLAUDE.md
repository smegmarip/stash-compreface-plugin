# Stash Compreface Plugin - Development Guide

**Status:** Production Ready (12/13 tasks complete, 92% coverage)
**Version:** 2.0.0
**Last Updated:** 2025-11-22

---

## Project Overview

Go RPC plugin for Stash that integrates with Compreface for face recognition and performer management. Features Vision Service v1.0.0 integration with occlusion detection, sprite-based face extraction, and gallery operations.

### Key Features

- **13 Plugin Tasks:** Performer sync, image recognition (HQ/LQ), identification, scene recognition (new/all, sprite/video), gallery processing, reset operations
- **Vision Service v1.0.0:** Face detection with InsightFace, occlusion filtering (ResNet18 ~100% TPR), face enhancement (CodeFormer/GFPGAN)
- **Sprite Processing:** VTT parsing and thumbnail extraction from Stash sprite sheets
- **Gallery Operations:** Complete CRUD for gallery-scoped face recognition
- **GPU Optimization:** Configurable batch sizes (default: 20) and cooldown periods (default: 10s)
- **Progress Reporting:** Real-time progress updates via `log.Progress()`
- **Tag Management:** Automatic tag creation and thread-safe caching

---

## Critical Constraints

### Subject Naming Format

**MUST PRESERVE:** `Person {id} {random_16_chars}`
**Example:** `Person 12345 ABC123XYZ456GHIJ`

Existing Compreface databases depend on this format. Changes would break performer associations on remote instances.

**Implementation:** `gorpc/internal/compreface/subject.go:CreateSubjectName()`

---

## Architecture

### Package Structure

```
internal/
├── rpc/          # Business logic (869 lines) - Task handlers, orchestration
├── stash/        # GraphQL repository (709 lines) - Tags, Images, Performers, Scenes
├── compreface/   # HTTP client (573 lines) - Face detection/recognition
├── vision/       # Vision Service (412 lines) - Video scene processing
├── quality/      # Quality assessment (1,416 lines) - Dual Go/Python implementation
└── config/       # Configuration (226 lines) - Settings, DNS resolution
```

**See `docs/ARCHITECTURE.md` for detailed component relationships**

### Domain Organization

- **RPC Layer:** Business logic and task routing (`internal/rpc/*.go`)
- **Repository Layer:** Type-safe GraphQL operations (`internal/stash/*.go`)
- **Service Layer:** External API clients (`internal/compreface/`, `internal/vision/`)
- **Utility Layer:** Shared helpers (`pkg/utils/`)

---

## Key Technical Decisions

### 1. GraphQL Type Safety

**Problem:** `go-graphql-client` type inference issues with nullable arrays
**Solution:** Use `ExecRaw()` for complex mutations, typed queries for reads

```go
// Mutations with nullable arrays - use ExecRaw
query := fmt.Sprintf(`mutation {
    performerCreate(input: {name: "%s", alias_list: %s}) { id }
}`, name, aliasJSON)
data, err := client.ExecRaw(ctx, query, nil)
```

### 2. Dual Quality Assessment

**Go-face (MMOD):** High precision, low recall - single operations
**Python dlib Service:** High recall with filtering - batch operations
**Auto Mode:** Selects detector based on operation type

**See `docs/QUALITY_SERVICE.md` for details**

### 3. Vision Service Architecture

**Decision:** Standalone `stash-auto-vision` service (separate project)
**Benefits:** InsightFace 512-D embeddings, scene segmentation, independent scaling

**See `docs/VISION_SERVICE.md` for integration details**

### 4. Service URL Resolution

DNS-aware resolution supporting container names, hostnames, IPs, and localhost.

**Implementation:** `internal/config/config.go:ResolveServiceURL()`

---

## Implementation Status

### Completed Features

✅ **Core Operations:** All 13 plugin tasks implemented
✅ **Vision Service v1.0.0:** Full integration with breaking API changes
✅ **Occlusion Detection:** ResNet18 model filtering (masks, hands, glasses)
✅ **Sprite Processing:** VTT parsing and thumbnail extraction
✅ **Gallery Operations:** Complete CRUD with tag management
✅ **Test Infrastructure:** Integration and E2E test suites

**Total:** ~5,500 lines Go + 483 lines Python
**Binary:** 8.4M
**Test Coverage:** 12/13 tasks verified (92%)

### Pending

⏸️ **Recognize Images (LQ):** Awaiting Vision Service image processing support (currently uses direct CompreFace same as HQ mode)

### Deferred/Optional

🔄 **React UI Plugin:** stash-create-performer refactor (not required for core functionality)
🔄 **Comprehensive Unit Tests:** Integration tests exist, full unit coverage deferred

---

## Quick Reference

### Build & Deploy

```bash
cd gorpc
go build -o stash-compreface-rpc     # Build
./build.sh                           # Multi-platform builds
```

### Configuration

**Minimium Required:**

- `recognitionApiKey` - Compreface recognition API key
- `detectionApiKey` - Compreface detection API key

**Performance Tuning:**

- `maxBatchSize` - Items per batch (default: 20)
- `cooldownSeconds` - Delay between batches (default: 10)
- `minSimilarity` - Face match threshold (default: 0.81)
- `minFaceSize` - Minimum face dimensions (default: 64px)

**Optional Services:**

- `visionServiceURL` - Vision Service for scene recognition (default: `http://vision-api:5010`)

### Testing

```bash
# E2E tests (requires Stash + Compreface running)
cd tests/e2e && ./run_suite.sh

# Individual task tests
./run_synchronize_performers.sh
./run_identify_images.sh
./run_recognize_scenes.sh  # Currently blocked by Vision Service issue
```

**See `docs/TESTING.md` for comprehensive testing procedures**

---

## Documentation Reference

- **Architecture:** `docs/ARCHITECTURE.md` - System design and component relationships
- **Features:**
  - `docs/COMPREFACE_INTEGRATION.md` - Face recognition, subject management
  - `docs/STASH_INTEGRATION.md` - GraphQL operations, tag/performer/scene management
  - `docs/VISION_SERVICE.md` - Video scene processing integration
  - `docs/QUALITY_SERVICE.md` - Face quality assessment strategies
- **Testing:** `docs/TESTING.md` - Test scenarios, strategies, procedures
- **User Guide:** `README.md` - Installation, configuration, usage

---

## For Future AI Sessions

### Common Patterns

**Batch Processing with Progress:**

```go
for page := 1; ; page++ {
    items, total, err := fetchItems(page, batchSize)
    if len(items) == 0 { break }

    for i, item := range items {
        progress := float64(page*batchSize+i) / float64(total)
        log.Progress(progress)
        processItem(item)  // Continue on individual failures
    }

    s.applyCooldown()  // After each batch
}
```

**Tag Operations (Always Complete Lists):**

```go
// Get current tags
scene, _ := stash.GetScene(client, sceneID)
tagIDs := []graphql.ID{}
for _, tag := range scene.Tags {
    tagIDs = append(tagIDs, tag.ID)
}

// Add new tag
tagIDs = append(tagIDs, newTagID)

// Update with complete list
stash.UpdateSceneTags(client, sceneID, tagIDs)
```

**Service Health Checks:**

```go
if err := visionClient.HealthCheck(); err != nil {
    return fmt.Errorf("Vision Service unavailable: %w", err)
}
```

### Critical Reminders

1. **Never change subject naming format** - `Person {id} {random}` is contractual
2. **Always use `ExecRaw()` for complex mutations** - Type safety issues with nullable arrays
3. **Pass complete tag/performer lists** - Not deltas, entire replacement arrays
4. **Continue on item failures** - Batch operations should complete despite individual errors
5. **Apply cooldown after batches** - Prevents GPU overheating on intensive operations

### Current Blockers

**Scene Recognition (Tasks 7-10):**

- Plugin code is complete and builds successfully
- Integration tests pass up to Vision Service call
- Vision Service not detecting faces in test videos (upstream issue)
- Next steps: Debug/fix Vision Service, then re-test all 4 scene recognition tasks

**Resolution:** Once Vision Service face detection is fixed, scene recognition should work without plugin changes.

---

## Session Bootstrap Information

**For next session:** See `SESSION_RESUME.md` for current task status, blockers, and continuation points.

**Git History:** Original 3,869-line planning document in commit history (git log --all --grep="comprehensive refactor plan")

**Project Files:** All source in `gorpc/internal/`, tests in `gorpc/tests/` and `tests/e2e/`

---

**Last Updated:** 2025-11-13
**Status:** Production-ready (9/11 tasks), Vision Service integration pending
