# Stash Face Quality Service

This service provides face quality assessment, preprocessing, and enhanced detection capabilities for the Stash Compreface plugin using dlib and OpenCV.

## Features

- **Quality Assessment**: Analyzes face pose, confidence scores, and quality metrics
- **Face Alignment**: Aligns faces using 68-point facial landmarks
- **Image Preprocessing**: Histogram equalization and enhancement
- **Multi-Orientation Detection**: Detects faces in various orientations (front, left, right, rotated)

## Requirements

- Python 3.11+
- dlib model files:
  - `shape_predictor_68_face_landmarks.dat`
  - `dlib_face_recognition_resnet_model_v1.dat`

## Installation

### Local Development

```bash
cd face-quality-service
pip install -r requirements.txt

# Ensure dlib model files are present (in face-quality-service/ or parent dir)
ls -la *.dat

# Run the service
python app.py
```

The service will start on port 6001 by default (configurable via `QUALITY_SERVICE_PORT` environment variable).

### Docker

```bash
# Using docker-compose (recommended)
docker-compose up -d

# Or build and run manually
docker build -t stash-face-quality:latest .

# Run the container
docker run -d \
  -p 6001:6001 \
  -v /path/to/models:/app/models \
  --name stash-face-quality \
  --network stash-compreface \
  stash-face-quality:latest
```

**Using .env file:**
```bash
# Copy the example environment file
cp .env.example .env

# Edit .env to customize settings (port, network, etc.)
nano .env

# Start with docker-compose
docker-compose up -d
```

## API Endpoints

### Health Check

```bash
GET /health
```

Returns service status.

### Assess Quality

```bash
POST /quality/assess
Content-Type: application/json

{
  "image": "base64_encoded_image",
  "faces": [
    {
      "box": {"x_min": 100, "y_min": 100, "x_max": 200, "y_max": 200}
    }
  ]
}
```

Returns quality metrics for each face:

```json
{
  "faces": [
    {
      "box": {...},
      "confidence": {
        "score": 1.23,
        "type": "front",
        "type_raw": 0
      },
      "cropped_size": [100, 100]
    }
  ]
}
```

**Confidence Types:**
- `front`: Front-facing face (optimal)
- `left`: Left profile
- `right`: Right profile
- `front-rotate-left`: Front face rotated left
- `front-rotate-right`: Front face rotated right

**Score Interpretation:**
- `>= 1.0`: High confidence (optimal quality)
- `0.5 - 1.0`: Medium confidence
- `< 0.5`: Low confidence

### Preprocess Image

```bash
POST /quality/preprocess
Content-Type: multipart/form-data

file: <image_file>
```

Returns enhanced image with histogram equalization applied.

### Enhanced Detection

```bash
POST /quality/detect
Content-Type: multipart/form-data

file: <image_file>
```

Returns detected faces with quality metrics:

```json
{
  "faces": [
    {
      "box": {...},
      "confidence": {...},
      "cropped_size": [...]
    }
  ]
}
```

## Configuration

Environment variables:

- `QUALITY_SERVICE_PORT`: Port to listen on (default: 6001)
- `QUALITY_SERVICE_TEMP`: Temporary directory for file storage (default: /tmp/quality_service)
- `WEB_SERVICE_PORT`: External port mapping (default: 6001, docker-compose only)
- `DOCKER_NETWORK`: Docker network name (default: stash-compreface, docker-compose only)

## Testing

```bash
# Test health check
curl http://localhost:6001/health

# Test quality assessment (requires test image)
curl -X POST http://localhost:6001/quality/assess \
  -H "Content-Type: multipart/form-data" \
  -F "file=@test_image.jpg" \
  -F 'faces=[{"box":{"x_min":100,"y_min":100,"x_max":200,"y_max":200}}]'
```

## Integration with Stash Compreface Plugin

The Go RPC plugin will automatically discover and call this service via HTTP:

**Plugin Configuration:**
In your Stash plugin settings, configure the Quality Service URL:
- Setting name: `qualityServiceUrl`
- Default: `http://stash-face-quality:6001` (auto-detected in Docker networks)
- Leave empty to disable enhanced quality assessment

**Go Client Usage:**
```go
// The plugin automatically resolves the service URL with DNS lookup
qualityClient := NewQualityClient(config.QualityServiceURL)
assessment, err := qualityClient.AssessFaces(imageBytes, faces)
```

**Service Discovery:**
- Docker container name: `stash-face-quality`
- Docker network: `stash-compreface`
- The plugin will resolve the container name via DNS automatically
- For non-Docker deployments, specify full URL in plugin settings

## Performance Notes

- Face alignment with dlib landmarks is computationally intensive
- Falls back to simple cropping if alignment fails
- Multi-orientation detection (1, -1) checks multiple face angles
- Recommend GPU acceleration for production workloads

## License

Same as parent Compreface plugin project.
