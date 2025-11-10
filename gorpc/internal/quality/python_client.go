package quality

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// PythonClient provides HTTP client for Python Quality Service
type PythonClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// PythonDetectRequest represents the detect endpoint request
type PythonDetectRequest struct {
	File io.Reader
}

// PythonDetectResponse represents the detect endpoint response
type PythonDetectResponse struct {
	Faces []PythonFaceDetection `json:"faces"`
}

// PythonFaceDetection matches the Python quality service response
type PythonFaceDetection struct {
	Box struct {
		XMin int `json:"x_min"`
		YMin int `json:"y_min"`
		XMax int `json:"x_max"`
		YMax int `json:"y_max"`
	} `json:"box"`
	Confidence struct {
		Score   float64 `json:"score"`
		Type    string  `json:"type"`
		TypeRaw int     `json:"type_raw"`
	} `json:"confidence"`
	CroppedSize []int `json:"cropped_size"`
}

// PythonAssessRequest represents the assess endpoint request
type PythonAssessRequest struct {
	Image string                  `json:"image"` // Base64 encoded image
	Faces []PythonFaceBoundingBox `json:"faces"`
}

// PythonFaceBoundingBox for assess request
type PythonFaceBoundingBox struct {
	Box map[string]int `json:"box"`
}

// PythonAssessResponse represents the assess endpoint response
type PythonAssessResponse struct {
	Faces []PythonFaceQualityMetrics `json:"faces"`
}

// PythonFaceQualityMetrics from quality assessment
type PythonFaceQualityMetrics struct {
	Index        int     `json:"index"`
	QualityScore float64 `json:"quality_score"`
	Confidence   float64 `json:"confidence"`
	Pose         string  `json:"pose"`
	IsFrontal    bool    `json:"is_frontal"`
	BlurScore    float64 `json:"blur_score"`
	Brightness   float64 `json:"brightness"`
}

// PythonPreprocessRequest represents the preprocess endpoint request
type PythonPreprocessRequest struct {
	Image      string   `json:"image"` // Base64 encoded image
	Operations []string `json:"operations"`
}

// PythonPreprocessResponse represents the preprocess endpoint response
type PythonPreprocessResponse struct {
	ProcessedImageBase64   string   `json:"processed_image_base64"`
	TransformationsApplied []string `json:"transformations_applied"`
}

// NewPythonClient creates a new Python Quality Service HTTP client
func NewPythonClient(baseURL string) *PythonClient {
	return &PythonClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetTimeout sets the HTTP client timeout
func (c *PythonClient) SetTimeout(timeout time.Duration) {
	c.HTTPClient.Timeout = timeout
}

// Health checks if the Python Quality Service is available
func (c *PythonClient) Health() error {
	url := fmt.Sprintf("%s/health", c.BaseURL)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// Detect performs face detection with quality metrics
func (c *PythonClient) Detect(imageData []byte) ([]FaceDetection, error) {
	url := fmt.Sprintf("%s/quality/detect", c.BaseURL)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "image.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, bytes.NewReader(imageData)); err != nil {
		return nil, fmt.Errorf("failed to copy image data: %w", err)
	}

	writer.Close()

	// Make request
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result PythonDetectResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert Python response to our FaceDetection type
	faces := make([]FaceDetection, len(result.Faces))
	for i, pFace := range result.Faces {
		faces[i] = FaceDetection{
			Box: BoundingBox{
				XMin: pFace.Box.XMin,
				YMin: pFace.Box.YMin,
				XMax: pFace.Box.XMax,
				YMax: pFace.Box.YMax,
			},
			Confidence: ConfidenceScore{
				Score:   pFace.Confidence.Score,
				Type:    pFace.Confidence.Type,
				TypeRaw: pFace.Confidence.TypeRaw,
			},
			Size: image.Point{
				X: pFace.CroppedSize[0],
				Y: pFace.CroppedSize[1],
			},
			Masked: false, // Python service doesn't provide mask info yet
		}
	}

	return faces, nil
}

// Assess performs quality assessment on detected faces
func (c *PythonClient) Assess(imageBase64 string, faces []BoundingBox) ([]PythonFaceQualityMetrics, error) {
	url := fmt.Sprintf("%s/quality/assess", c.BaseURL)

	// Convert faces to Python format
	pythonFaces := make([]PythonFaceBoundingBox, len(faces))
	for i, face := range faces {
		pythonFaces[i] = PythonFaceBoundingBox{
			Box: map[string]int{
				"x_min": face.XMin,
				"y_min": face.YMin,
				"x_max": face.XMax,
				"y_max": face.YMax,
			},
		}
	}

	reqBody := PythonAssessRequest{
		Image: imageBase64,
		Faces: pythonFaces,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result PythonAssessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Faces, nil
}

// Preprocess performs image preprocessing
func (c *PythonClient) Preprocess(imageBase64 string, operations []string) (string, []string, error) {
	url := fmt.Sprintf("%s/quality/preprocess", c.BaseURL)

	reqBody := PythonPreprocessRequest{
		Image:      imageBase64,
		Operations: operations,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result PythonPreprocessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.ProcessedImageBase64, result.TransformationsApplied, nil
}
