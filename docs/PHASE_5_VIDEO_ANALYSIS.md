# Phase 5: Video Face Recognition - Solution Analysis

**Date:** 2025-11-08
**Status:** Research & Architecture Decision
**Purpose:** Determine optimal approach for video face recognition in stash-compreface-plugin

---

## Executive Summary

This document analyzes three approaches for video face recognition:
1. **Existing Python Implementation** (video_recognition.py + common.py)
2. **Advanced Vision Techniques** (stash-auto-vision research)
3. **Independent Go Implementation** (for current plugin)

**Recommendation Preview:** Hybrid approach - Use existing Python Quality Service + add sprite/frame extraction in Go.

---

## 1. Existing Python Implementation Analysis

### Overview

**Files:** `video_recognition.py` (250 lines), `common.py` (capture functions ~270 lines)
**Dependencies:** dlib, OpenCV, face_recognition, numpy
**Status:** Functional, battle-tested

### Key Components

#### 1.1 Sprite Sheet Processing (`capture_sprites`)

**Method:** Parse Stash-generated sprite sheets (VTT + image grid)

**Workflow:**
1. Download sprite sheet image (JPEG grid of video frames)
2. Download VTT file (WebVTT with x,y,w,h coordinates for each frame)
3. Parse VTT to extract bounding boxes: `#xywh=(\d+),(\d+),(\d+),(\d+)`
4. Extract individual frames from grid using coordinates
5. For each frame:
   - Convert to grayscale
   - Apply histogram equalization (cv2.equalizeHist)
   - Detect faces using dlib MMOD detector
   - Calculate 128-D face embeddings via dlib ResNet
   - De-duplicate faces using L2 distance threshold (0.6)
   - Quality filtering: confidence + pose (front/left/right)
   - Save best quality frame per unique face

**Advantages:**
- ✅ Fast - No video decoding required
- ✅ Efficient - Stash already generates sprites
- ✅ Comprehensive - Samples entire video
- ✅ De-duplication - Tracks unique faces across frames

**Disadvantages:**
- ❌ Fixed sampling - Limited to sprite density (typically ~100 frames)
- ❌ Low resolution - Sprite frames are downscaled (typically 160x90)
- ❌ Quality loss - JPEG compression artifacts

#### 1.2 Frame Extraction (`capture_frames`)

**Method:** Direct video decoding with adaptive sampling

**Workflow:**
1. Open video with cv2.VideoCapture
2. Get FPS and total frames
3. Initial sampling interval: `frame_rate * OPENCV_MIN_INTERVAL`
4. For each frame:
   - Skip frames based on adaptive interval
   - Convert to RGB and grayscale
   - Apply histogram equalization
   - Detect faces using dlib MMOD
   - Calculate 128-D embeddings
   - De-duplicate (L2 distance < 0.6)
   - Quality filter (confidence + pose)
   - **Adaptive sampling:**
     - If face detected: interval ÷= 2 (sample more frequently)
     - If no face: interval += 1 (skip more frames)
   - Stop when max faces reached (OPENCV_MAX_FACES)

**Advantages:**
- ✅ Adaptive - Focuses on face-dense regions
- ✅ High quality - Full resolution frames
- ✅ Flexible - Can target specific intervals
- ✅ Smart stopping - Exits early when enough faces found

**Disadvantages:**
- ❌ Slow - Video decoding overhead
- ❌ CPU intensive - Frame extraction + processing
- ❌ Redundant - Processes many similar frames

#### 1.3 Face Quality Assessment

**Quality Metrics:**
- **Confidence Score:** dlib MMOD detector score (0-inf, typically 0-3)
- **Pose Type:** front, front-rotate-left, front-rotate-right, left, right
- **Face Size:** Minimum dimension in pixels
- **Mask Detection:** whether person is wearing mask

**Quality Thresholds (from config):**
```python
OPENCV_CONFIDENCE = 2.0        # High quality threshold
OPENCV_LOW_CONFIDENCE = 0.8    # Low quality threshold (for low_quality mode)
MIN_FACE_SIZE = 64             # Minimum face dimension (pixels)
```

**Selection Logic:**
```python
if (confidence >= OPENCV_CONFIDENCE and "front" in pose) or not high_quality:
    select_frame()
elif confidence >= OPENCV_LOW_CONFIDENCE and pose == "front" and low_quality:
    select_frame()
```

**De-duplication:**
- Track all detected faces with 128-D embeddings
- New face if: `np.linalg.norm(known_encoding - face_encoding) >= 0.6`
- Replace existing face if: same person but better quality score

#### 1.4 Performance Characteristics

**Sprite Processing:**
- Time: ~2-5 seconds per video (100 frames)
- Faces found: 1-5 unique faces typically
- Quality: Lower (downscaled, compressed)

