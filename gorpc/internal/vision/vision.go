package vision

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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
// - FastAPI web service (port 5000)
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

// VisionServiceClient handles communication with Vision Service
type VisionServiceClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewVisionServiceClient creates a new client
func NewVisionServiceClient(baseURL string) *VisionServiceClient {
	return &VisionServiceClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ============================================================================
// Request/Response Types
// ============================================================================

// AnalyzeRequest represents job submission parameters (matching Vision API schema)
type AnalyzeRequest struct {
	VideoPath      string  `json:"video_path"`
	SceneID        string  `json:"scene_id"`
	JobID          string  `json:"job_id,omitempty"`
	ProcessingMode string  `json:"processing_mode,omitempty"` // sequential or parallel
	Modules        Modules `json:"modules"`
}

// Modules configures which analysis modules to enable
type Modules struct {
	Faces FacesModule `json:"faces"`
}

// FacesModule configuration
type FacesModule struct {
	Enabled    bool               `json:"enabled"`
	Parameters FacesParameters    `json:"parameters,omitempty"`
}

// FacesParameters configures face recognition behavior (matching openapi.yml)
type FacesParameters struct {
	MinConfidence                float64 `json:"min_confidence,omitempty"`                // default: 0.9
	MaxFaces                     int     `json:"max_faces,omitempty"`                     // default: 50
	SamplingInterval             float64 `json:"sampling_interval,omitempty"`             // default: 2.0
	UseSprites                   bool    `json:"use_sprites,omitempty"`                   // default: false
	SpriteVTTURL                 string  `json:"sprite_vtt_url,omitempty"`
	SpriteImageURL               string  `json:"sprite_image_url,omitempty"`
	EnableDeduplication          bool    `json:"enable_deduplication,omitempty"`          // default: true
	EmbeddingSimilarityThreshold float64 `json:"embedding_similarity_threshold,omitempty"` // default: 0.6
	DetectDemographics           bool    `json:"detect_demographics,omitempty"`           // default: true
	CacheDuration                int     `json:"cache_duration,omitempty"`                // default: 3600
}

// JobResponse represents job submission response
type JobResponse struct {
	JobID     string    `json:"job_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// JobStatus represents job status and progress
type JobStatus struct {
	JobID       string                 `json:"job_id"`
	Status      string                 `json:"status"`
	Progress    float64                `json:"progress"`
	Stage       string                 `json:"stage,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Summary     map[string]interface{} `json:"result_summary,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	FailedAt    *time.Time             `json:"failed_at,omitempty"`
}

// AnalyzeResults represents the full analysis results from Vision API
type AnalyzeResults struct {
	JobID    string         `json:"job_id"`
	SceneID  string         `json:"scene_id"`
	Status   string         `json:"status"`
	Faces    *FacesResults  `json:"faces,omitempty"`    // Faces module results
	Scenes   interface{}    `json:"scenes,omitempty"`   // Scenes module results (not used yet)
	Semantics interface{}   `json:"semantics,omitempty"` // Semantics module results (Phase 2)
	Objects  interface{}    `json:"objects,omitempty"`  // Objects module results (Phase 3)
	Metadata interface{}    `json:"metadata,omitempty"` // Processing metadata
}

// FacesResults represents face analysis results from the Faces service
type FacesResults struct {
	JobID    string         `json:"job_id"`
	SceneID  string         `json:"scene_id"`
	Status   string         `json:"status"`
	Faces    []VisionFace   `json:"faces"`
	Metadata ResultMetadata `json:"metadata"`
}

// VisionFace represents a unique face cluster detected in video
type VisionFace struct {
	FaceID                  string             `json:"face_id"`
	Embedding               []float64          `json:"embedding"` // 512-D ArcFace embedding
	Demographics            *Demographics      `json:"demographics,omitempty"`
	Detections              []VisionDetection  `json:"detections"`
	RepresentativeDetection VisionDetection    `json:"representative_detection"`
}

// Demographics represents age, gender, emotion detection
type Demographics struct {
	Age     int    `json:"age"`
	Gender  string `json:"gender"`  // "M" or "F"
	Emotion string `json:"emotion"` // neutral, happy, sad, angry, surprise, disgust, fear
}

// VisionDetection represents a single face detection in a frame
type VisionDetection struct {
	FrameIndex   int                    `json:"frame_index"`
	Timestamp    float64                `json:"timestamp"`
	BBox         VisionBoundingBox      `json:"bbox"`
	Confidence   float64                `json:"confidence"`
	QualityScore float64                `json:"quality_score"`
	Pose         string                 `json:"pose"`
	Landmarks    map[string]interface{} `json:"landmarks,omitempty"`
}

// VisionBoundingBox represents face coordinates
type VisionBoundingBox struct {
	XMin int `json:"x_min"`
	YMin int `json:"y_min"`
	XMax int `json:"x_max"`
	YMax int `json:"y_max"`
}

// ResultMetadata provides processing statistics
type ResultMetadata struct {
	TotalFrames           int     `json:"total_frames"`
	FramesProcessed       int     `json:"frames_processed"`
	UniqueFaces           int     `json:"unique_faces"`
	TotalDetections       int     `json:"total_detections"`
	ProcessingTimeSeconds float64 `json:"processing_time_seconds"`
	Method                string  `json:"method"`
	Model                 string  `json:"model"`
}

// ============================================================================
// API Methods
// ============================================================================

// SubmitJob submits a face recognition job to the Vision Service
func (c *VisionServiceClient) SubmitJob(req AnalyzeRequest) (*JobResponse, error) {
	url := fmt.Sprintf("%s/vision/analyze", c.BaseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Debugf("Submitting Vision Service job: scene_id=%s, video_path=%s", req.SceneID, req.VideoPath)

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

// BuildAnalyzeRequest creates a standard request with sensible defaults for face recognition
func BuildAnalyzeRequest(videoPath, sceneID string, useSprites bool, spriteVTT, spriteImage string) AnalyzeRequest {
	return AnalyzeRequest{
		VideoPath:      videoPath,
		SceneID:        sceneID,
		ProcessingMode: "sequential", // Sequential processing to avoid GPU memory contention
		Modules: Modules{
			Faces: FacesModule{
				Enabled: true,
				Parameters: FacesParameters{
					MinConfidence:                0.9,   // High confidence detections only
					MaxFaces:                     50,    // Maximum unique faces to extract
					SamplingInterval:             2.0,   // Sample every 2 seconds initially
					UseSprites:                   useSprites,
					SpriteVTTURL:                 spriteVTT,
					SpriteImageURL:               spriteImage,
					EnableDeduplication:          true,  // De-duplicate faces across video
					EmbeddingSimilarityThreshold: 0.6,   // Cosine similarity threshold for clustering
					DetectDemographics:           true,  // Detect age, gender, emotion
					CacheDuration:                3600,  // Cache for 1 hour
				},
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
// Uses the frame-server's /extract-frame endpoint via Vision API
func (c *VisionServiceClient) ExtractFrame(videoPath string, timestamp float64) ([]byte, error) {
	url := fmt.Sprintf("%s/extract-frame?video_path=%s&timestamp=%.2f&output_format=jpeg&quality=95",
		c.BaseURL, videoPath, timestamp)

	log.Debugf("Extracting frame: video=%s, timestamp=%.2fs", videoPath, timestamp)

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
