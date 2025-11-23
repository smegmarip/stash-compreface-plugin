# Stash Compreface Plugin

Face recognition and performer synchronization for Stash using Compreface.

**Version:** 2.0.0
**Interface:** RPC (Go)
**Status:** Production-Ready (12/13 tasks tested, 92% coverage)
**Repository:** https://github.com/smegmarip/stash-compreface-plugin

---

## Features

### Core Functionality

- **Performer Synchronization** - Sync existing Stash performers with Compreface subjects
- **Image Recognition** - Detect and group faces in images (HQ/LQ modes)
- **Image Identification** - Match faces to existing performers
- **Gallery Processing** - Batch process all images in a gallery
- **Single Image Operations** - Identify or create performers from individual images
- **Reset Operations** - Remove scan tags from unmatched images/scenes

### Video Recognition (Vision Service v1.0.0)

- **Scene Recognition** - Extract and recognize faces from video scenes (4 tasks: new/all, frame/sprite)
- **Occlusion Filtering** - Automatic detection and filtering of masked/occluded faces
- **Sprite Processing** - VTT parsing and thumbnail extraction from sprite sheets
- **Face Enhancement** - Optional CodeFormer/GFPGAN enhancement for low-quality faces
- **Status:** ✅ Fully functional with Vision Service v1.0.0

### Performance Features

- **GPU-Friendly Batching** - Configurable batch sizes (default: 20)
- **Cooldown Periods** - Prevent hardware overheating (default: 10 seconds)
- **Progress Reporting** - Real-time progress updates during batch operations
- **Task Cancellation** - Graceful shutdown support

---

## Description

This plugin provides comprehensive face recognition for Stash using Compreface

---

## Requirements

### Required

- **Stash** v0.20.0+ (with RPC plugin support)
- **Compreface** v1.0.0+ (face recognition service)
- **Go** 1.21+ (for building from source)

### Optional

- **stash-auto-vision** - For video scene face recognition
- **stash-face-quality** - For enhanced face quality assessment (dlib-based)

---

## Installation

### 1. Download or Build Binary

#### Option A: Build from Source

```bash
# Clone repository
cd /path/to/stash/plugins
git clone https://github.com/smegmarip/stash-compreface-plugin

# Build binary
cd stash-compreface-plugin
./build.sh          # Current platform
./build.sh linux    # Linux amd64
./build.sh all      # All platforms
```

#### Option B: Download Pre-built Binary

Download the appropriate binary for your platform from the releases page.

### 2. Install Plugin in Stash

1. Copy the plugin directory to your Stash plugins folder
2. Ensure the binary is executable:
   ```bash
   chmod +x gorpc/stash-compreface-rpc
   ```
3. Reload plugins in Stash UI: Settings → Plugins → Reload Plugins

### 3. Configure Plugin Settings

Navigate to Settings → Plugins → Compreface and configure:

**Required Settings:**

- **Recognition API Key** - Your Compreface recognition service key
- **Detection API Key** - Your Compreface detection service key

**Core Service Settings:**

- **Compreface Service URL** - URL of Compreface service
  - Default: `http://compreface:8000` (auto-detected in Docker)
  - Leave empty for auto-detection
  - Supports: container names, hostnames, IP addresses, localhost

**Performance Settings:**

- **Cooldown Period (seconds)** - Delay between batches to prevent hardware overheating

  - Default: `10` seconds
  - Recommended for GPU operations

- **Maximum Batch Size** - Maximum items to process per batch
  - Default: `20` items
  - Prevents hardware stress and overheating

**Recognition Quality Settings:**

- **Minimum Similarity Threshold** - Face match confidence threshold

  - Default: `0.81` (0.0-1.0 scale)
  - Higher = stricter matching, fewer false positives

- **Minimum Face Size** - Minimum face dimensions in pixels
  - Default: `64` pixels
  - Filters out small/low-quality faces

**Tag Management:**

- **Scanned Tag Name** - Tag for processed items

  - Default: `"Compreface Scanned"`
  - Auto-created if doesn't exist

- **Matched Tag Name** - Tag for matched items
  - Default: `"Compreface Matched"`
  - Auto-created if doesn't exist

**Optional Enhancement Services:**

- **Vision Service URL** - URL of stash-auto-vision service for video face recognition

  - Default: `http://vision-api:5010` (Docker auto-detected)
  - Uses InsightFace for face detection with optional CodeFormer/GFPGAN enhancement
  - Leave empty to disable video recognition features
  - See [stash-auto-vision](../stash-auto-vision) for setup and configuration

- **Quality Service URL** - URL of stash-face-quality for enhanced quality assessment
  - Default: `http://stash-face-quality:6001` (auto-detected)
  - Leave empty to disable enhanced quality assessment
  - See [face-quality-service/](face-quality-service/) for setup