**Frame Extraction:**
- Time: ~10-60 seconds per video (depends on length, adaptive sampling)
- Faces found: 1-5 unique faces typically
- Quality: Higher (full resolution)

**Resource Usage:**
- CPU: Heavy (dlib MMOD + ResNet inference)
- Memory: ~500MB for models
- Disk: Temporary frame files (cleaned up after)

### Key Strengths

1. **Battle-tested:** Used in production, handles edge cases
2. **Quality-aware:** Sophisticated confidence + pose filtering
3. **De-duplication:** Tracks unique faces across entire video
4. **Adaptive sampling:** Focuses compute on face-dense regions
5. **Histogram equalization:** Improves detection in low-light scenarios

### Key Weaknesses

1. **Python-only:** Cannot be embedded in Go plugin
2. **Heavy dependencies:** dlib models (~100MB), OpenCV, numpy
3. **Local detection:** Uses local dlib instead of Compreface API
4. **Synchronous:** Blocking operations, no async/concurrency

---

## 2. Advanced Vision Techniques Analysis (stash-auto-vision)

### Overview

**Source:** `/Users/x/dev/resources/repo/stash-auto-vision/CLAUDE.md`
**Scope:** Broader than face recognition - full scene analysis
**Models:** CLIP, InsightFace, YOLO-World, PySceneDetect
**Status:** Research phase, not implemented

### Relevant Components for Face Recognition

#### 2.1 InsightFace - State-of-the-Art Face Recognition

**Repository:** https://github.com/deepinsight/insightface
**License:** MIT
**Accuracy:** 99.86% on LFW dataset

