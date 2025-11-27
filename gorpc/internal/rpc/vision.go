package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"os"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
	"github.com/smegmarip/stash-compreface-plugin/internal/vision"
)

// ============================================================================
// Vision Service Shared Functions
// ============================================================================
//
// These functions are shared between image and scene processing pipelines.
// Both use the Vision Service for face detection/quality assessment and
// Compreface for subject matching/storage.
//
// Processing Flow:
// 1. Vision Service: Detect faces, assess quality, get demographics
// 2. Crop face region from source media
// 3. Compreface: Match against subjects or create new subject
// 4. Stash: Create/update performer, update media tags
//
// ============================================================================

// SourceType identifies the type of media being processed
type SourceType string

const (
	SourceTypeImage SourceType = "image"
	SourceTypeScene SourceType = "scene"
)

// ============================================================================
// Vision Service Job Submission
// ============================================================================

// BuildImageAnalyzeRequest creates a Vision Service request for image analysis
func (s *Service) BuildImageAnalyzeRequest(imagePath string, imageID string) vision.AnalyzeRequest {
	minConfidence := s.config.MinConfidenceScore
	minQuality := s.config.MinProcessingQualityScore
	qualityTrigger := s.config.EnhanceQualityScoreTrigger

	enhancementParams := vision.EnhancementParameters{
		Enabled:        true,
		QualityTrigger: qualityTrigger,
		Model:          "codeformer",
		FidelityWeight: 0.25,
	}

	parameters := vision.FacesParameters{
		FaceMinConfidence:  minConfidence,
		FaceMinQuality:     minQuality,
		MaxFaces:           10, // Images typically have fewer faces than video
		DetectDemographics: true,
		Enhancement:        &enhancementParams,
	}

	return vision.AnalyzeRequest{
		Source:         imagePath,
		SourceType:     "image",
		SourceID:       imageID,
		ProcessingMode: "sequential",
		Modules: vision.Modules{
			Faces: vision.FacesModule{
				Enabled:    true,
				Parameters: parameters,
			},
		},
	}
}

// SubmitImageJob submits an image to Vision Service and waits for results
func (s *Service) SubmitImageJob(visionClient *vision.VisionServiceClient, imagePath string, imageID string) (*vision.AnalyzeResults, error) {
	request := s.BuildImageAnalyzeRequest(imagePath, imageID)

	// Log request for debugging
	requestData, _ := json.Marshal(request)
	log.Debugf("Image %s: Submitting request to Vision Service: %s", imageID, string(requestData))

	// Submit job
	jobResp, err := visionClient.SubmitJob(request)
	if err != nil {
		return nil, fmt.Errorf("failed to submit job: %w", err)
	}

	log.Debugf("Image %s: Vision Service job submitted (job_id=%s)", imageID, jobResp.JobID)

	// Wait for completion
	results, err := visionClient.WaitForCompletion(jobResp.JobID, func(p float64) {
		log.Debugf("Image %s: Vision Service progress: %.1f%%", imageID, p*100)
	})
	if err != nil {
		return nil, fmt.Errorf("vision service job failed: %w", err)
	}

	return results, nil
}

// ============================================================================
// Image Loading Utilities
// ============================================================================

// LoadImageBytes loads an image file and returns it as JPEG bytes.
// Supports various formats: JPEG, PNG, GIF, BMP, WEBP.
// Note: Image format registration is done via blank imports in images.go
func LoadImageBytes(imagePath string) ([]byte, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Encode as JPEG
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("failed to encode as JPEG: %w", err)
	}

	return buf.Bytes(), nil
}

// ============================================================================
// Face Processing
// ============================================================================

// FaceProcessingContext provides context for face processing.
// Either Scene or ImageBytes must be provided.
type FaceProcessingContext struct {
	Scene      *stash.Scene // For scene processing (video/sprite extraction)
	ImageBytes []byte       // For image processing (pre-loaded image data)
	SourceID   string       // ID of the source (image ID or scene ID)
}

