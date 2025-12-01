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
	// Read original image bytes
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	// Normalize EXIF orientation (returns original if no transformation needed)
	normalizedBytes, err := NormalizeImageOrientation(imageBytes)
	if err != nil {
		log.Warnf("Failed to normalize EXIF orientation for %s: %v (continuing with original)", imagePath, err)
		normalizedBytes = imageBytes
	}

	// Decode normalized image
	img, format, err := image.Decode(bytes.NewReader(normalizedBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	log.Debugf("Decoded image format: %s", format)

	// Re-encode as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("failed to encode image as JPEG: %w", err)
	}

	return buf.Bytes(), nil
}

// ============================================================================
// Face Processing
// ============================================================================

// processFace processes a single detected face from Vision Service.
// Used by both image and scene processing pipelines.
// Returns the performer ID if matched or created, empty string if skipped.
func (s *Service) processFace(visionClient *vision.VisionServiceClient, ctx FaceProcessingContext, face vision.VisionFace, metadata vision.ResultMetadata) (graphql.ID, error) {
	// Get the representative detection (best quality frame)
	det := face.RepresentativeDetection

	// check for null
	isEnhancedFace := det.Enhanced
	if metadata.FrameEnhancement != nil && isEnhancedFace {
	}

	// Assess face quality for recognition attempt (lower bar)
	qr := s.assessFaceQuality(det.Quality, s.config.MinProcessingQualityScore)

	log.Debugf("Processing face %s: timestamp=%.2fs, confidence=%.2f, quality=%.2f, size=%.2f, pose=%.2f, occlusion=%.2f, sharpness=%.2f, enhanced=%v, method=%s",
		face.FaceID, det.Timestamp, det.Confidence, qr.Composite, qr.Size, qr.Pose, qr.Occlusion, qr.Sharpness, isEnhancedFace, metadata.Method)

	if !qr.Acceptable {
		log.Debugf("Skipping face %s: %s", face.FaceID, qr.Reason)
		return "", nil
	}

	// Try embedding-based recognition first (if 512-D embedding available)
	if len(face.Embedding) == 512 {
		performerID, _ := s.recognizeEmbeddedStashFace(face)
		if performerID != "" {
			return performerID, nil
		}
	}

	// Extract frame/thumbnail based on context
	frameBytes, err := s.extractFrameBytesFromContext(visionClient, ctx, face, metadata)
	if err != nil {
		return "", err
	}

	// Crop face from frame using bounding box
	faceCrop, err := s.cropFaceFromFrame(frameBytes, det.BBox, 20)
	if err != nil {
		if faceCrop != nil {
			log.Warnf("Using uncropped frame for face %s due to cropping error: %v", face.FaceID, err)
		} else {
			return "", fmt.Errorf("failed to crop face: %w", err)
		}
	}

	// Save cropped face for debugging
	debugPath := fmt.Sprintf("/root/.stash/debug/face_%s.jpg", face.FaceID)
	err = os.WriteFile(debugPath, faceCrop, 0644)
	if err != nil {
		log.Warnf("Failed to save debug cropped face %s: %v", face.FaceID, err)
	} else {
		log.Debugf("Saved debug cropped face to %s", debugPath)
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

		// find and return existing performer by matched subject, or empty if not found
		return s.findExistingStashPerformerBySubject(bestMatch, face)
	}

createNewSubject:
	// first, create Compreface subject
	addResponse, err := s.createComprefaceSubject(faceCrop, ctx, face)
	if err != nil {
		return "", err
	}
	// then, create Stash performer from Compreface subject
	performerID, err := s.createStashPerformerFromComprefaceSubject(addResponse.ImageID, face, addResponse.Subject)
	if err != nil {
		return "", err
	}
	return performerID, nil
}

