# Quality Service Migration Guide

**Date:** 2025-11-09
**Changes:** Service rename, port change, environment variable configuration

---

## Summary of Changes

The quality service has been renamed and reconfigured for better clarity and integration with the Stash ecosystem:

### Service Identity
- **Old Name:** `quality-service` (generic)
- **New Name:** `stash-face-quality` (descriptive)
- **Container Name:** `stash-face-quality`
- **Image Name:** `stash-face-quality:latest`

### Network & Port
- **Old Port:** 8001
- **New Port:** 6001 (default, configurable)
- **Docker Network:** `stash-compreface` (default, configurable)

### Directory Structure
- **Old Directory:** `quality_service/`
- **New Directory:** `face-quality-service/`

---

## Migration Steps

### For Docker Compose Users

1. **Stop old service:**
   ```bash
   docker-compose down
   docker rmi stash-quality-service:latest  # Remove old image
   ```

2. **Copy environment configuration:**
   ```bash
   cd face-quality-service
   cp .env.example .env
   # Edit .env if you need custom settings
   ```

3. **Start new service:**
   ```bash
   docker-compose up -d
   ```

4. **Verify service is running:**
   ```bash
   curl http://localhost:6001/health
   # Should return: {"status": "ok", ...}
   ```

### For Manual Docker Users

1. **Stop and remove old container:**
   ```bash
   docker stop quality-service
   docker rm quality-service
   docker rmi stash-quality-service:latest
   ```

2. **Build new image:**
   ```bash
   cd face-quality-service
   docker build -t stash-face-quality:latest .
   ```

3. **Run new container:**
   ```bash
   docker run -d \
     -p 6001:6001 \
     -v /path/to/models:/app/models \
     --name stash-face-quality \
     --network stash-compreface \
     -e QUALITY_SERVICE_PORT=6001 \
     stash-face-quality:latest
   ```

### For Python Virtual Environment Users

1. **No migration needed** - The service still runs the same way:
   ```bash
   cd face-quality-service  # Changed directory name
   python app.py
   # Now defaults to port 6001 instead of 8001
   ```

2. **To use custom port:**
   ```bash
   export QUALITY_SERVICE_PORT=8001  # Or any port you prefer
   python app.py
   ```

---

## Plugin Configuration

### In Stash Plugin Settings

The plugin now has a **Quality Service URL** setting:

1. Navigate to: **Settings → Plugins → Compreface**
2. Find setting: **Quality Service URL**
3. Configuration options:
   - **Leave empty:** Service disabled (default if not using)
   - **Docker (auto-detect):** `http://stash-face-quality:6001` (default)
   - **Localhost:** `http://localhost:6001`
   - **Remote server:** `http://your-server:6001`

### DNS Resolution

The plugin automatically resolves service URLs:
- **Container names** → DNS lookup in Docker networks
- **IP addresses** → Used as-is
- **localhost** → Used as-is
- **Hostnames** → DNS resolution attempted

---

## Breaking Changes

### Port Change
- **Old:** Service listened on port `8001`
- **New:** Service listens on port `6001` by default
- **Action Required:** Update any hardcoded references to `localhost:8001`

### Container Name Change
- **Old:** `quality-service`
- **New:** `stash-face-quality`
- **Action Required:** Update any Docker networking configs that reference the old name

### Directory Name Change
- **Old:** `quality_service/`
- **New:** `face-quality-service/`
- **Action Required:** Update any file paths or volume mounts

---

## Configuration Options

### Environment Variables

| Variable | Description | Default | Location |
|----------|-------------|---------|----------|
| `QUALITY_SERVICE_PORT` | Internal service port | `6001` | Dockerfile, app.py |
| `WEB_SERVICE_PORT` | External port mapping | `6001` | docker-compose.yml |
| `DOCKER_NETWORK` | Docker network name | `stash-compreface` | docker-compose.yml |
| `QUALITY_SERVICE_TEMP` | Temp directory | `/tmp/quality_service` | Dockerfile, app.py |

### Using .env File (docker-compose only)

```bash
# face-quality-service/.env
DOCKER_NETWORK=stash-compreface
WEB_SERVICE_PORT=6001
INTERNAL_PORT=6001
QUALITY_SERVICE_TEMP=/tmp/quality_service
```

---

## Verification

### Test Service Health
```bash
curl http://localhost:6001/health
```

Expected response:
```json
{
  "status": "ok",
  "service": "face-quality-service",
  "version": "1.0.0"
}
```

### Test from Plugin

The plugin logs will show:
```
Quality Service configured at: http://stash-face-quality:6001
```

Or if not configured:
```
Quality Service not configured (enhanced quality assessment disabled)
```

---

## Rollback

If you need to revert to the old configuration:

1. **Revert port in .env:**
   ```bash
   WEB_SERVICE_PORT=8001
   INTERNAL_PORT=8001
   ```

2. **Update Dockerfile:**
   ```dockerfile
   EXPOSE 8001
   ENV QUALITY_SERVICE_PORT=8001
   ```

3. **Update app.py:**
   ```python
   port = int(os.environ.get('QUALITY_SERVICE_PORT', 8001))
   ```

4. **Rebuild and restart:**
   ```bash
   docker-compose up -d --build
   ```

---

## Support

For issues or questions:
- Check logs: `docker logs stash-face-quality`
- Review configuration: `docker exec stash-face-quality env | grep QUALITY`
- Plugin logs: Check Stash plugin logs for service connection issues

---

**Note:** This migration guide is for reference only. The changes have been applied to all code and documentation files in this repository.