// processFace processes a single detected face from Vision Service.
// Used by both image and scene processing pipelines.
// Returns the performer ID if matched or created, empty string if skipped.
func (s *Service) processFace(visionClient *vision.VisionServiceClient, ctx FaceProcessingContext, face vision.VisionFace, metadata vision.ResultMetadata) (graphql.ID, error) {
	// Get the representative detection (best quality frame)
	det := face.RepresentativeDetection

	// check for null
	isEnhancedFace := det.Enhanced
	var frameEnhancement *vision.EnhancementParameters
	if metadata.FrameEnhancement != nil && isEnhancedFace {
		frameEnhancement = metadata.FrameEnhancement
	}

	// Assess face quality for recognition attempt (lower bar)
	qr := s.assessFaceQuality(det.Quality, s.config.MinProcessingQualityScore)

	log.Debugf("Processing face %s: timestamp=%.2fs, confidence=%.2f, quality=%.2f, size=%.2f, pose=%.2f, occlusion=%.2f, sharpness=%.2f, enhanced=%v, method=%s",
		face.FaceID, det.Timestamp, det.Confidence, qr.Composite, qr.Size, qr.Pose, qr.Occlusion, qr.Sharpness, isEnhancedFace, metadata.Method)

	if !qr.Acceptable {
		log.Debugf("Skipping face %s: %s", face.FaceID, qr.Reason)
		return "", nil
	}

	// Extract frame/thumbnail based on context
	var frameBytes []byte
	var err error

	if ctx.ImageBytes != nil {
		// Use pre-loaded image bytes (for image processing)
		frameBytes = ctx.ImageBytes
	} else if metadata.Method == "sprites" && ctx.Scene != nil {
		// Extract thumbnail from sprite image
		spriteVTT := s.NormalizeHost(ctx.Scene.Paths.VTT)
		spriteImage := s.NormalizeHost(ctx.Scene.Paths.Sprite)

		log.Debugf("Extracting face from sprite: vtt=%s, sprite=%s, timestamp=%.2f",
			spriteVTT, spriteImage, det.Timestamp)
		frameBytes, err = ExtractFromSprite(spriteImage, spriteVTT, det.Timestamp)
		if err != nil {
			return "", fmt.Errorf("failed to extract sprite thumbnail at %.2fs: %w", det.Timestamp, err)
		}
	} else if ctx.Scene != nil {
		// Extract frame from video at the representative detection timestamp
		videoPath := ctx.Scene.Files[0].Path
		frameBytes, err = visionClient.ExtractFrame(videoPath, det.Timestamp, frameEnhancement)
		if err != nil {
			return "", fmt.Errorf("failed to extract frame at %.2fs: %w", det.Timestamp, err)
		}
	} else {
		return "", fmt.Errorf("no scene or image bytes provided for frame extraction")
	}

	// Crop face from frame using bounding box
	faceCrop, err := s.cropFaceFromFrame(frameBytes, det.BBox)
	if err != nil {
		if faceCrop != nil {
			log.Warnf("Using uncropped frame for face %s due to cropping error: %v", face.FaceID, err)
		} else {
			return "", fmt.Errorf("failed to crop face: %w", err)
		}
	}

	log.Debugf("Extracted and cropped face from frame (%.0f bytes)", len(faceCrop))

	// Try to recognize face in Compreface
	recognitionResp, err := s.comprefaceClient.RecognizeFacesFromBytes(faceCrop, "face.jpg")
	if err != nil {
		return "", fmt.Errorf("compreface recognition failed: %w", err)
	}

	// Check if face matched to existing subject
	if len(recognitionResp.Result) > 0 && len(recognitionResp.Result[0].Subjects) > 0 {
		// Face matched to existing subject
		bestMatch := recognitionResp.Result[0].Subjects[0] // Highest similarity match
		if bestMatch.Similarity < s.config.MinSimilarity {
			// Similarity too low, treat as no match
			goto createNewSubject
		}

		subject := bestMatch.Subject
		similarity := bestMatch.Similarity

		log.Debugf("Face %s matched to Compreface subject %s (similarity: %.2f)", face.FaceID, subject, similarity)

		// Find performer with matching alias
		performerID, err := stash.FindPerformerBySubjectName(s.graphqlClient, subject)
		if err != nil {
			return "", fmt.Errorf("failed to find performer for subject %s: %w", subject, err)
		}

		if performerID != "" {
			// Get performer details for logging
			performerName := "Undetermined"
			performer, err := stash.GetPerformerByID(s.graphqlClient, performerID)
			if err == nil && performer != nil {
				performerName = performer.Name
			}
			log.Infof("Matched face %s to performer (name: %s, subject: %s, similarity: %.2f)",
				face.FaceID, performerName, subject, similarity)
			return performerID, nil
		}

		log.Warnf("Subject %s exists in Compreface but no matching performer found", subject)
		return "", nil
	}

createNewSubject:
	// Check quality for subject creation (higher bar than recognition)
	qrCreate := s.assessFaceQuality(det.Quality, s.config.MinQualityScore)
	if !qrCreate.Acceptable {
		log.Debugf("Skipping face %s for subject creation: %s", face.FaceID, qrCreate.Reason)
		return "", nil
	}

	// No match - create new subject and performer
	subjectName := createSubjectName(ctx.SourceID, face.FaceID)

	log.Debugf("Creating new subject for unmatched face %s (composite=%.2f)", face.FaceID, qrCreate.Composite)

	// Add subject to Compreface with face crop
	addResponse, err := s.comprefaceClient.AddSubjectFromBytes(subjectName, faceCrop, "face.jpg")
	if err != nil {
		return "", fmt.Errorf("failed to add subject to Compreface: %w", err)
	}

	log.Debugf("Created Compreface subject: %s (image_id: %s)", addResponse.Subject, addResponse.ImageID)

	// Create performer in Stash with demographics if available
	var gender string
	var age int
	if face.Demographics != nil {
		gender = face.Demographics.Gender
		age = face.Demographics.Age
	}

	performerSubject := stash.PerformerSubject{
		Name:   subjectName,
		Age:    age,
		Gender: gender,
		Image:  s.comprefaceClient.SubjectImageURL(addResponse.ImageID),
	}

	performer, err := s.createPerformerWithDetails(performerSubject)
	if err != nil {
		return "", fmt.Errorf("failed to create performer: %w", err)
	}

	log.Infof("Created new performer %s for unknown face %s (subject: %s, age: %d, gender: %s)",
		performer.Name, face.FaceID, subjectName, age, gender)

	return graphql.ID(performer.ID), nil
}