**Key Features:**
- RetinaFace detection (better than dlib MMOD)
- ArcFace/SubCenter-ArcFace recognition
- 512-D embeddings (vs dlib's 128-D)
- GPU acceleration
- Age/gender/emotion analysis

**Performance:**
- Detection: ~30 FPS on GPU, ~5 FPS on CPU
- Recognition: ~1000 embeddings/second on GPU
- VRAM: ~500MB (buffalo_s) to ~1GB (buffalo_l)

**API Example:**
```python
# Detect faces
faces = insightface.detect(frame)
# Returns: [{"bbox": [x, y, w, h], "confidence": 0.99, "landmarks": [...]}]

# Generate embedding
embedding = insightface.get_embedding(face_image)
# Returns: 512-dimensional vector

# Analyze attributes
attributes = insightface.analyze(face_image)
# Returns: {"age": 28, "gender": "female", "emotion": "happy"}
```

#### 2.2 PySceneDetect - Video Scene Splitting

**Repository:** https://github.com/Breakthrough/PySceneDetect
**License:** BSD 3-Clause

**Key Features:**
- Detect shot changes and scene transitions
- Multiple detection algorithms (Content, Threshold, Adaptive, Hash)
- Automatic video splitting
- 100-500 FPS processing speed

**Use Case:**
- Split multi-scene videos BEFORE face recognition
- Focus analysis on actual scene boundaries
- Avoid redundant processing of static scenes

#### 2.3 Proposed Architecture

**Docker Compose Services:**
- `face-recognition-server` (Port 5200) - InsightFace API
- `clip-server` (Port 5100) - Scene classification
- `object-detection-server` (Port 5300) - Object detection

**Face Recognition API:**
```python
POST /detect
{
    "image_path": "/media/scene.mp4",
    "timestamp": 30.5
}
Response: {"faces": [{"bbox": [...], "confidence": 0.99}]}

POST /recognize
{
    "image_path": "/media/scene.mp4",
    "timestamp": 30.5,
    "min_confidence": 0.8
}
Response: {"matches": [{"performer_id": "123", "confidence": 0.95}]}
```

### Advantages Over Current Implementation

1. **Better Accuracy:** InsightFace (99.86%) vs dlib (~97%)
2. **GPU Acceleration:** 30 FPS vs 5 FPS
3. **Modern Embeddings:** 512-D (more discriminative) vs 128-D
4. **Richer Metadata:** Age, gender, emotion analysis
5. **Scene Intelligence:** Automatic scene splitting
6. **Service Architecture:** Microservices can be reused by multiple plugins

### Disadvantages

1. **External Dependency:** Requires separate service deployment
2. **Complexity:** More moving parts (Docker, networking, coordination)
3. **Latency:** HTTP overhead for service calls
4. **Resource:** Requires GPU for optimal performance
5. **Not Yet Built:** Research phase, no implementation

---

## 3. Independent Go Approach Investigation

### Overview

**Goal:** Implement video face recognition natively in Go plugin
**Scope:** Sprite parsing + frame extraction + quality assessment
**Integration:** Use existing Compreface API for face detection/recognition

### 3.1 Technical Requirements

**Must Have:**
1. Parse Stash sprite sheets (VTT + JPEG grid)
2. Extract frames from video using ffmpeg
3. De-duplicate faces across frames
4. Quality filtering (size, pose, confidence)
5. Call Compreface API for detection/recognition
6. Progress reporting and cancellation support

**Nice to Have:**
1. Adaptive sampling based on face density
2. Shot change detection
3. Parallel processing of frames
4. Streaming results

### 3.2 Go Libraries Available

#### Video Processing

**1. ffmpeg-go** - https://github.com/u2takey/ffmpeg-go
- FFmpeg bindings for Go
- Frame extraction at specific timestamps
- Requires ffmpeg binary on system

**2. goav** - https://github.com/giorgisio/goav
- CGO bindings to FFmpeg libraries
- More control, but requires compilation with FFmpeg

**3. exec ffmpeg directly:**
```go
cmd := exec.Command("ffmpeg",
    "-i", videoPath,
    "-vf", fmt.Sprintf("select='eq(n,%d)'", frameNumber),
    "-vframes", "1",
    "-f", "image2",
    outputPath)
```

#### Image Processing

**1. Standard library:** `image`, `image/jpeg`, `image/png`
- Built-in, no dependencies
- Basic operations (decode, crop, resize)

**2. imaging** - https://github.com/disintegration/imaging
- Crop, resize, rotate, flip
- No CGO dependencies

**3. gift** - https://github.com/disintegration/gift
- Image filters (blur, sharpen, contrast)
- Histogram equalization equivalent

#### Face Quality Assessment Options

**Option A: Use Existing Python Quality Service**
- Reuse quality_service (port 8001)
- Already has dlib detection + quality scoring
- Add video frame endpoints

**Option B: Port Quality Logic to Go**
- Use go-face (https://github.com/Kagami/go-face)
- CGO bindings to dlib
- Complex build requirements

**Option C: Use Compreface Only**
- No local quality assessment
- Rely on Compreface confidence scores
- Simpler, but less control

### 3.3 Proposed Go Implementation

**File Structure:**
```
gorpc/
├── video/
│   ├── sprite_parser.go      # Parse VTT + extract sprite frames
│   ├── frame_extractor.go    # Extract frames via ffmpeg
│   ├── face_deduplicator.go  # Track unique faces
│   └── quality_client.go     # Optional: call quality service
├── scenes.go                 # Main scene recognition logic
```

**Sprite Parser:**
```go
type SpriteFrame struct {
    Index int
    X, Y, W, H int
    Image image.Image
}

func ParseSpriteSheet(spriteURL, vttURL string) ([]SpriteFrame, error) {
    // 1. Download sprite image
    resp, err := http.Get(spriteURL)
    img, err := jpeg.Decode(resp.Body)

    // 2. Download VTT file
    resp, err = http.Get(vttURL)
    vttContent, err := io.ReadAll(resp.Body)

    // 3. Parse VTT coordinates
    regex := regexp.MustCompile(`#xywh=(\d+),(\d+),(\d+),(\d+)`)
    coords := regex.FindAllStringSubmatch(string(vttContent), -1)

    // 4. Extract frames from grid
    frames := []SpriteFrame{}
    for i, coord := range coords {
        x, _ := strconv.Atoi(coord[1])
        y, _ := strconv.Atoi(coord[2])
        w, _ := strconv.Atoi(coord[3])
        h, _ := strconv.Atoi(coord[4])

        // Crop sprite
        subImg := img.(SubImager).SubImage(image.Rect(x, y, x+w, y+h))
        frames = append(frames, SpriteFrame{i, x, y, w, h, subImg})
    }

    return frames, nil
}
```

**Frame Extractor:**
```go
func ExtractFrame(videoPath string, timestamp float64, outputPath string) error {
    cmd := exec.Command("ffmpeg",
        "-ss", fmt.Sprintf("%.2f", timestamp),
        "-i", videoPath,
        "-vframes", "1",
        "-q:v", "2",  // High quality JPEG
        "-y",
        outputPath)

    return cmd.Run()
}

func ExtractFramesAdaptive(videoPath string, fps int, totalFrames int) ([]string, error) {
    interval := fps * 5  // Start with 5-second intervals
    framePaths := []string{}

    for frameNum := 0; frameNum < totalFrames; frameNum += interval {
        timestamp := float64(frameNum) / float64(fps)
        outputPath := fmt.Sprintf("/tmp/frame_%d.jpg", frameNum)

        err := ExtractFrame(videoPath, timestamp, outputPath)
        if err != nil {
            continue
        }

        framePaths = append(framePaths, outputPath)
    }

    return framePaths, nil
}
```

**Face Deduplicator:**
```go
type FaceTracker struct {
    knownFaces []FaceEmbedding
    mu         sync.Mutex
}

type FaceEmbedding struct {
    Embedding []float64
    BestFrame string
    BestScore float64
}

func (ft *FaceTracker) IsUniqueFace(embedding []float64, threshold float64) bool {
    ft.mu.Lock()
    defer ft.mu.Unlock()

    for _, known := range ft.knownFaces {
        similarity := cosineSimilarity(embedding, known.Embedding)
        if similarity > threshold {
            return false  // Already seen this face
        }
    }

    return true
}

func cosineSimilarity(a, b []float64) float64 {
    dotProduct := 0.0
    normA := 0.0
    normB := 0.0

    for i := range a {
        dotProduct += a[i] * b[i]
        normA += a[i] * a[i]
        normB += b[i] * b[i]
    }

    return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

**Main Scene Recognition:**
```go
func (a *ComprefaceAPI) recognizeScenes(useSprites bool) error {
    // Get unscanned scenes
    scenes, err := a.findScenesWithoutTag(scannedTagID)

    for i, scene := range scenes {
        log.Progress(float64(i) / float64(len(scenes)))

        var frames []string
        if useSprites {
            // Parse sprite sheet
            sprites, err := ParseSpriteSheet(scene.SpriteURL, scene.VttURL)
            frames = saveSpritesToTemp(sprites)
        } else {
            // Extract frames via ffmpeg
            frames, err = ExtractFramesAdaptive(scene.Path, scene.FPS, scene.TotalFrames)
        }

        // Process frames
        tracker := NewFaceTracker()
        for _, framePath := range frames {
            // Detect faces via Compreface
            detections, err := a.client.DetectFaces(framePath)

            for _, face := range detections.Result {
                // Check if unique face
                if tracker.IsUniqueFace(face.Embedding, 0.6) {
                    // Recognize via Compreface
                    recognition, err := a.client.RecognizeFaces(framePath)

                    // Match to performer or create new subject
                    a.processSceneFace(scene.ID, face, recognition)

                    tracker.AddFace(face.Embedding, framePath, face.Confidence)
                }
            }

            os.Remove(framePath)  // Cleanup
        }

        // Add scanned tag
        a.addTagToScene(scene.ID, scannedTagID)

        // Cooldown after each scene
        a.applyCooldown()
    }

    return nil
}
```

### 3.4 Quality Assessment Integration

**Option A: Python Quality Service (Recommended)**

Add video endpoints to existing quality_service:

```python
# quality_service/app.py

@app.route('/quality/assess_video_frame', methods=['POST'])
def assess_video_frame():
    """
    Assess quality of a frame from video.
    Input: {"frame_base64": "...", "faces": [...]}
    Output: {"faces": [{"index": 0, "quality_score": 0.87, "pose": "front", ...}]}
    """
    data = request.json
    frame_bytes = base64.b64decode(data['frame_base64'])
    frame = load_image_from_bytes(frame_bytes)

    quality_faces = []
    for face_data in data['faces']:
        # Use existing dlib quality assessment
        confidence = dlib_confidence_score(frame, face_data['box'])
        quality_faces.append({
            "index": face_data['index'],
            "quality_score": confidence['score'],
            "pose": confidence['type'],
            "is_frontal": "front" in confidence['type']
        })

    return jsonify({"faces": quality_faces})
```

Go client:
```go
type QualityClient struct {
    BaseURL string
}

func (qc *QualityClient) AssessVideoFrame(framePath string, faces []FaceDetection) ([]FaceQuality, error) {
    // Read frame
    frameBytes, err := os.ReadFile(framePath)
    frameB64 := base64.StdEncoding.EncodeToString(frameBytes)

    // Call service
    resp, err := http.Post(qc.BaseURL+"/quality/assess_video_frame",
        "application/json",
        bytes.NewBuffer([]byte(fmt.Sprintf(`{"frame_base64": "%s", "faces": %s}`, frameB64, facesJSON))))

    // Parse response
    var result struct {
        Faces []FaceQuality `json:"faces"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    return result.Faces, nil
}
```

**Option B: Compreface-Only (Simpler)**

Skip local quality assessment entirely:
- Use Compreface detection confidence directly
- Filter by face size only
- Simpler, but less control over quality

**Option C: go-face (Complex)**

Port dlib quality logic to Go:
- Requires CGO + dlib compilation
- Platform-specific builds
- Higher maintenance burden

**Recommendation:** Option A (Python Quality Service)

### 3.5 Advantages of Go Approach

1. **Native Integration:** Part of plugin binary, no external service
2. **Concurrent Processing:** Can process frames in parallel
3. **Progress Reporting:** Real-time log.Progress() updates
4. **Cancellation Support:** Check a.stopping flag in loops
5. **Resource Control:** Batching + cooldown built-in
6. **Reuse Compreface:** Leverage existing API for detection/recognition

### 3.6 Disadvantages of Go Approach

1. **Less Sophisticated:** No adaptive sampling (or need to implement)
2. **FFmpeg Dependency:** Requires ffmpeg binary on system
3. **Quality Assessment:** Need Python service OR accept lower quality filtering
4. **Development Time:** Need to implement sprite parsing, frame extraction, de-duplication
5. **Testing Burden:** More code to test and maintain

---

## 4. Comparison Matrix

| Aspect | Existing Python | stash-auto-vision | Independent Go |
|--------|----------------|-------------------|----------------|
| **Implementation Status** | ✅ Complete | ❌ Research only | ⚠️ Needs implementation |
| **Integration** | ❌ External script | ❌ External service | ✅ Native in plugin |
| **Face Detection** | dlib MMOD (local) | InsightFace (service) | Compreface API |
| **Detection Accuracy** | ~97% (dlib) | ~99.86% (InsightFace) | Compreface (varies) |
| **Quality Assessment** | ✅ Sophisticated | ✅ Rich metadata | ⚠️ Basic OR service call |
| **Sprite Support** | ✅ Yes | ⚠️ Unknown | ✅ Can implement |
| **Frame Extraction** | ✅ Adaptive sampling | ⚠️ Unknown | ✅ Can implement |
| **De-duplication** | ✅ Embedding-based | ✅ Embedding-based | ✅ Can implement |
| **GPU Acceleration** | ❌ CPU only (dlib) | ✅ GPU (InsightFace) | ⚠️ Via Compreface |
| **Performance** | Medium (5 FPS) | Fast (30 FPS) | Medium (depends) |
| **Resource Usage** | Heavy (500MB models) | Heavy (GPU + services) | Light (Go binary) |
| **Dependencies** | Heavy (dlib, OpenCV) | Heavy (Docker stack) | Light (ffmpeg binary) |
| **Progress Reporting** | ⚠️ Basic (log.progress) | ✅ Via RPC | ✅ Native RPC |
| **Cancellation Support** | ❌ No | ✅ Via RPC | ✅ Native RPC |
| **Cooldown Periods** | ❌ No | ✅ Configurable | ✅ Native support |
| **Development Effort** | ✅ Done | ❌ High (full stack) | ⚠️ Medium (just video) |
| **Maintenance Burden** | Medium (Python) | High (microservices) | Medium (Go) |
| **Scene Splitting** | ❌ No | ✅ PySceneDetect | ⚠️ Can add |
| **Broader Vision Tasks** | ❌ Face only | ✅ Full scene analysis | ❌ Face only |

---

## 5. Requirements for stash-compreface-plugin Phase 5

### Functional Requirements

1. **FR1: Sprite Sheet Processing**
   - Parse Stash-generated sprite sheets (VTT + JPEG)
   - Extract individual frames from grid
   - Support standard Stash sprite format

2. **FR2: Frame Extraction**
   - Extract frames from video at configurable intervals
   - Support adaptive sampling (focus on face-dense regions)
   - Handle various video formats (mp4, mkv, avi, etc.)

3. **FR3: Face Detection**
   - Detect faces in extracted frames
   - Use Compreface detection API
   - Filter by minimum face size

4. **FR4: Face De-duplication**
   - Track unique faces across all frames
   - Use embedding similarity threshold
   - Select best quality frame per unique face

5. **FR5: Quality Assessment**
   - Assess face quality (confidence, pose, size)
   - Filter low-quality detections
   - Support high-quality and low-quality modes

6. **FR6: Face Recognition**
   - Match detected faces to Compreface subjects
   - Create new subjects for unrecognized faces
   - Link subjects to Stash performers

7. **FR7: Scene Tagging**
   - Add "Compreface Scanned" tag to processed scenes
   - Add "Compreface Matched" tag when performers found
   - Add people count tags (1 person, 2 people, etc.)

8. **FR8: Progress & Cancellation**
   - Report progress via log.Progress()
   - Support task cancellation via Stop()
   - Apply cooldown between scenes

### Non-Functional Requirements

1. **NFR1: Performance**
   - Process scenes in reasonable time (< 30 seconds per scene)
   - Support batch processing with configurable batch sizes
   - Apply cooldown to prevent hardware stress

2. **NFR2: Resource Efficiency**
   - Minimize memory usage (cleanup temp files)
   - Limit concurrent operations
   - Respect system resources

3. **NFR3: Reliability**
   - Handle video codec errors gracefully
   - Continue processing on individual scene failures
   - Clean up resources on cancellation

4. **NFR4: Maintainability**
   - Clean, documented code
   - Testable components
   - Follow existing plugin patterns

5. **NFR5: User Experience**
   - Clear progress indicators
   - Informative logging
   - Sensible default settings

### Priority Rankings

**Must Have (Phase 5):**
- FR1: Sprite sheet processing (preferred method)
- FR3: Face detection via Compreface
- FR4: Face de-duplication
- FR6: Face recognition
- FR7: Scene tagging
- FR8: Progress & cancellation
- NFR1: Performance (with cooldown)

**Should Have (Phase 5):**
- FR2: Frame extraction (fallback method)
- FR5: Quality assessment (via Python service)
- NFR2: Resource efficiency

**Could Have (Future):**
- Adaptive sampling
- Scene splitting (shot detection)
- Parallel frame processing

**Won't Have (Out of Scope):**
- Broader scene analysis (CLIP, YOLO)
- GPU-accelerated detection
- Real-time processing

---

## 6. Recommended Architecture

### Decision: Hybrid Approach

**Primary:** Go implementation with Python Quality Service support

**Rationale:**
1. **Native Integration:** Embed in Go plugin for tight Stash integration
2. **Leverage Existing:** Reuse Python quality service (already built)
3. **Pragmatic:** Compreface API for detection, quality service for filtering
4. **Incremental:** Can upgrade to InsightFace service later if needed

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│             Stash Compreface RPC Plugin (Go)                     │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Scene Recognition (scenes.go)                           │  │
│  │  - Fetch unscanned scenes                                │  │
│  │  - Batch processing + cooldown                           │  │
│  │  - Progress reporting                                    │  │
│  └──────────────────┬───────────────────────────────────────┘  │
│                     │                                            │
│        ┌────────────┴─────────────┐                             │
│        ▼                           ▼                             │
│  ┌──────────┐              ┌──────────────┐                    │
│  │ Sprite   │              │ Frame        │                     │
│  │ Parser   │              │ Extractor    │                     │
│  │ (VTT)    │              │ (FFmpeg)     │                     │
│  └────┬─────┘              └──────┬───────┘                    │
│       │                           │                              │
│       └───────────┬───────────────┘                             │
│                   │                                              │
│                   ▼                                              │
│         ┌─────────────────────┐                                 │
│         │ Face Deduplicator   │                                 │
│         │ - Embedding tracking│                                 │
│         │ - Best frame select │                                 │
│         └──────────┬──────────┘                                 │
│                    │                                             │
│       ┌────────────┼────────────┐                               │
│       ▼            ▼             ▼                               │
│  ┌─────────┐ ┌─────────┐  ┌──────────┐                        │
│  │Compreface│ │ Quality │  │ Performer│                        │
│  │ Client   │ │ Service │  │ Matching │                        │
│  │(Detect)  │ │(Assess) │  │  Logic   │                        │
│  └─────────┘ └─────────┘  └──────────┘                        │
└─────────────────────────────────────────────────────────────────┘
         │           │              │
         ▼           ▼              ▼
   Compreface    Quality       GraphQL
   API (8000)   Service (8001)  (Stash)
```

### Component Breakdown

#### 1. Sprite Parser (Go)
**File:** `gorpc/video/sprite_parser.go`
**Responsibility:** Parse VTT + extract sprite frames
**Dependencies:** Standard library (image, regexp, http)

#### 2. Frame Extractor (Go)
**File:** `gorpc/video/frame_extractor.go`
**Responsibility:** Extract frames via ffmpeg
**Dependencies:** os/exec (call ffmpeg binary)

#### 3. Face Deduplicator (Go)
**File:** `gorpc/video/face_deduplicator.go`
**Responsibility:** Track unique faces, select best frames
**Dependencies:** math (cosine similarity)

#### 4. Quality Client (Go)
**File:** `gorpc/video/quality_client.go`
**Responsibility:** Call Python quality service for filtering
**Dependencies:** net/http

#### 5. Python Quality Service (Existing)
**Location:** `quality_service/app.py`
**Enhancement:** Add video frame assessment endpoint
**Responsibility:** dlib-based quality scoring

### Implementation Plan

**Phase 5.1: Core Video Processing (2 days)**
- Implement sprite_parser.go
- Implement frame_extractor.go
- Implement face_deduplicator.go
- Unit tests for each component

**Phase 5.2: Compreface Integration (1 day)**
- Integrate Compreface detection API
- Integrate Compreface recognition API
- Handle API errors gracefully

**Phase 5.3: Quality Service Integration (1 day)**
- Add video frame endpoint to quality_service
- Implement quality_client.go
- Integration tests

**Phase 5.4: Scene Recognition Logic (1 day)**
- Implement recognizeScenes() in scenes.go
- Add batch processing + cooldown
- Add progress reporting + cancellation
- Update scene tags/performers

**Phase 5.5: Testing & Refinement (1 day)**
- End-to-end testing with real videos
- Performance optimization
- Error handling improvements
- Documentation

**Total Estimated Duration:** 6 days

---

## 7. Alternative: stash-auto-vision Web Service

### When to Build stash-auto-vision

**Build IF:**
1. You want broader scene analysis (not just faces)
2. You want state-of-the-art accuracy (InsightFace 99.86%)
3. You have GPU resources available
4. Multiple plugins will benefit from vision services
5. You're willing to maintain microservices architecture

**Don't Build IF:**
1. Face recognition is the only use case
2. GPU resources are limited
3. Simplicity and maintainability are priorities
4. Current Compreface accuracy is sufficient

### Proposed Interface (if building service)

**Service:** Face Recognition Server
**Port:** 5200
**Base URL:** http://localhost:5200

**Endpoints:**

```python
# Detect faces in video frame
POST /video/detect_faces
{
    "video_path": "/media/scene.mp4",
    "timestamp": 30.5,
    "min_confidence": 0.8
}
Response: {
    "faces": [
        {"bbox": [x, y, w, h], "confidence": 0.99, "landmarks": [...], "embedding": [...]}
    ]
}

# Assess frame quality
POST /video/assess_quality
{
    "video_path": "/media/scene.mp4",
    "timestamp": 30.5,
    "faces": [{"bbox": [x, y, w, h]}]
}
Response: {
    "quality": [
        {"face_index": 0, "score": 0.87, "pose": "front", "blur": 0.12}
    ]
}

# Recognize faces (match to database)
POST /video/recognize_faces
{
    "video_path": "/media/scene.mp4",
    "timestamp": 30.5,
    "embeddings": [[...], [...]]
}
Response: {
    "matches": [
        {"embedding_index": 0, "subject": "Person 123 ABC", "confidence": 0.95}
    ]
}

# Process entire video (batch)
POST /video/process_video
{
    "video_path": "/media/scene.mp4",
    "mode": "sprites",  // or "adaptive_frames"
    "quality_mode": "high"
}
Response: {
    "job_id": "abc-123",
    "status": "processing"
}

GET /video/job_status/:job_id
Response: {
    "status": "completed",
    "progress": 1.0,
    "unique_faces": 3,
    "frames_processed": 45
}
```

**Go Client:**

```go
type VideoFaceService struct {
    BaseURL string
}

func (vfs *VideoFaceService) DetectFacesInFrame(videoPath string, timestamp float64) ([]Face, error) {
    payload := map[string]interface{}{
        "video_path": videoPath,
        "timestamp": timestamp,
    }

    resp, err := http.Post(vfs.BaseURL+"/video/detect_faces", "application/json", ...)

    var result struct {
        Faces []Face `json:"faces"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    return result.Faces, nil
}

