package compreface

import "net/http"

// Client handles API calls to Compreface service
type Client struct {
	BaseURL         string
	RecognitionKey  string
	DetectionKey    string
	VerificationKey string
	MinSimilarity   float64
	httpClient      *http.Client
}

// FaceDetection represents a detected face from Compreface
type FaceDetection struct {
	Box        BoundingBox      `json:"box"`
	Embedding  []float64        `json:"embedding"`
	Confidence float64          `json:"detection_probability"`
	Age        AgeRange         `json:"age"`
	Gender     Gender           `json:"gender"`
	Mask       Mask             `json:"mask"`
	Landmarks  map[string][]int `json:"landmarks"`
}

// BoundingBox represents face coordinates
type BoundingBox struct {
	XMin        int     `json:"x_min"`
	YMin        int     `json:"y_min"`
	XMax        int     `json:"x_max"`
	YMax        int     `json:"y_max"`
	Probability float64 `json:"probability"`
}

// AgeRange represents age estimation
type AgeRange struct {
	Low         int     `json:"low"`
	High        int     `json:"high"`
	Probability float64 `json:"probability"`
}

// Gender represents gender classification
type Gender struct {
	Value       string  `json:"value"`
	Probability float64 `json:"probability"`
}

// Mask represents mask detection
type Mask struct {
	Value       string  `json:"value"` // "without_mask", "with_mask", "mask_weared_incorrect"
	Probability float64 `json:"probability"`
}

// DetectionResponse is the response from face detection API
type DetectionResponse struct {
	Result          []FaceDetection   `json:"result"`
	PluginsVersions map[string]string `json:"plugins_versions"`
}

// FaceRecognition represents a recognized face match
type FaceRecognition struct {
	Subject    string  `json:"subject"`
	Similarity float64 `json:"similarity"`
	ImageID    string  `json:"image_id,omitempty"`
}

// RecognitionResult contains the recognition result for a single face
type RecognitionResult struct {
	Box       BoundingBox       `json:"box"`
	Subjects  []FaceRecognition `json:"subjects"`
	Embedding []float64         `json:"embedding,omitempty"`
	Age       AgeRange          `json:"age"`
	Gender    Gender            `json:"gender"`
	Mask      Mask              `json:"mask"`
}

// RecognitionResponse is the response from face recognition API
type RecognitionResponse struct {
	Result          []RecognitionResult `json:"result"`
	PluginsVersions map[string]string   `json:"plugins_versions"`
}

// AddSubjectResponse is the response from adding a subject
type AddSubjectResponse struct {
	ImageID string `json:"image_id"`
	Subject string `json:"subject"`
}

// SubjectListResponse is the response from listing subjects
type SubjectListResponse struct {
	Subjects []string `json:"subjects"`
}

// FaceListItem represents a face in a subject
type FaceListItem struct {
	ImageID string `json:"image_id"`
	Subject string `json:"subject"`
}

// FaceListResponse is the response from listing faces
type FaceListResponse struct {
	Faces []FaceListItem `json:"faces"`
}

// ============================================================================
// Embedding-Based Recognition Types
// ============================================================================

// EmbeddingRecognitionRequest for embedding-based recognition
type EmbeddingRecognitionRequest struct {
	Embeddings [][]float64 `json:"embeddings"`
}

// EmbeddingSimilarity represents a subject match from embedding recognition
type EmbeddingSimilarity struct {
	Subject    string  `json:"subject"`
	Similarity float64 `json:"similarity"`
}

// EmbeddingResult contains the result for a single embedding
type EmbeddingResult struct {
	Embedding    []float64             `json:"embedding"`
	Similarities []EmbeddingSimilarity `json:"similarities"`
}

// EmbeddingRecognitionResponse is the response from embedding recognition API
type EmbeddingRecognitionResponse struct {
	Result []EmbeddingResult `json:"result"`
}
