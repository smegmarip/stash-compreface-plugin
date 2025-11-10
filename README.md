# Stash Compreface Plugin

Face recognition and performer synchronization for Stash using Compreface.

**Version:** 2.0.0
**Interface:** RPC (Go)
**Repository:** https://github.com/smegmarip/stash-compreface-plugin

---

## Features

### Core Functionality
- **Performer Synchronization** - Sync existing Stash performers with Compreface subjects
- **Image Recognition** - Detect and group faces in images (HQ/LQ modes)
- **Image Identification** - Match faces to existing performers
- **Gallery Processing** - Batch process all images in a gallery
- **Single Image Operations** - Identify or create performers from individual images
- **Reset Operations** - Remove scan tags from unmatched images

### Video Recognition (Requires Vision Service)
- **Scene Recognition** - Extract and recognize faces from video scenes
- **Sprite Recognition** - Process scene sprite sheets
- **Note:** Video features require the separate [stash-auto-vision](../stash-auto-vision) service

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
  - Default: `0.89` (0.0-1.0 scale)
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
- **Vision Service URL** - URL of stash-auto-vision for video face recognition
  - Default: `http://stash-auto-vision:5000` (auto-detected)
  - Leave empty to disable video recognition
  - See [stash-auto-vision](../stash-auto-vision) for setup

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

See [CLAUDE.md](CLAUDE.md) for comprehensive usage documentation and task workflows.

---

## Development

See [CLAUDE.md](CLAUDE.md) for development documentation, architecture details, and implementation notes.

---

## License

This project is licensed under the GPL License - see the LICENSE.md file for details

---

## Credits

- **Compreface** - https://github.com/exadel-inc/CompreFace
- **Stash** - https://github.com/stashapp/stash
- **InsightFace** (Vision Service) - https://github.com/deepinsight/insightface