func (vfs *VideoFaceService) ProcessVideo(videoPath, mode, qualityMode string) (string, error) {
    // Submit job
    payload := map[string]interface{}{
        "video_path": videoPath,
        "mode": mode,
        "quality_mode": qualityMode,
    }

    resp, err := http.Post(vfs.BaseURL+"/video/process_video", ...)

    var result struct {
        JobID string `json:"job_id"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    return result.JobID, nil
}

func (vfs *VideoFaceService) PollJobStatus(jobID string) (*VideoProcessingResult, error) {
    resp, err := http.Get(vfs.BaseURL+"/video/job_status/"+jobID)

    var result VideoProcessingResult
    json.NewDecoder(resp.Body).Decode(&result)

    return &result, nil
}
```

### Pros of Service Approach

1. **State-of-the-art:** InsightFace accuracy (99.86% vs ~97%)
2. **GPU Acceleration:** 6x faster (30 FPS vs 5 FPS)
3. **Reusable:** Multiple plugins can use same service
4. **Scalable:** Can run on dedicated GPU server
5. **Future-proof:** Easy to swap models/add features

### Cons of Service Approach

1. **Complexity:** Docker Compose, networking, coordination
2. **Deployment:** Users must run additional services
3. **Resource:** Requires GPU for optimal performance
4. **Latency:** HTTP overhead for each frame
5. **Maintenance:** More moving parts to maintain

---

## 8. Final Recommendation

### Recommendation: Hybrid Approach for Phase 5

**Implementation:**
1. **Go Core:** Implement sprite parsing, frame extraction, de-duplication in Go
2. **Compreface API:** Use for face detection and recognition
3. **Quality Service:** Extend existing Python quality_service for quality assessment
4. **Future Path:** If accuracy insufficient, migrate to InsightFace service

**Reasoning:**
- ✅ **Pragmatic:** Builds on existing quality service investment
- ✅ **Maintainable:** Keeps complexity in Go plugin
- ✅ **Incremental:** Can upgrade to full vision service later
- ✅ **Sufficient:** Current Compreface + quality service meets requirements
- ✅ **Lower Risk:** Smaller scope, faster delivery

### Deferred to Future

**stash-auto-vision Service:**
- Build as separate project when:
  - Broader scene analysis needed (tags, objects)
  - Multiple plugins require vision services
  - GPU resources available
  - Team has capacity for microservices maintenance

**For Now:**
- Focus on Phase 5: Video face recognition in current plugin
- Keep architecture flexible for future service integration

---

## Appendix A: Configuration Parameters

### Proposed Phase 5 Settings

```yaml
# compreface.yml
settings:
  # ... existing settings ...

  videoSpriteMode:
    displayName: Use Sprite Sheets for Video Processing
    description: Use Stash sprite sheets (faster) vs frame extraction (higher quality)
    type: BOOLEAN
    default: true

  videoFrameInterval:
    displayName: Video Frame Interval (seconds)
    description: Interval between extracted frames when not using sprites (default 5 seconds)
    type: NUMBER
    default: 5

  videoMaxFaces:
    displayName: Maximum Faces Per Video
    description: Stop processing video after finding this many unique faces (default 10)
    type: NUMBER
    default: 10

  useQualityService:
    displayName: Use Quality Service for Video
    description: Enable Python quality service for video face quality assessment
    type: BOOLEAN
    default: true

  qualityServiceURL:
    displayName: Quality Service URL
    description: URL of quality assessment service (default http://localhost:8001)
    type: STRING
    default: http://localhost:8001
```

### Equivalent Python Config (for reference)

```python
# From video_recognition.py and common.py
OPENCV_MIN_INTERVAL = 5      # Minimum seconds between frames
OPENCV_MAX_INTERVAL = 30     # Maximum seconds between frames
OPENCV_MAX_FACES = 10        # Stop after finding N unique faces
OPENCV_CONFIDENCE = 2.0      # High quality threshold
OPENCV_LOW_CONFIDENCE = 0.8  # Low quality threshold
MIN_FACE_SIZE = 64           # Minimum face dimension (pixels)
```

---

## Appendix B: Testing Strategy

### Unit Tests

```go
// video/sprite_parser_test.go
func TestParseSpriteSheet(t *testing.T) {
    // Test with mock sprite + VTT
    frames, err := ParseSpriteSheet(mockSpriteURL, mockVttURL)
    assert.NoError(t, err)
    assert.Equal(t, 100, len(frames))  // Typical sprite count
}

// video/face_deduplicator_test.go
func TestFaceDeduplication(t *testing.T) {
    tracker := NewFaceTracker()

    // Add first face
    assert.True(t, tracker.IsUniqueFace(embedding1, 0.6))
    tracker.AddFace(embedding1, "frame1.jpg", 2.5)

    // Add similar face (should deduplicate)
    assert.False(t, tracker.IsUniqueFace(embedding1Similar, 0.6))

    // Add different face
    assert.True(t, tracker.IsUniqueFace(embedding2, 0.6))
}

// video/quality_client_test.go
func TestQualityAssessment(t *testing.T) {
    client := NewQualityClient("http://localhost:8001")

    quality, err := client.AssessFrame("test_frame.jpg", []FaceDetection{...})
    assert.NoError(t, err)
    assert.Greater(t, quality[0].Score, 0.0)
    assert.Equal(t, "front", quality[0].Pose)
}
```

### Integration Tests

```go
// scenes_test.go
func TestSceneRecognitionWithSprites(t *testing.T) {
    api := &ComprefaceAPI{...}

    // Process test scene with known faces
    err := api.recognizeScenes(true)
    assert.NoError(t, err)

    // Verify scene tagged
    scene, _ := api.getScene(testSceneID)
    assert.Contains(t, scene.Tags, "Compreface Scanned")
}

func TestSceneRecognitionWithFrames(t *testing.T) {
    api := &ComprefaceAPI{...}

    // Process with frame extraction
    err := api.recognizeScenes(false)
    assert.NoError(t, err)
}
```

### Performance Benchmarks

```go
func BenchmarkSpriteProcessing(b *testing.B) {
    for i := 0; i < b.N; i++ {
        ParseSpriteSheet(benchSpriteURL, benchVttURL)
    }
}

func BenchmarkFrameExtraction(b *testing.B) {
    for i := 0; i < b.N; i++ {
        ExtractFrame(benchVideoPath, 30.0, "output.jpg")
    }
}
```

---

**End of Document**

**Status:** Ready for architecture approval
**Next Steps:**
1. Review and approve architecture decision
2. Begin Phase 5.1 implementation (sprite parser + frame extractor)
3. Extend quality service with video endpoints
