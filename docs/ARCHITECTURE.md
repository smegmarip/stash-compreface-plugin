# Stash Compreface Plugin - Architecture

**Version:** 2.1.0
**Last Updated:** 2025-11-27

---

## Overview

The Stash Compreface Plugin is a Go RPC plugin that provides face recognition and performer synchronization for Stash. It integrates with Compreface for face recognition/matching and with stash-auto-vision (Vision Service) for video/image face detection with quality assessment and occlusion filtering.

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Stash Instance                           │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │           Stash Compreface RPC Plugin (Go)                │  │
│  │                                                           │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌────────────────┐  │  │
│  │  │ RPC Server   │  │  GraphQL     │  │  Compreface    │  │  │
│  │  │ (main.go)    │  │  Client      │  │  HTTP Client   │  │  │
│  │  └──────────────┘  └──────────────┘  └────────────────┘  │  │
│  │         │                  │                  │           │  │
│  │         └──────────────────┴──────────────────┘           │  │
│  │                            │                              │  │
│  └────────────────────────────┼──────────────────────────────┘  │
│                               │                                 │
└───────────────────────────────┼─────────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
            ┌───────▼────────┐      ┌──────▼───────┐
            │   Compreface   │      │    Vision    │
            │    Service     │      │   Service    │
            │  (Face Recog)  │      │ (Detection)  │
            └────────────────┘      └──────────────┘
```

---

## Package Structure

```
gorpc/
├── main.go                    # RPC entry point (14 lines)
└── internal/
    ├── rpc/                   # Business logic (~2,400 lines)
    │   ├── handlers.go        # Task routing
    │   ├── service.go         # Service initialization
    │   ├── images.go          # Image recognition workflows
    │   ├── scenes.go          # Scene recognition workflows
    │   ├── vision.go          # Vision Service integration
    │   ├── performers.go      # Performer synchronization
    │   ├── sprites.go         # VTT/sprite extraction
    │   ├── types.go           # RPC type definitions
    │   └── utils.go           # Shared utilities
    ├── compreface/            # HTTP client (~765 lines)
    │   ├── compreface.go      # API operations
    │   ├── subjects.go        # Subject naming
    │   └── types.go           # Request/response types
    ├── stash/                 # GraphQL client (~1,100 lines)
    │   ├── client.go          # GraphQL client setup
    │   ├── images.go          # Image operations
    │   ├── scenes.go          # Scene operations
    │   ├── performers.go      # Performer operations
    │   ├── galleries.go       # Gallery operations
    │   ├── tags.go            # Tag operations with caching
    │   └── types.go           # GraphQL types
    ├── vision/                # Vision Service client (~460 lines)
    │   ├── vision.go          # API client
    │   └── types.go           # Vision types
    └── config/                # Configuration (~380 lines)
        └── config.go          # Settings, DNS resolution

Total: ~5,500 lines Go
```

---

## Core Components

### 1. RPC Task Router (`internal/rpc/handlers.go`)

Routes Stash plugin tasks to appropriate handlers.

**Task Modes (14 total):**

| Mode | Description |
|------|-------------|
| `synchronizePerformers` | Sync performers with Compreface subjects |
| `recognizeImages` | Detect and recognize faces in images |
| `identifyImagesAll` | Identify all images (batch) |
| `identifyImagesNew` | Identify new/unscanned images only |
| `resetUnmatchedImages` | Remove scan tags from unmatched images |
| `recognizeNewScenes` | New scenes via frame extraction |
| `recognizeNewSceneSprites` | New scenes via sprite sheets |
| `recognizeAllScenes` | All scenes via frame extraction |
| `recognizeAllSceneSprites` | All scenes via sprite sheets |
| `resetUnmatchedScenes` | Remove scan tags from unmatched scenes |
| `identifyImage` | Single image identification |
| `createPerformerFromImage` | Create performer from specific face |
| `identifyGallery` | Process entire gallery |

### 2. Configuration (`internal/config/`)

**Required Settings:**
- `recognitionApiKey` - Compreface recognition API key
- `detectionApiKey` - Compreface detection API key

**Optional Settings:**
- `comprefaceUrl` - Default: `http://compreface:8000`
- `visionServiceUrl` - Default: `http://vision-api:5010`
- `cooldownSeconds` - Default: 10
- `maxBatchSize` - Default: 20
- `minSimilarity` - Default: 0.81
- `minFaceSize` - Default: 64
- `minConfidenceScore` - Default: 0.7
- `minQualityScore` - Default: 0 (use component gates)
- `minProcessingQualityScore` - Default: 0 (use component gates)
- `enhanceQualityScoreTrigger` - Default: 0.5

