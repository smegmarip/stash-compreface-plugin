package vision

import (
	"net/http"
	"time"
)

// VisionServiceClient handles communication with Vision Service
type VisionServiceClient struct {
	BaseURL        string
	FrameServerURL string // Internal frame server container address
	HTTPClient     *http.Client
}

// ============================================================================
// Request/Response Types
// ============================================================================

// AnalyzeRequest represents job submission parameters (matching Vision API schema)
type AnalyzeRequest struct {
	Source         string  `json:"source"` // Renamed from video_path (breaking change v1.0.0)
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
	Enabled    bool            `json:"enabled"`
	Parameters FacesParameters `json:"parameters,omitempty"`
}

// FacesParameters configures face recognition behavior (matching openapi.yml)
type FacesParameters struct {
	FaceMinConfidence            float64                `json:"face_min_confidence,omitempty"` // Renamed from min_confidence (breaking change v1.0.0), server default: 0.9
	FaceMinQuality               float64                `json:"face_min_quality,omitempty"`    // Minimum quality threshold, server default: 0.0
	MaxFaces                     int                    `json:"max_faces,omitempty"`           // default: 50
	SamplingInterval             float64                `json:"sampling_interval,omitempty"`   // default: 2.0
	UseSprites                   bool                   `json:"use_sprites,omitempty"`         // default: false
	SpriteVTTURL                 string                 `json:"sprite_vtt_url,omitempty"`
	SpriteImageURL               string                 `json:"sprite_image_url,omitempty"`
	EnableDeduplication          bool                   `json:"enable_deduplication,omitempty"`           // default: true
	EmbeddingSimilarityThreshold float64                `json:"embedding_similarity_threshold,omitempty"` // default: 0.6
	DetectDemographics           bool                   `json:"detect_demographics,omitempty"`            // default: true
	CacheDuration                int                    `json:"cache_duration,omitempty"`                 // default: 3600
	Enhancement                  *EnhancementParameters `json:"enhancement,omitempty"`                    // Optional face enhancement settings
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
	JobID     string        `json:"job_id"`
	SceneID   string        `json:"scene_id"`
	Status    string        `json:"status"`
	Faces     *FacesResults `json:"faces,omitempty"`     // Faces module results
	Scenes    interface{}   `json:"scenes,omitempty"`    // Scenes module results (not used yet)
	Semantics interface{}   `json:"semantics,omitempty"` // Semantics module results (Phase 2)
	Objects   interface{}   `json:"objects,omitempty"`   // Objects module results (Phase 3)
	Metadata  interface{}   `json:"metadata,omitempty"`  // Processing metadata
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
	FaceID                  string            `json:"face_id"`
	Embedding               []float64         `json:"embedding"` // 512-D ArcFace embedding
	Demographics            *Demographics     `json:"demographics,omitempty"`
	Detections              []VisionDetection `json:"detections"`
	RepresentativeDetection VisionDetection   `json:"representative_detection"`
}

// Demographics represents age, gender, emotion detection
type Demographics struct {
	Age     int    `json:"age"`
	Gender  string `json:"gender"`  // "M" or "F"
	Emotion string `json:"emotion"` // neutral, happy, sad, angry, surprise, disgust, fear
}

// VisionDetection represents a single face detection in a frame
type VisionDetection struct {
	FrameIndex           int                    `json:"frame_index"`
	Timestamp            float64                `json:"timestamp"`
	BBox                 VisionBoundingBox      `json:"bbox"`
	Confidence           float64                `json:"confidence"`
	QualityScore         float64                `json:"quality_score"`
	Pose                 string                 `json:"pose"`
	Landmarks            map[string]interface{} `json:"landmarks,omitempty"`
	Enhanced             bool                   `json:"enhanced,omitempty"`              // True if face was enhanced via CodeFormer/GFPGAN (added v1.0.0)
	Occluded             bool                   `json:"occluded,omitempty"`              // True if face is occluded (glasses, mask, hand, etc.)
	OcclusionProbability float64                `json:"occlusion_probability,omitempty"` // Probability that face is occluded (0.0-1.0)
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
	Source                string                 `json:"source"` // Renamed from video_path (breaking change v1.0.0)
	TotalFrames           int                    `json:"total_frames"`
	FramesProcessed       int                    `json:"frames_processed"`
	UniqueFaces           int                    `json:"unique_faces"`
	TotalDetections       int                    `json:"total_detections"`
	ProcessingTimeSeconds float64                `json:"processing_time_seconds"`
	Method                string                 `json:"method"`
	Model                 string                 `json:"model"`
	FrameEnhancement      *EnhancementParameters `json:"frame_enhancement,omitempty"` // Enhancement settings used (null if disabled, added v1.0.0)
}

// EnhancementParameters defines face enhancement settings for low-quality detections
type EnhancementParameters struct {
	Enabled        bool    `json:"enabled"`                   // Enable face enhancement
	QualityTrigger float64 `json:"quality_trigger,omitempty"` // Trigger enhancement if quality < threshold (default: 0.5)
	Model          string  `json:"model,omitempty"`           // Enhancement model: "codeformer" or "gfpgan"
	FidelityWeight float64 `json:"fidelity_weight,omitempty"` // Fidelity vs quality tradeoff (0.0-1.0, default: 0.5)
}
