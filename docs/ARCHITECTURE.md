# Stash Compreface Plugin - Architecture

**Version:** 2.0.0
**Last Updated:** 2025-11-22

---

## Overview

The Stash Compreface Plugin is a Go RPC plugin that provides face recognition and performer synchronization for Stash. It integrates with Compreface for face recognition/matching and optionally with stash-auto-vision for video scene face detection with occlusion filtering.

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Stash Instance                            │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │           Stash Compreface RPC Plugin (Go)                 │ │
│  │                                                            │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐ │ │
│  │  │ RPC Server   │  │  GraphQL     │  │   Compreface    │ │ │
│  │  │ (main.go)    │  │  Client      │  │   HTTP Client   │ │ │
│  │  └──────────────┘  └──────────────┘  └─────────────────┘ │ │
│  │         │                  │                   │          │ │
│  │         └──────────────────┴───────────────────┘          │ │
│  │                            │                               │ │
│  └────────────────────────────┼───────────────────────────────┘ │
│                               │                                 │
└───────────────────────────────┼─────────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
            ┌───────▼────────┐      ┌──────▼───────┐
            │   Compreface   │      │   Quality    │
            │    Service     │      │   Service    │
            │  (Face Recog)  │      │  (Optional)  │
            └────────────────┘      └──────────────┘
```

---

## Core Components

### 1. RPC Server (`main.go`)

**Purpose:** Handles Stash plugin RPC calls and routes tasks

**Key Functions:**

- `Run()` - Main task router (11 modes)
- `Stop()` - Graceful shutdown
- `errorOutput()` - Error response formatting

**Task Modes (13 total):**

1. `synchronizePerformers` - Sync performers with Compreface
2. `recognizeImagesHQ` - High-quality image recognition
3. `recognizeImagesLQ` - Low-quality image recognition
4. `identifyImagesAll` - Identify all images
5. `identifyImagesNew` - Identify new images only
6. `resetUnmatchedImages` - Reset unmatched image tags
7. `recognizeNewScenes` - New scenes via frame extraction
8. `recognizeNewSceneSprites` - New scenes via sprite sheets
9. `recognizeAllScenes` - All scenes via frame extraction (rescan)
10. `recognizeAllSceneSprites` - All scenes via sprite sheets (rescan)
11. `identifyImage` - Single image identification
12. `createPerformerFromImage` - Create performer from face
13. `identifyGallery` - Process entire gallery

**Note:** Scene tasks 7-10 support scanPartial parameter for partial match rescanning and method-based face extraction (sprite vs video).

### 2. Configuration System (`config.go`)

**Purpose:** Load and validate plugin settings

**Key Functions:**

- `loadPluginConfig()` - Fetch settings from Stash
- `resolveServiceURL()` - DNS resolution with auto-detection
- `getPluginConfiguration()` - GraphQL query for settings

**Settings:**

- Required: `recognitionApiKey`, `detectionApiKey`
- Optional: `comprefaceUrl`, `visionServiceUrl`, `cooldownSeconds`, `maxBatchSize`, `minSimilarity`, `minFaceSize`, `scannedTagName`, `matchedTagName`

**Auto-Detection:**

- Compreface: `http://compreface:8000` (Docker Compose)
- Vision Service: `http://vision-api:5010` (Docker Compose)

### 3. Compreface HTTP Client (`compreface_client.go`)

**Purpose:** Interface with Compreface REST API

**API Methods:**

**Recognition Service:**

- `RecognizeFace(embedding)` - Match face to existing subjects
- `AddSubject(subjectName, imageBytes)` - Create new subject
- `AddExample(subjectName, imageBytes)` - Add example to subject
- `ListSubjects()` - Get all subjects
- `DeleteSubject(subjectName)` - Remove subject
- `GetSubjectExamples(subjectName)` - List subject examples

**Detection Service:**

- `DetectFaces(imageBytes)` - Detect faces in image

**Verification Service:**

- `VerifyFace(sourceBytes, targetBytes)` - Compare two faces

**Features:**

- Base64 image encoding
- Multipart form data uploads
- Structured error handling
- Configurable similarity threshold

### 4. GraphQL Client (Multiple Files)

**Purpose:** Interface with Stash database via GraphQL

**Tag Operations (`tags.go`):**

- `findOrCreateTag(name)` - Get or create tag by name
- Tag caching for performance

