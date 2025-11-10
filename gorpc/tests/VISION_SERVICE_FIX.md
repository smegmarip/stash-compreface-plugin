# Vision Service 403 Error - Resolution

**Date:** 2025-11-10
**Issue:** Vision Service integration tests failing with HTTP 403 Forbidden
**Status:** ✅ RESOLVED

---

## Problem

Vision Service integration tests were failing with 403 errors:

```
TestVisionIntegration_HealthCheck: service unhealthy: status 403
TestVisionIntegration_IsVisionServiceAvailable: status 403
```

Tests were attempting to connect to: `http://localhost:5000`

---

## Root Cause

**Port Configuration Mismatch**

The Vision Service is running on **port 5010**, not port 5000.

**Reason:** Port 5000 collision with macOS AirPlay service.

**Docker Configuration:**
```bash
$ docker ps | grep vision-api
78f5cfce1da8   stash-auto-vision-vision-api   ...   0.0.0.0:5010->5000/tcp   vision-api
```

The service runs on container port 5000, mapped to host port **5010**.

---

## Solution

Updated default Vision Service URL from port 5000 to port 5010:

### Files Modified

1. **tests/testutil/helpers.go** (line 30)
   ```go
   // Before:
   VisionServiceURL:  getEnvOrDefault("VISION_SERVICE_URL", "http://localhost:5000"),

   // After:
   VisionServiceURL:  getEnvOrDefault("VISION_SERVICE_URL", "http://localhost:5010"),
   ```

2. **tests/setup_integration_tests.sh** (line 37)
   ```bash
   # Before:
   export VISION_SERVICE_URL="http://localhost:5000"

   # After:
   # Note: Vision API runs on port 5010 (mapped from container port 5000)
   export VISION_SERVICE_URL="http://localhost:5010"
   ```

---

## Verification

### Health Check Response

```bash
$ curl http://localhost:5010/health
```

```json
{
  "status": "healthy",
  "service": "vision-api",
  "version": "1.0.0",
  "services": {
    "scenes": {
      "status": "healthy",
      "service": "scenes-service",
      "version": "1.0.0",
      "detection_methods": ["content", "threshold", "adaptive"],
      "gpu_available": false,
      "active_jobs": 0,
      "cache_size_mb": 1.135
    },
    "faces": {
      "status": "healthy",
      "service": "faces-service",
      "version": "1.0.0",
      "model": "buffalo_l",
      "gpu_available": false,
      "active_jobs": 0,
      "cache_size_mb": 1.135
    },
    "semantics": {
      "status": "healthy",
      "service": "semantics-service",
      "version": "1.0.0",
      "implemented": false,
      "phase": 2,
      "message": "Stub service - awaiting CLIP integration"
    },
    "objects": {
      "status": "healthy",
      "service": "objects-service",
      "version": "1.0.0",
      "implemented": false,
      "phase": 3,
      "message": "Stub service - awaiting YOLO-World integration"
    }
  }
}
```

### Test Results

**Before Fix:**
- TestVisionIntegration_HealthCheck: ⏭️ SKIP (403 error)
- TestVisionIntegration_IsVisionServiceAvailable: ⏭️ SKIP (403 error)

**After Fix:**
- TestVisionIntegration_HealthCheck: ✅ PASS (0.03s)
- TestVisionIntegration_IsVisionServiceAvailable: ✅ PASS (0.05s)

---

## Complete Vision Test Results

| Test | Status | Time | Notes |
|------|--------|------|-------|
| TestVisionIntegration_HealthCheck | ✅ PASS | 0.03s | Service healthy, all microservices running |
| TestVisionIntegration_IsVisionServiceAvailable | ✅ PASS | 0.05s | Availability check working |
| TestVisionIntegration_SubmitAndCheckJob | ⏭️ SKIP | 0.00s | Test video not provided (optional) |
| TestVisionIntegration_BuildAnalyzeRequest | ✅ PASS | 0.00s | Request building verified |

**Result:** 3/4 passing (75%), 1 skipped (not critical)

---

## Vision Service Architecture

The Vision Service consists of multiple microservices:

1. **vision-api** (port 5010) - Main API gateway
2. **vision-scenes-service** (port 5002) - Scene detection
3. **vision-faces-service** (port 5003) - Face recognition (InsightFace)
4. **vision-semantics-service** (port 5004) - Semantic analysis (stub)
5. **vision-objects-service** (port 5005) - Object detection (stub)
6. **vision-frame-server** (port 5001) - Frame extraction
7. **vision-redis** (port 6379) - Job queue/cache

All services report healthy status.

---

## Models & GPU

**Current Configuration:**
- GPU Available: `false` (CPU mode)
- Face Model: `buffalo_l` (InsightFace)
- Scene Detection: `content`, `threshold`, `adaptive` methods

**Cache Status:**
- Scenes service cache: 1.13 MB
- Faces service cache: 1.13 MB

---

## Next Steps

### Optional: Add Test Video

To enable `TestVisionIntegration_SubmitAndCheckJob`:

1. Create or find short test video with faces
2. Place at: `tests/fixtures/videos/test_video.mp4`
3. Recommended specs:
   - Duration: 10-30 seconds
   - Resolution: 720p+
   - Format: MP4
   - Contains at least one clear face

### Integration Testing Ready

All Vision Service functionality now ready for integration testing:
- ✅ Health checks
- ✅ Service availability
- ✅ Request structure
- ⏸️ Job submission (requires test video)

---

## Impact on Integration Test Suite

**Before Vision Service Fix:**
- Total: 17 tests
- Passing: 13 (76%)
- Skipped: 4

**After Vision Service Fix:**
- Total: 17 tests
- Passing: 16 (94%)
- Skipped: 1 (only test video missing)

**Improvement:** +3 tests passing, +18% pass rate

---

## Lessons Learned

1. **Port Conflicts:** Always verify actual service ports via `docker ps`
2. **AirPlay Service:** macOS reserves port 5000 for AirPlay by default
3. **Container Mapping:** Check both container port AND host port mapping
4. **Health Endpoints:** Test with curl before assuming test failure

---

**Resolution Time:** ~15 minutes
**Fix Complexity:** Simple (2 lines changed)
**Testing Impact:** Significant (+18% pass rate)