// cropFaceFromFrame crops a face region from a frame using the bounding box
func (s *Service) cropFaceFromFrame(frameBytes []byte, bbox vision.VisionBoundingBox) ([]byte, error) {
	// Decode frame bytes to image.Image
	img, _, err := image.Decode(bytes.NewReader(frameBytes))
	if err != nil {
		return frameBytes, fmt.Errorf("failed to decode frame: %w", err)
	}

	// Convert Vision bbox to Compreface bbox (same structure, just different types)
	cfBox := compreface.BoundingBox{
		XMin: bbox.XMin,
		YMin: bbox.YMin,
		XMax: bbox.XMax,
		YMax: bbox.YMax,
	}

	// Reuse existing cropping logic with padding
	padding := 10 // Match images.go behavior
	cropped, err := s.extractBoxImage(img, cfBox, padding)
	if err != nil {
		return frameBytes, fmt.Errorf("failed to crop face region: %w", err)
	}

	// Encode cropped image back to JPEG bytes
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, cropped, &jpeg.Options{Quality: 90}); err != nil {
		return frameBytes, fmt.Errorf("failed to encode cropped face: %w", err)
	}

	return buf.Bytes(), nil
}

// createSubjectName creates a unique subject name for Compreface
// Format: "Person {source_id} {random}"
func createSubjectName(sourceID, _ string) string {
	return compreface.CreateSubjectName(sourceID)
}

// ============================================================================
// Quality Assessment
// ============================================================================

// assessFaceQuality evaluates face quality components for CompreFace compatibility.
// Used by both image and scene processing pipelines.
//
// minComposite > 0 = override mode (flat composite threshold)
// minComposite = 0 = component gates mode (individual thresholds)
func (s *Service) assessFaceQuality(quality *vision.QualityResult, minComposite float64) FaceQualityResult {
	result := FaceQualityResult{
		Acceptable: true,
		Composite:  1.0,
		Size:       1.0,
		Pose:       1.0,
		Occlusion:  1.0,
		Sharpness:  1.0,
	}

	if quality == nil {
		return result // No quality data, assume acceptable
	}

	result.Composite = quality.Composite
	result.Size = quality.Components.Size
	result.Pose = quality.Components.Pose
	result.Occlusion = quality.Components.Occlusion
	result.Sharpness = quality.Components.Sharpness

	// Override mode: if minComposite > 0, use flat composite scoring only
	if minComposite > 0 {
		if result.Composite < minComposite {
			result.Acceptable = false
			result.Reason = fmt.Sprintf("composite=%.2f < %.2f", result.Composite, minComposite)
		}
		return result // Skip component gates
	}

	// Default mode: component-based gates
	if result.Size < 0.2 {
		result.Acceptable = false
		result.Reason = fmt.Sprintf("size=%.2f < 0.2", result.Size)
		return result
	}
	if result.Pose < 0.5 {
		result.Acceptable = false
		result.Reason = fmt.Sprintf("pose=%.2f < 0.5", result.Pose)
		return result
	}
	if result.Occlusion < 0.6 {
		result.Acceptable = false
		result.Reason = fmt.Sprintf("occlusion=%.2f < 0.6", result.Occlusion)
		return result
	}

	return result
}