**Image Operations (`images.go`):**

- `findImages(filter, page, perPage)` - Query images
- `getImage(id)` - Fetch single image
- `updateImage(id, tags, performers)` - Update image metadata

**Performer Operations (`performers.go`):**

- `findPerformers(filter)` - Query performers
- `getPerformer(id)` - Fetch single performer
- `createPerformer(name, aliases)` - Create new performer
- `updatePerformer(id, data)` - Update performer metadata

**Scene Operations (`scenes.go`):**

- `findScenes(filter, page, perPage)` - Query scenes
- `getScene(id)` - Fetch single scene
- `updateScenePerformers(id, performerIDs)` - Update scene metadata

### 5. Core Business Logic

**Face Detection (`face_detection.go`):**

- Image processing and face detection
- Bounding box validation
- Quality filtering

**Face Recognition (`face_recognition.go`):**

- `recognizeImages(lowQuality)` - Batch image recognition
- Face-to-performer matching
- Subject creation for unknown faces

**Performer Sync (`performers.go`):**

- `synchronizePerformers()` - Sync performers to Compreface
- Alias management ("Person {id} {random}" format)
- Performer image detection

**Image Identification (`images.go`):**

- `identifyImages(newOnly)` - Match faces to performers
- `identifyImage(id, createPerformer, faceIndex)` - Single image
- `identifyGallery(id, createPerformer)` - Batch gallery

**Scene Recognition (`scenes.go`):**

- `recognizeNewScenes(useSprites)` - Video face recognition
- Vision Service integration
- Frame extraction and face processing

### 6. Vision Service Client (`vision_service_client.go`)

**Purpose:** Interface with stash-auto-vision service for video/image face detection

**API Version:** v1.0.0 (breaking changes from v0.x)

**API Methods:**

- `SubmitJob(request)` - Submit video/image for processing
- `GetJobStatus(jobID)` - Poll job progress
- `GetResults(jobID)` - Retrieve detected faces
- `WaitForCompletion(jobID, callback)` - Block until complete
- `ExtractFrame(source, timestamp, enhancement)` - Extract frame/thumbnail
- `HealthCheck()` - Verify service availability

**Key Changes in v1.0.0:**

- Breaking: `video_path` → `source` (supports images and videos)
- Breaking: `min_confidence` → `face_min_confidence`
- New: `face_min_quality` parameter for quality filtering
- New: Face occlusion detection (masks, hands, glasses)
- New: Face enhancement metadata (CodeFormer/GFPGAN)
- New: Sprite-based detection (VTT + sprite images)

**Data Flow:**

1. Plugin submits source (video/image/sprite) to Vision Service
2. Service extracts frames/thumbnails (FFmpeg/PyAV/VTT parsing)
3. Service detects faces using InsightFace (RetinaFace + ArcFace)
4. Service performs occlusion detection using ResNet18 (~100% TPR on hands)
5. Service optionally enhances low-quality faces (CodeFormer/GFPGAN)
6. Service performs face de-duplication via embedding similarity
7. Service returns: embeddings, quality scores, demographics, occlusion flags
8. Plugin filters occluded faces
9. Plugin matches embeddings to Compreface subjects
10. Plugin creates performers and updates scenes/images

**Status:** ✅ Production ready with v1.0.0 integration

### 7. Quality Service (Optional)

**Purpose:** Enhanced face quality assessment using dlib

**Architecture:**

- Standalone Python Flask service
- Docker containerization
- Optional dependency (not required for core functionality)

**API Endpoints:**

- `POST /quality/assess` - Assess face quality
- `POST /quality/preprocess` - Enhance image
- `POST /quality/detect` - Enhanced detection with quality metrics

**Features:**

- 68-point facial landmarks
- Face alignment
- Quality scoring (pose, blur, brightness)
- Cropping with padding

**Status:** Implemented and tested (Phase 1.5)

### 8. Sprite Processing (`sprites.go`)

**Purpose:** Extract face thumbnails from Stash sprite sheets

**Functions:**

- `ParseVTT(content)` - Parse WebVTT timestamp→coordinate mappings
- `FindCueForTimestamp(cues, timestamp)` - Locate sprite region
- `FetchSpriteImage(url)` - Download sprite image
- `FetchVTT(url)` - Download VTT file
- `ExtractThumbnailFromSprite(img, cue)` - Extract thumbnail region
- `ExtractFromSprite(spriteURL, vttURL, timestamp)` - Main workflow