// processFaceForIdentification processes a Vision-detected face for the identify workflow.
// Returns FaceIdentity with metadata instead of just performerID.
// Respects createPerformer flag - if false, only attempts recognition without creation.
func (s *Service) processFaceForIdentification(
	visionClient *vision.VisionServiceClient,
	ctx FaceProcessingContext,
	face vision.VisionFace,
	metadata vision.ResultMetadata,
	createPerformer bool,
) (*FaceIdentity, error) {
	det := face.RepresentativeDetection

	// Quality check (lower bar for recognition attempt)
	qr := s.assessFaceQuality(det.Quality, s.config.MinProcessingQualityScore)
	if !qr.Acceptable {
		log.Debugf("Skipping face %s for identification: %s", face.FaceID, qr.Reason)
		return nil, nil
	}

	// Initialize FaceIdentity with Vision data
	identity := &FaceIdentity{
		ImageID: ctx.SourceID,
		BoundingBox: &compreface.BoundingBox{
			XMin: int(det.BBox.XMin),
			YMin: int(det.BBox.YMin),
			XMax: int(det.BBox.XMax),
			YMax: int(det.BBox.YMax),
		},
		Performer: PerformerData{},
	}
	if face.Demographics != nil {
		identity.Performer.Age = face.Demographics.Age
		identity.Performer.Gender = face.Demographics.Gender
	}

	var performerID graphql.ID
	var similarity float64

	// Step 1: Try embedding recognition
	if len(face.Embedding) == 512 {
		performerID, _ = s.recognizeEmbeddedStashFace(face)
		if performerID != "" {
			similarity = 0.95 // Embedding match is high confidence
		}
	}

	// Step 2-6: If no embedding match, try image-based or create
	if performerID == "" {
		// Step 2: Extract frame and crop face
		frameBytes, err := s.extractFrameBytesFromContext(visionClient, ctx, face, metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to extract frame: %w", err)
		}

		faceCrop, err := s.cropFaceFromFrame(frameBytes, det.BBox, 20)
		if err != nil && faceCrop == nil {
			return nil, fmt.Errorf("failed to crop face: %w", err)
		}

		// Step 3: Try image-based recognition
		recognitionResp, err := s.comprefaceClient.RecognizeFacesFromBytes(faceCrop, "face.jpg")
		if err != nil {
			return nil, fmt.Errorf("compreface recognition failed: %w", err)
		}

		// Step 4: Check if matched to existing subject
		if len(recognitionResp.Result) > 0 && len(recognitionResp.Result[0].Subjects) > 0 {
			bestMatch := recognitionResp.Result[0].Subjects[0]
			if bestMatch.Similarity >= s.config.MinSimilarity {
				performerID, _ = s.findExistingStashPerformerBySubject(bestMatch, face)
				similarity = bestMatch.Similarity
			}
		}

		// Step 5: No match found
		if performerID == "" {
			if !createPerformer {
				// Return identity without performer
				identity.Performer.Name = createSubjectName(ctx.SourceID, face.FaceID)
				conf := 0.0
				identity.Confidence = &conf
				log.Debugf("Face %s: No match, createPerformer=false, returning unmatched identity", face.FaceID)
				return identity, nil
			}

			// Step 6: Create new subject and performer
			addResponse, err := s.createComprefaceSubject(faceCrop, ctx, face)
			if err != nil {
				// Quality too low or creation failed
				identity.Performer.Name = createSubjectName(ctx.SourceID, face.FaceID)
				conf := 0.0
				identity.Confidence = &conf
				log.Debugf("Face %s: Failed to create subject: %v", face.FaceID, err)
				return identity, nil
			}

			performerID, err = s.createStashPerformerFromComprefaceSubject(addResponse.ImageID, face, addResponse.Subject)
			if err != nil {
				return nil, fmt.Errorf("failed to create performer: %w", err)
			}
			similarity = 1.0 // New creation, full confidence
		}
	}

	// Populate identity with performer (if matched or created)
	performer, err := stash.GetPerformerByID(s.graphqlClient, performerID)
	if err == nil && performer != nil {
		confidence := similarity * 100
		identity.Performer.ID = (*string)(&performer.ID)
		identity.Performer.Name = performer.Name
		identity.Confidence = &confidence
	}

	return identity, nil
}

// recognizeEmbeddedStashFace attempts to recognize and match a face to a Stash performer using its embedding.
func (s *Service) recognizeEmbeddedStashFace(face vision.VisionFace) (graphql.ID, error) {
	// Try embedding-based recognition first (if 512-D embedding available)
	if len(face.Embedding) == 512 {
		performerID, similarity, err := s.recognizeByEmbedding(face.Embedding)
		if err != nil {
			log.Debugf("Face %s: Embedding recognition failed: %v, trying image-based", face.FaceID, err)
		} else if performerID != "" {
			// Get performer details for logging
			performerName := "Undetermined"
			performer, err := stash.GetPerformerByID(s.graphqlClient, performerID)
			if err == nil && performer != nil {
				performerName = performer.Name
			}
			log.Infof("Face %s: Matched via embedding (name: %s, similarity: %.2f)", face.FaceID, performerName, similarity)
			return performerID, nil
		} else {
			log.Debugf("Face %s: No embedding match found, trying image-based", face.FaceID)
		}
	}
	return "", nil
}