**Service Auto-Detection:**
DNS-aware resolution supporting container names, hostnames, IPs, and localhost.

### 3. Compreface Client (`internal/compreface/`)

**Recognition API:**
- `RecognizeFacesFromBytes()` - Image-based recognition
- `RecognizeEmbedding()` - Embedding-based recognition (512-D ArcFace)
- `RecognizeEmbeddings()` - Batch embedding recognition

**Subject Management:**
- `AddSubjectFromBytes()` - Create new subject with face image
- `ListSubjects()` - Get all subjects
- `DeleteSubject()` - Remove subject

**Detection API:**
- `DetectFacesFromBytes()` - Detect faces in image

**Embedding Recognition:**
Uses Vision Service 512-D ArcFace embeddings for faster matching by sending pre-computed embeddings directly to Compreface, bypassing image re-processing. Falls back to image-based recognition if no match.

### 4. GraphQL Client (`internal/stash/`)

**Operations:**
- **Images:** `FindImages()`, `GetImage()`, `UpdateImage()`
- **Scenes:** `FindScenes()`, `GetScene()`, `UpdateScene()`
- **Performers:** `FindPerformers()`, `GetPerformerByID()`, `CreatePerformer()`, `UpdatePerformer()`, `FindPerformerBySubjectName()`
- **Galleries:** `FindGalleries()`, `GetGallery()`, `UpdateGallery()`
- **Tags:** `FindOrCreateTag()` with thread-safe caching

**Tag Management:**
Always pass complete tag lists (not deltas) when updating entities.

### 5. Vision Service Client (`internal/vision/`)

**API Version:** v1.0.0

**Operations:**
- `SubmitJob()` - Submit video/image for processing
- `WaitForCompletion()` - Poll until job complete
- `ExtractFrame()` - Extract frame at timestamp with optional enhancement
- `HealthCheck()` - Verify service availability

**Face Detection Features:**
- InsightFace RetinaFace + ArcFace (512-D embeddings)
- Quality assessment (size, pose, occlusion, sharpness)
- Occlusion detection (masks, hands, glasses) - ResNet18 ~100% TPR
- Face enhancement (CodeFormer/GFPGAN)
- Face de-duplication via embedding similarity
- Sprite-based detection (VTT + sprite images)

### 6. Sprite Processing (`internal/rpc/sprites.go`)

Extracts face thumbnails from Stash sprite sheets for scene recognition.

**Functions:**
- `ParseVTT()` - Parse WebVTT timestamp→coordinate mappings
- `FetchSpriteImage()` - Download sprite image
- `ExtractFromSprite()` - Extract thumbnail at timestamp

---

## Data Flows

### Image Recognition Flow

```
1. Query unscanned images (GraphQL)
2. For each image:
   a. Call Vision Service for face detection
   b. For each face with 512-D embedding:
      i.  Try embedding recognition (fast path)
      ii. If no match, try image-based recognition
      iii. If matched, link performer to image
      iv. If no match, create new subject + performer
   c. Tag image as "Compreface Scanned"
3. Apply cooldown between batches
```

### Scene Recognition Flow

```
1. Query unprocessed scenes (GraphQL)
2. For each scene:
   a. Submit to Vision Service (video or sprite mode)
   b. Vision Service extracts frames, detects faces, generates embeddings
   c. For each unique face:
      i.  Try embedding recognition first
      ii. If no match, extract frame, crop face, try image-based
      iii. Create performer if new face
   d. Update scene performers and tags
3. Apply cooldown between batches
```

