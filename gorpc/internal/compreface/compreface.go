package compreface

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// ============================================================================
// Compreface HTTP Client - API Operations
// ============================================================================

// NewClient creates a new Compreface API client
func NewClient(baseURL string, recognitionKey string, detectionKey string, verificationKey string, minSimilarity float64) *Client {
	return &Client{
		BaseURL:         baseURL,
		RecognitionKey:  recognitionKey,
		DetectionKey:    detectionKey,
		VerificationKey: verificationKey,
		MinSimilarity:   minSimilarity,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// DetectFaces detects faces in an image file
// POST /api/v1/detection/detect
func (c *Client) DetectFaces(imagePath string) (*DetectionResponse, error) {
	url := fmt.Sprintf("%s/api/v1/detection/detect", c.BaseURL)

	// Read image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(imagePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = part.Write(imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to write image data: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-api-key", c.DetectionKey)

	// Send request
	log.Tracef("DetectFaces: POST %s", url)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var detection DetectionResponse
	err = json.Unmarshal(respBody, &detection)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Debugf("DetectFaces: Found %d face(s)", len(detection.Result))
	return &detection, nil
}

// DetectFacesFromBytes detects faces in image bytes
func (c *Client) DetectFacesFromBytes(imageBytes []byte, filename string) (*DetectionResponse, error) {
	url := fmt.Sprintf("%s/api/v1/detection/detect", c.BaseURL)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = part.Write(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to write image data: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-api-key", c.DetectionKey)

	// Send request
	log.Tracef("DetectFacesFromBytes: POST %s", url)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var detection DetectionResponse
	err = json.Unmarshal(respBody, &detection)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Debugf("DetectFacesFromBytes: Found %d face(s)", len(detection.Result))
	return &detection, nil
}

// RecognizeFaces recognizes faces in an image file
// POST /api/v1/recognition/recognize
//
// DEPRECATED: Submits full images to Compreface's internal detector (inferior to Vision Service).
// Prefer Vision Service + RecognizeFacesFromBytes() for cropped faces.
// Kept for backward compatibility only.
func (c *Client) RecognizeFaces(imagePath string) (*RecognitionResponse, error) {
	// Read image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}

	return c.RecognizeFacesFromBytes(imageData, filepath.Base(imagePath))
}

// RecognizeFacesFromBytes recognizes faces in image bytes
func (c *Client) RecognizeFacesFromBytes(imageBytes []byte, filename string) (*RecognitionResponse, error) {
	pluginArgs := "landmarks,gender,age,calculator,mask"
	url := fmt.Sprintf("%s/api/v1/recognition/recognize?face_plugins=%s", c.BaseURL, url.QueryEscape(pluginArgs))

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = part.Write(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to write image data: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-api-key", c.RecognitionKey)

	// Send request
	log.Tracef("RecognizeFaces: POST %s", url)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var recognition RecognitionResponse
	err = json.Unmarshal(respBody, &recognition)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Debugf("RecognizeFaces: Found %d face(s) with recognition data", len(recognition.Result))
	return &recognition, nil
}

// AddSubject adds a new subject with an image
// POST /api/v1/recognition/faces?subject={subject}
func (c *Client) AddSubject(subjectName string, imagePath string) (*AddSubjectResponse, error) {
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}

	return c.AddSubjectFromBytes(subjectName, imageData, filepath.Base(imagePath))
}

// AddSubjectFromBytes adds a new subject with image bytes
func (c *Client) AddSubjectFromBytes(subjectName string, imageBytes []byte, filename string) (*AddSubjectResponse, error) {
	reqURL := fmt.Sprintf("%s/api/v1/recognition/faces?subject=%s", c.BaseURL, url.QueryEscape(subjectName))

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = part.Write(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to write image data: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-api-key", c.RecognitionKey)

	// Send request
	log.Tracef("AddSubject: POST %s", reqURL)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var addResp AddSubjectResponse
	err = json.Unmarshal(respBody, &addResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Infof("AddSubject: Created subject '%s' with image_id=%s", subjectName, addResp.ImageID)
	return &addResp, nil
}

// ListSubjects lists all subjects
// GET /api/v1/recognition/subjects
func (c *Client) ListSubjects() ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/recognition/subjects", c.BaseURL)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.RecognitionKey)

	// Send request
	log.Tracef("ListSubjects: GET %s", url)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var listResp SubjectListResponse
	err = json.Unmarshal(respBody, &listResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Debugf("ListSubjects: Found %d subject(s)", len(listResp.Subjects))
	return listResp.Subjects, nil
}

// DeleteSubject deletes a subject
// DELETE /api/v1/recognition/subjects/{subject}
func (c *Client) DeleteSubject(subjectName string) error {
	url := fmt.Sprintf("%s/api/v1/recognition/subjects/%s", c.BaseURL, subjectName)

	// Create request
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.RecognitionKey)

	// Send request
	log.Tracef("DeleteSubject: DELETE %s", url)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	log.Infof("DeleteSubject: Deleted subject '%s'", subjectName)
	return nil
}

// ListFaces lists all faces for a subject
// GET /api/v1/recognition/faces?subject={subject}
func (c *Client) ListFaces(subjectName string) ([]FaceListItem, error) {
	url := fmt.Sprintf("%s/api/v1/recognition/faces?subject=%s", c.BaseURL, url.QueryEscape(subjectName))

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.RecognitionKey)

	// Send request
	log.Tracef("ListFaces: GET %s", url)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var listResp FaceListResponse
	err = json.Unmarshal(respBody, &listResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Debugf("ListFaces: Found %d face(s) for subject '%s'", len(listResp.Faces), subjectName)
	return listResp.Faces, nil
}

// DeleteFace deletes a specific face image
// DELETE /api/v1/recognition/faces/{image_id}
func (c *Client) DeleteFace(imageID string) error {
	url := fmt.Sprintf("%s/api/v1/recognition/faces/%s", c.BaseURL, imageID)

	// Create request
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.RecognitionKey)

	// Send request
	log.Tracef("DeleteFace: DELETE %s", url)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	log.Infof("DeleteFace: Deleted face image_id=%s", imageID)
	return nil
}

// SubjectImageURL constructs the URL to access a subject's image by image ID
func (c *Client) SubjectImageURL(imageID string) string {
	return fmt.Sprintf("%s/api/v1/static/%s/images/%s",
		c.BaseURL, c.RecognitionKey, imageID)
}

// ============================================================================
// Embedding-Based Recognition
// ============================================================================

// RecognizeEmbedding performs recognition using a pre-computed embedding
// POST /api/v1/recognition/embeddings/recognize?prediction_count=<n>
func (c *Client) RecognizeEmbedding(embedding []float64, predictionCount int) (*EmbeddingRecognitionResponse, error) {
	return c.RecognizeEmbeddings([][]float64{embedding}, predictionCount)
}

// RecognizeEmbeddings performs batch recognition for multiple embeddings
// POST /api/v1/recognition/embeddings/recognize?prediction_count=<n>
func (c *Client) RecognizeEmbeddings(embeddings [][]float64, predictionCount int) (*EmbeddingRecognitionResponse, error) {
	reqURL := fmt.Sprintf("%s/api/v1/recognition/embeddings/recognize?prediction_count=%d", c.BaseURL, predictionCount)

	// Create request body
	reqBody := EmbeddingRecognitionRequest{
		Embeddings: embeddings,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.RecognitionKey)

	// Send request
	log.Tracef("RecognizeEmbeddings: POST %s (%d embeddings)", reqURL, len(embeddings))
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var recognition EmbeddingRecognitionResponse
	err = json.Unmarshal(respBody, &recognition)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Debugf("RecognizeEmbeddings: Got results for %d embedding(s)", len(recognition.Result))
	return &recognition, nil
}
