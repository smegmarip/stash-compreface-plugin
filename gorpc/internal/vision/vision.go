package vision

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// ============================================================================
// Vision Service Client - Stash Auto Vision Integration
// ============================================================================
//
// This client communicates with the standalone stash-auto-vision service
// for high-accuracy video face recognition using InsightFace.
//
// Service Architecture:
// - FastAPI web service (port 5010)
// - Celery + Redis for async job processing
// - InsightFace RetinaFace detection + ArcFace embeddings (512-D)
// - FFmpeg frame extraction with adaptive sampling
// - De-duplication via cosine similarity
//
// API Flow:
// 1. Submit job with video path → receive job_id
// 2. Poll job status until completed/failed
// 3. Retrieve results with face detections and embeddings
//
// ============================================================================

// ============================================================================
// API Methods
// ============================================================================

// NewVisionServiceClient creates a new client
func NewVisionServiceClient(baseURL string) *VisionServiceClient {
	return &VisionServiceClient{
		BaseURL:        baseURL,
		FrameServerURL: "http://vision-frame-server:5001", // Internal container address
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// SubmitJob submits a face recognition job to the Vision Service
func (c *VisionServiceClient) SubmitJob(req AnalyzeRequest) (*JobResponse, error) {
	url := fmt.Sprintf("%s/vision/analyze", c.BaseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Debugf("Submitting Vision Service job to %s: scene_id=%s, source=%s", url, req.SceneID, req.Source)

	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to submit job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var jobResp JobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jobResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Infof("Vision Service job submitted: job_id=%s", jobResp.JobID)
	return &jobResp, nil
}

// GetJobStatus polls job status and progress
func (c *VisionServiceClient) GetJobStatus(jobID string) (*JobStatus, error) {
	url := fmt.Sprintf("%s/vision/jobs/%s/status", c.BaseURL, jobID)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var status JobStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode status: %w", err)
	}

	return &status, nil
}

// GetResults retrieves job results (only available when status=completed)
func (c *VisionServiceClient) GetResults(jobID string) (*AnalyzeResults, error) {
	url := fmt.Sprintf("%s/vision/jobs/%s/results", c.BaseURL, jobID)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get results: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil, fmt.Errorf("job not completed yet")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var results AnalyzeResults
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode results: %w", err)
	}

	return &results, nil
}

// WaitForCompletion polls until job completes or fails
//
// This method implements the job polling pattern with:
// - 2-second polling interval
// - 1-hour timeout
// - Progress callback for UI updates
// - Detailed status logging
func (c *VisionServiceClient) WaitForCompletion(jobID string, progressCallback func(float64)) (*AnalyzeResults, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(1 * time.Hour)

	log.Infof("Waiting for Vision Service job %s to complete", jobID)

	for {
		select {
		case <-ticker.C:
			status, err := c.GetJobStatus(jobID)
			if err != nil {
				return nil, err
			}

			// Update progress
			if progressCallback != nil {
				progressCallback(status.Progress)
			}

			// Log detailed status
			if status.Stage != "" {
				log.Debugf("Job %s: status=%s, stage=%s, progress=%.1f%%, message=%s",
					jobID, status.Status, status.Stage, status.Progress*100, status.Message)
			} else {
				log.Debugf("Job %s: status=%s, progress=%.1f%%",
					jobID, status.Status, status.Progress*100)
			}

			// Check terminal status
			switch status.Status {
			case "completed":
				log.Infof("Vision Service job %s completed successfully", jobID)
				if status.Summary != nil {
					log.Infof("Summary: %+v", status.Summary)
				}
				return c.GetResults(jobID)

			case "failed":
				return nil, fmt.Errorf("job failed: %s", status.Error)
			}

		case <-timeout:
			return nil, fmt.Errorf("job timeout after 1 hour")
		}
	}
}

// HealthCheck checks if Vision Service is available and healthy
func (c *VisionServiceClient) HealthCheck() error {
	url := fmt.Sprintf("%s/health", c.BaseURL)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("service unhealthy: status %d", resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return fmt.Errorf("failed to decode health response: %w", err)
	}

	log.Debugf("Vision Service health: %+v", health)
	return nil
}

// ============================================================================
// Helper Methods
// ============================================================================

// BuildAnalyzeRequest creates a standard request for face recognition
func BuildAnalyzeRequest(videoPath, sceneID string, facesParameters FacesParameters) AnalyzeRequest {
	return AnalyzeRequest{
		Source:         videoPath, // Renamed from VideoPath (breaking change v1.0.0)
		SceneID:        sceneID,
		ProcessingMode: "sequential", // Sequential processing to avoid GPU memory contention
		Modules: Modules{
			Faces: FacesModule{
				Enabled:    true,
				Parameters: facesParameters,
			},
		},
	}
}

// IsVisionServiceAvailable checks if Vision Service is configured and reachable
func IsVisionServiceAvailable(baseURL string) bool {
	if baseURL == "" {
		return false
	}

	client := NewVisionServiceClient(baseURL)
	err := client.HealthCheck()
	if err != nil {
		log.Warnf("Vision Service not available at %s: %v", baseURL, err)
		return false
	}

	log.Infof("Vision Service available at %s", baseURL)
	return true
}

// ExtractFrame extracts a single frame from video at given timestamp
// Uses the frame-server's /extract-frame endpoint (separate service on different port)
func (c *VisionServiceClient) ExtractFrame(videoPath string, timestamp float64, enhancement *EnhancementParameters) ([]byte, error) {
	useEnhanced := false
	if enhancement != nil && enhancement.Enabled {
		useEnhanced = true
	}
	baseUrl := fmt.Sprintf("%s/extract-frame", c.FrameServerURL)
	params := url.Values{}
	params.Add("video_path", videoPath)
	params.Add("timestamp", fmt.Sprintf("%.2f", timestamp))
	params.Add("output_format", "jpeg")
	params.Add("quality", "95")
	frameType := ""
	if useEnhanced {
		// Use enhanced frame extraction
		params.Add("enhance", "1")
		params.Add("model", enhancement.Model)
		params.Add("fidelity_weight", fmt.Sprintf("%.2f", enhancement.FidelityWeight))
		frameType = " enhanced"
	}
	url := fmt.Sprintf("%s?%s", baseUrl, params.Encode())
	log.Debugf("Extracting%s frame from: %s ", frameType, url)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to extract frame: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("frame extraction failed: status %d", resp.StatusCode)
	}

	// Read frame bytes
	var buf bytes.Buffer
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read frame: %w", err)
	}

	log.Tracef("Frame extracted: %d bytes", buf.Len())
	return buf.Bytes(), nil
}