### Performer Sync Flow

```
1. Query performers with "Person" name/alias pattern
2. For each performer with image:
   a. Download performer image
   b. Detect face with Compreface
   c. Generate subject name: "Person {id} {16-char-random}"
   d. Create Compreface subject
   e. Add subject name as performer alias
3. Tag as "Compreface Synced"
```

---

## Subject Naming Convention

**Format:** `Person {stash_id} {16-char-random}`

**Example:** `Person 12345 ABC123XYZ456GHIJ`

**Implementation:** `internal/compreface/subjects.go:CreateSubjectName()`

**Why This Format:**
1. **Uniqueness** - Random suffix prevents collisions
2. **Traceability** - Stash ID links back to source
3. **Pattern Matching** - Regex: `^Person .*$`

**CRITICAL:** Never change this format - existing Compreface databases depend on it.

---

## Performance Features

### Batching

Process items in configurable chunks (default: 20) to prevent resource exhaustion.

```go
for page := 1; ; page++ {
    items, total, err := fetchItems(page, batchSize)
    if len(items) == 0 { break }
    // Process batch...
    s.applyCooldown()
}
```

### Cooldown Periods

Prevent GPU/CPU overheating during intensive operations. Default: 10 seconds between batches.

### Progress Reporting

Real-time feedback via `log.Progress()` updates Stash UI progress bar.

### Embedding-First Recognition

Try embedding recognition before image-based for ~100ms savings per face:
1. Send 4KB embedding (vs 20-50KB image)
2. Skip face cropping and re-detection
3. Fall back to image-based if no match

---

## Error Handling

**Strategy:**
1. **Graceful Degradation** - Continue on individual failures
2. **Error Wrapping** - Add context with `fmt.Errorf`
3. **Structured Logging** - `log.Error`, `log.Warn`, `log.Debug`

**Example:**
```go
for _, image := range images {
    err := processImage(image)
    if err != nil {
        log.Warnf("Failed to process image %s: %v", image.ID, err)
        continue // Continue with next image
    }
}
```

---

## Quality Assessment

Face quality is evaluated using component-based gates or composite scoring:

**Component Gates (default when minQualityScore=0):**
- Size: ≥ 0.2
- Pose: ≥ 0.5
- Occlusion: ≥ 0.6

**Composite Mode (when minQualityScore>0):**
- Single threshold for overall quality score

**Two-Tier Quality:**
- `minProcessingQualityScore` - Lower bar for recognition attempts
- `minQualityScore` - Higher bar for subject creation

---

## Testing

See [TESTING.md](TESTING.md) for comprehensive testing procedures.

**Test Levels:**
1. **Unit Tests** - `gorpc/tests/unit/` - Component isolation with mocks
2. **Integration Tests** - `gorpc/tests/integration/` - Live service interactions
3. **E2E Tests** - `tests/e2e/` - Complete task workflows

**Coverage:** 12/13 tasks verified (92%)

---

## Deployment

### Build

```bash
cd gorpc
./build.sh          # Current platform
./build.sh linux    # Linux amd64
./build.sh all      # All platforms
```

### Installation

1. Copy plugin directory to Stash plugins folder
2. Ensure binary is executable: `chmod +x gorpc/stash-compreface-rpc`
3. Reload plugins in Stash UI
4. Configure API keys in plugin settings

### Docker Compose

```yaml
services:
  stash:
    image: stashapp/stash:latest
    volumes:
      - ./plugins:/config/plugins

  compreface:
    image: exadel/compreface:latest
    ports:
      - "8000:8000"

  vision-api:
    image: stash-auto-vision:latest
    ports:
      - "5010:5010"
```

Plugin auto-detects services at `http://compreface:8000` and `http://vision-api:5010`.

---

## References

- **Compreface API:** https://github.com/exadel-inc/CompreFace/blob/master/docs/Rest-API-description.md
- **Stash Plugin API:** https://docs.stashapp.cc/in-app-manual/plugins/
- **GraphQL Schema:** https://github.com/stashapp/stash/blob/develop/graphql/schema/schema.graphql
