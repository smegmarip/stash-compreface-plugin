# Stash Compreface Plugin - Development Guide

**Status:** Production Ready (13/14 tasks complete, 93% coverage)
**Version:** 2.1.0
**Last Updated:** 2025-11-27

---

## Project Overview

Go RPC plugin for Stash that integrates with Compreface for face recognition and performer management. Features Vision Service v1.0.0 integration with occlusion detection, embedding-based recognition, sprite-based face extraction, and gallery operations.

### Key Features

- **14 Plugin Tasks:** Performer sync, image recognition, identification, scene recognition (new/all, sprite/video), gallery processing, reset operations
- **Vision Service v1.0.0:** Face detection with InsightFace, occlusion filtering (ResNet18 ~100% TPR), face enhancement (CodeFormer/GFPGAN)
- **Embedding Recognition:** 512-D ArcFace embeddings for fast matching, falls back to image-based
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

**Implementation:** `gorpc/internal/compreface/subjects.go:CreateSubjectName()`

---

## Architecture

### Package Structure

```
internal/
├── rpc/          # Business logic (~2,400 lines) - Task handlers, orchestration
├── stash/        # GraphQL repository (~1,100 lines) - Tags, Images, Performers, Scenes
├── compreface/   # HTTP client (~765 lines) - Face detection/recognition, embeddings
├── vision/       # Vision Service (~460 lines) - Video/image face detection
└── config/       # Configuration (~380 lines) - Settings, DNS resolution
```

**Total:** ~5,500 lines Go

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

### 2. Embedding-First Recognition

**Strategy:** Try embedding recognition before image-based for performance
**Benefits:** ~100ms faster per face, 4KB vs 20-50KB bandwidth

```go
// Try embedding first, fall back to image-based
if len(face.Embedding) == 512 {
    performerID, similarity, err := s.recognizeByEmbedding(face.Embedding)
    if err == nil && performerID != "" {
        return performerID, nil
    }
}
// Fall back to image-based recognition...
```

### 3. Vision Service Architecture

**Decision:** Standalone `stash-auto-vision` service (separate project)
**Benefits:** InsightFace 512-D embeddings, quality assessment, occlusion detection, independent scaling

**See `docs/VISION_SERVICE.md` for integration details**

### 4. Service URL Resolution

DNS-aware resolution supporting container names, hostnames, IPs, and localhost.

**Implementation:** `internal/config/config.go:ResolveServiceURL()`

---

## Implementation Status

### Completed Features

✅ **Core Operations:** All 14 plugin tasks implemented
✅ **Vision Service v1.0.0:** Full integration with face detection and quality assessment
✅ **Embedding Recognition:** 512-D ArcFace embedding matching with fallback
✅ **Occlusion Detection:** ResNet18 model filtering (masks, hands, glasses)
✅ **Sprite Processing:** VTT parsing and thumbnail extraction
✅ **Gallery Operations:** Complete CRUD with tag management
✅ **Test Infrastructure:** Integration and E2E test suites

**Binary:** ~8.4M
**Test Coverage:** 13/14 tasks verified (93%)

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

**Minimum Required:**

- `recognitionApiKey` - Compreface recognition API key
- `detectionApiKey` - Compreface detection API key

**Performance Tuning:**

- `maxBatchSize` - Items per batch (default: 20)
- `cooldownSeconds` - Delay between batches (default: 10)
- `minSimilarity` - Face match threshold (default: 0.81)
- `minFaceSize` - Minimum face dimensions (default: 64px)

**Quality Settings:**

- `minConfidenceScore` - Detection confidence (default: 0.7)
- `minQualityScore` - Subject creation threshold (default: 0, uses component gates)
- `minProcessingQualityScore` - Recognition attempt threshold (default: 0)
- `enhanceQualityScoreTrigger` - Face enhancement trigger (default: 0.5)

**Optional Services:**

- `frameServerUrl` - Vision Service for frame extraction (default: `http://vision-frame-server:5001`)
- `visionServiceURL` - Vision Service for face detection (default: `http://vision-api:5010`)

### Testing

```bash
# Unit tests
cd gorpc && go test ./tests/unit/... -v

# E2E tests (requires Stash + Compreface + Vision Service running)
cd tests/e2e && ./comprehensive_tests.sh
```

**See `docs/TESTING.md` for comprehensive testing procedures**

---

## Documentation Reference

- **Architecture:** `docs/ARCHITECTURE.md` - System design and component relationships
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

**Embedding Recognition Flow:**

```go
// processFace() in internal/rpc/vision.go
// 1. Quality check
// 2. Try embedding recognition (if 512-D embedding available)
// 3. Fall back to image-based recognition
// 4. Create new subject if no match
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
6. **Try embedding recognition first** - Falls back to image-based if no match

### Key Files

| File | Purpose |
|------|---------|
| `internal/rpc/handlers.go` | Task routing (14 modes) |
| `internal/rpc/vision.go` | Vision Service integration, processFace() |
| `internal/rpc/images.go` | Image recognition workflows |
| `internal/rpc/scenes.go` | Scene recognition workflows |
| `internal/compreface/compreface.go` | Compreface API client |
| `internal/compreface/types.go` | Request/response types including embeddings |
| `internal/stash/performers.go` | Performer GraphQL operations |
| `internal/vision/vision.go` | Vision Service client |

### Project Files

- **Source:** `gorpc/internal/`
- **Tests:** `gorpc/tests/` and `tests/e2e/`
- **Docs:** `docs/`

---

**Last Updated:** 2025-11-27
**Status:** Production-ready (13/14 tasks tested)