**Workflow:**

1. Parse VTT file for timestamp→(x,y,w,h) mappings
2. Download sprite image (grid of thumbnails)
3. Extract specific thumbnail using VTT coordinates
4. Return thumbnail bytes for face detection/recognition

**Use Case:** Scene recognition with `useSprites=true` parameter

**Status:** ✅ Production ready

### 9. Gallery Operations (`galleries.go`)

**Purpose:** Gallery-specific GraphQL operations

**Functions:**

- `FindGalleries(filter, page, perPage)` - Query with pagination
- `GetGallery(galleryID)` - Retrieve single gallery
- `UpdateGallery(galleryID, input)` - Update gallery
- `AddTagToGallery(galleryID, tagID)` - Add tag (append)
- `UpdateGalleryTags(galleryID, tagIDs)` - Replace all tags
- `RemoveTagFromGallery(galleryID, tagID)` - Remove tag (preserve others)

**Integration:** Used by `identifyGallery` task for gallery-scoped face recognition

**Status:** ✅ Production ready

---

## Data Flow

### Image Recognition Flow

```
1. User triggers "Recognize Images (HQ)" task
2. Plugin queries Stash for unscanned images (GraphQL)
3. For each image:
   a. Download image from Stash
   b. Call Compreface Detection API → get faces
   c. Filter faces by size and quality
   d. For each detected face:
      i. Call Compreface Recognition API → match existing subjects
      ii. If no match → create new subject
      iii. Create Stash performer for subject
      iv. Link performer to image
   e. Tag image as "Compreface Scanned"
4. Apply cooldown period (default: 10 seconds)
5. Repeat for next batch (default: 20 images)
```

### Performer Synchronization Flow

```
1. User triggers "Synchronize Performers" task
2. Plugin queries all performers with images (GraphQL)
3. For each performer:
   a. Check for "Person ..." alias → already synced, skip
   b. Download performer profile image from Stash
   c. Call Compreface Detection API → detect face
   d. Generate subject name: "Person {performer_id} {16-char-random}"
   e. Call Compreface Add Subject API → create subject
   f. Add subject name as performer alias (GraphQL)
4. Apply cooldown period
5. Repeat for next batch
```

### Video Recognition Flow (Requires Vision Service)

```
1. User triggers scene recognition task
2. Plugin queries scenes based on task type (GraphQL)
3. For each scene:
   a. Submit video path to Vision Service
   b. Vision Service:
      i. Extract frames at intervals (FFmpeg)
      ii. Detect faces in frames (InsightFace RetinaFace)
      iii. Generate 512-D embeddings (InsightFace ArcFace)
      iv. De-duplicate faces by embedding similarity
      v. Select representative detection (best quality)
   c. Plugin receives face results:
      - Face ID
      - 512-D embedding
      - Representative detection (timestamp, bbox, quality)
      - All detections
   d. For each face:
      i. Extract frame at representative timestamp
      ii. Crop face from frame
      iii. Submit to Compreface for recognition
      iv. If matched → link performer to scene
      v. If not matched → create new subject + performer
   e. Tag scene as "Compreface Scanned"
4. Apply cooldown period
5. Repeat for next batch
```

---

## Subject Naming Convention

### Format

```
Person {stash_id} {16-char-random}
```

### Example

```
Person 12345 ABC123XYZ456GHIJ
```

### Implementation

**Go:**

```go
func randomSubject(length int, prefix string) string {
    const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, length)
    for i := range b {
        b[i] = charset[rand.Intn(len(charset))]
    }
    return prefix + string(b)
}

// Usage:
subject := randomSubject(16, fmt.Sprintf("Person %s ", imageID))
```

**Python (original):**

```python
def random_subject(length=16, prefix=""):
    characters = string.ascii_uppercase + string.digits
    random_string = "".join(random.choice(characters) for _ in range(length))
    return f"{prefix}{random_string}"

# Usage:
subject = random_subject(16, f"Person {image['id']} ")
```

### Why This Format

1. **Uniqueness:** Random suffix prevents collisions
2. **Traceability:** Stash ID allows linking back to source
3. **Compatibility:** Matches existing Python plugin format
4. **Sync Support:** Pattern matching via regex: `^Person .*$`

---

## Performance Features