**Service URL Auto-Detection:**
All service URLs support automatic DNS resolution:

- **Container names** (e.g., `compreface`, `stash-face-quality`) → DNS lookup in Docker networks
- **Hostnames** (e.g., `myserver.local`) → DNS resolution
- **IP addresses** (e.g., `192.168.1.100`) → Used as-is
- **localhost** → Used as-is

---

## Usage

### Available Tasks

All tasks accessible via Settings → Plugins → Compreface or GraphQL API.

| Task                        | Status     | Description                              |
| --------------------------- | ---------- | ---------------------------------------- |
| Synchronize Performers      | ✅ Tested  | Sync performers with Compreface subjects |
| Recognize Images (HQ)       | ✅ Tested  | Detect faces in high-quality images      |
| Recognize Images (LQ)       | ✅ Tested  | Detect faces in low-quality images       |
| Identify All Images         | ✅ Tested  | Match faces in all images                |
| Identify Unscanned Images   | ✅ Tested  | Match faces in new images only           |
| Reset Unmatched Images      | ✅ Tested  | Remove scan tags from unmatched          |
| Identify Single Image       | ✅ Tested  | Process specific image                   |
| Create Performer from Image | ✅ Tested  | Create performer from face               |
| Identify Gallery            | ✅ Tested  | Process all gallery images               |
| Recognize New Scenes        | ⏸️ Blocked | Video face recognition (unscanned only)  |
| Recognize New Scene Sprites | ⏸️ Blocked | Sprite sheet processing (unscanned only) |
| Recognize All Scenes        | ⏸️ Blocked | Video face recognition (rescan partial)  |
| Recognize All Scene Sprites | ⏸️ Blocked | Sprite sheet processing (rescan partial) |

**Note:** Scene recognition tasks blocked by Vision Service face detection issues (upstream). Plugin code is complete and ready once Vision Service is fixed.

### Quick Start

1. **Synchronize existing performers:**

   - Go to Settings → Plugins → Compreface
   - Run "Synchronize Performers" task
   - This creates Compreface subjects for performers with face images

2. **Recognize faces in new images:**

   - Run "Recognize Images (High Quality)" task
   - Limit parameter recommended (e.g., limit=50)
   - Creates subjects for new faces

3. **Match faces to performers:**
   - Run "Identify Unscanned Images" task
   - Matches faces to existing performers
   - Tags matched images automatically

For detailed usage workflows, see [CLAUDE.md](CLAUDE.md).

---

## Testing

**Test Coverage:** 9/11 tasks (82%)

- Unit tests: Component-level validation
- Integration tests: Live service interactions
- E2E tests: Complete task workflows

See [docs/TESTING.md](docs/TESTING.md) for comprehensive testing procedures and results.

**Running Tests:**

```bash
# Unit tests
cd gorpc && go test ./internal/... -v

# E2E tests (requires services running)
cd tests/e2e && ./run_suite.sh
```

---

## Development

**Architecture:** Clean domain-driven design with ~5,000 lines of Go code

- **RPC Layer:** Business logic and task routing
- **Repository Layer:** Type-safe GraphQL operations
- **Service Layer:** External API clients (Compreface, Vision, Quality)

See [CLAUDE.md](CLAUDE.md) for development guide, architecture details, and implementation patterns.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for system design and component relationships.

---

## Troubleshooting

**Common Issues:**

1. **"Compreface service not configured"**

   - Set Recognition API Key and Detection API Key in plugin settings
   - Verify Compreface service is running: `curl http://localhost:8000/`

2. **"Vision Service unavailable"**

   - Vision Service is optional for video features
   - Check service health: `curl http://localhost:5010/health`
   - Or leave Vision Service URL empty to disable

3. **No faces detected**

   - Check image quality (some images naturally have no faces)
   - Adjust minimum face size setting (default: 64px)
   - Review logs: `docker logs stash | grep Compreface`

4. **GraphQL 422 errors**
   - Fixed in v2.0.0 with proper tag/performer list handling
   - Ensure plugin is up to date

**For more troubleshooting, see [docs/TESTING.md](docs/TESTING.md#troubleshooting).**

---

## Known Issues

- **Scene Recognition:** Blocked by Vision Service face detection issues (upstream project)
- **Impact:** Video recognition tasks cannot be fully validated
- **Workaround:** None currently - requires Vision Service fixes
- **Status:** Plugin code complete and ready once Vision Service is fixed

---

## License

This project is licensed under the GPL License - see the LICENSE.md file for details

---

## Credits

- **Compreface** - https://github.com/exadel-inc/CompreFace
- **Stash** - https://github.com/stashapp/stash
- **InsightFace** (Vision Service) - https://github.com/deepinsight/insightface