// extractFrameBytesFromContext extracts the appropriate frame bytes based on the processing context.
func (s *Service) extractFrameBytesFromContext(visionClient *vision.VisionServiceClient, ctx FaceProcessingContext, face vision.VisionFace, metadata vision.ResultMetadata) ([]byte, error) {
	// Get the representative detection (best quality frame)
	det := face.RepresentativeDetection

	// check for null
	isEnhancedFace := det.Enhanced
	var frameEnhancement *vision.EnhancementParameters
	if metadata.FrameEnhancement != nil && isEnhancedFace {
		frameEnhancement = metadata.FrameEnhancement
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
			return nil, fmt.Errorf("failed to extract sprite thumbnail at %.2fs: %w", det.Timestamp, err)
		}
	} else if ctx.Scene != nil {
		// Extract frame from video at the representative detection timestamp
		videoPath := ctx.Scene.Files[0].Path
		frameBytes, err = visionClient.ExtractFrame(videoPath, det.Timestamp, frameEnhancement)
		if err != nil {
			return nil, fmt.Errorf("failed to extract frame at %.2fs: %w", det.Timestamp, err)
		}
	} else {
		return nil, fmt.Errorf("no scene or image bytes provided for frame extraction")
	}
	return frameBytes, nil
}

// findExistingStashPerformerBySubject finds a Stash performer by Compreface subject name from recognition result.
func (s *Service) findExistingStashPerformerBySubject(recognitionResult compreface.FaceRecognition, face vision.VisionFace) (graphql.ID, error) {
	subject := recognitionResult.Subject
	similarity := recognitionResult.Similarity

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

// createComprefaceSubject creates a new subject in Compreface for an unmatched face.
func (s *Service) createComprefaceSubject(faceImage []byte, ctx FaceProcessingContext, face vision.VisionFace) (*compreface.AddSubjectResponse, error) {
	// Get the representative detection (best quality frame)
	det := face.RepresentativeDetection

	// Check quality for subject creation (higher bar than recognition)
	qrCreate := s.assessFaceQuality(det.Quality, s.config.MinQualityScore)
	if !qrCreate.Acceptable {
		err := fmt.Errorf("skipping face %s for subject creation: %s", face.FaceID, qrCreate.Reason)
		log.Debugf(err.Error())
		return nil, err
	}

	// No match - create new subject and performer
	subjectName := createSubjectName(ctx.SourceID, face.FaceID)

	log.Debugf("Creating new subject for unmatched face %s (composite=%.2f)", face.FaceID, qrCreate.Composite)

	// Add subject to Compreface with face crop
	addResponse, err := s.comprefaceClient.AddSubjectFromBytes(subjectName, faceImage, "face.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to add subject to Compreface: %w", err)
	}

	log.Debugf("Created Compreface subject: %s (image_id: %s)", addResponse.Subject, addResponse.ImageID)

	return addResponse, nil
}

// createStashPerformerFromComprefaceSubject creates a new Stash performer from a Compreface subject.
func (s *Service) createStashPerformerFromComprefaceSubject(comprefaceImageId string, face vision.VisionFace, subjectName string) (graphql.ID, error) {

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
		Image:  s.comprefaceClient.SubjectImageURL(comprefaceImageId),
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
func (s *Service) cropFaceFromFrame(frameBytes []byte, bbox vision.VisionBoundingBox, padding int) ([]byte, error) {
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

// ============================================================================
// Embedding-Based Recognition
// ============================================================================

// recognizeByEmbedding attempts to match a face using its pre-computed embedding.
// Returns performer ID and similarity if matched, empty string if no match.
func (s *Service) recognizeByEmbedding(embedding []float64) (graphql.ID, float64, error) {
	resp, err := s.comprefaceClient.RecognizeEmbedding(embedding, 1)
	if err != nil {
		return "", 0, err
	}

	if len(resp.Result) > 0 && len(resp.Result[0].Similarities) > 0 {
		best := resp.Result[0].Similarities[0]
		log.Debugf("Embedding recognition best match: subject=%s, similarity=%.2f", best.Subject, best.Similarity)
		if best.Similarity >= s.config.MinSimilarity {
			// Find performer by subject name
			performerID, err := stash.FindPerformerBySubjectName(s.graphqlClient, best.Subject)
			if err != nil {
				return "", 0, fmt.Errorf("failed to find performer for subject %s: %w", best.Subject, err)
			}
			if performerID != "" {
				return performerID, best.Similarity, nil
			}
		}
	}
	return "", 0, nil
}