### 1. Batching

**Purpose:** Process items in chunks to prevent resource exhaustion

**Configuration:**

- Setting: `maxBatchSize` (default: 20)
- Applied to: All batch operations (images, performers, scenes)

**Implementation:**

```go
batchSize := a.config.MaxBatchSize
for page := 1; ; page++ {
    items, total, err := a.fetchItems(page, batchSize)
    if len(items) == 0 {
        break
    }
    // Process batch...
}
```

### 2. Cooldown Periods

**Purpose:** Prevent GPU/CPU overheating during intensive operations

**Configuration:**

- Setting: `cooldownSeconds` (default: 10)
- Applied after: Each batch completion

**Implementation:**

```go
func (a *ComprefaceAPI) applyCooldown() {
    if a.config.CooldownSeconds > 0 {
        log.Infof("Cooling down for %d seconds...", a.config.CooldownSeconds)
        time.Sleep(time.Duration(a.config.CooldownSeconds) * time.Second)
    }
}
```

### 3. Progress Reporting

**Purpose:** Provide real-time feedback on long-running tasks

**Implementation:**

```go
progress := float64(current) / float64(total)
log.Progress(progress) // Updates Stash UI progress bar
```

### 4. Task Cancellation

**Purpose:** Allow users to stop long-running tasks gracefully

**Implementation:**

```go
func (a *ComprefaceAPI) Stop(input struct{}, output *bool) error {
    a.stopping = true
    return nil
}

// Check in loops:
if a.stopping {
    return fmt.Errorf("operation cancelled")
}
```

---

## Error Handling

### Strategy

1. **Graceful Degradation:** Continue processing on individual failures
2. **Structured Logging:** Use `log.Error`, `log.Warn` for visibility
3. **Error Wrapping:** Add context with `fmt.Errorf`
4. **Error Tagging:** Mark failed items with error tags

### Examples

**Item-Level Error:**

```go
for _, image := range images {
    err := a.processImage(image)
    if err != nil {
        log.Warnf("Failed to process image %s: %v", image.ID, err)
        a.addErrorTag(image.ID)
        continue // Continue with next image
    }
}
```

**Service Unavailable:**

```go
if err := visionClient.HealthCheck(); err != nil {
    return fmt.Errorf("Vision Service unavailable: %w", err)
}
```

**API Error:**

```go
if resp.StatusCode != 200 {
    return &APIError{
        StatusCode: resp.StatusCode,
        Body:       string(body),
    }
}
```

---

## Testing Architecture

See [TESTING.md](TESTING.md) for comprehensive testing procedures.

**Test Levels:**

1. **Unit Tests:** Go test suite for core functions
2. **Integration Tests:** Test against local Compreface
3. **End-to-End Tests:** Full workflow testing
4. **Performance Tests:** Batch processing, memory usage

---

## Deployment

### Binary Build

```bash
./build.sh          # Current platform
./build.sh linux    # Linux amd64
./build.sh all      # All platforms
```

### Installation

1. Copy plugin directory to Stash plugins folder
2. Ensure binary is executable: `chmod +x gorpc/stash-compreface-rpc`
3. Reload plugins in Stash UI
4. Configure settings (API keys, URLs)

### Docker Compose Integration

```yaml
services:
  stash:
    image: stashapp/stash:latest
    volumes:
      - ./plugins:/config/plugins
    # ...

  compreface:
    image: exadel/compreface:latest
    ports:
      - "8000:8000"
    # ...
```

**Plugin auto-detects** Compreface at `http://compreface:8000` when URL not configured.

---

## Future Enhancements

### Vision Service Integration

**Status:** Client complete, service implementation deferred

**When Available:**

- Enhanced video face recognition
- Frame extraction with adaptive sampling
- InsightFace 512-D embeddings (higher accuracy than dlib 128-D)
- Scene segmentation support

### UI Plugin

**Status:** Not started (Phase 7 - optional)

**Goals:**

- React-based face selection modal
- Create performers from detected faces in UI
- Real-time job progress visualization

---

## References

- **Compreface API:** https://github.com/exadel-inc/CompreFace/blob/master/docs/Rest-API-description.md
- **Stash Plugin API:** https://docs.stashapp.cc/in-app-manual/plugins/
- **GraphQL Schema:** https://github.com/stashapp/stash/blob/develop/graphql/schema/schema.graphql
