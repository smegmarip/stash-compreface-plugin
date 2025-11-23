package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
	"github.com/smegmarip/stash-compreface-plugin/internal/vision"
)

// recognizeScenes performs face recognition on scenes using Vision Service
func (s *Service) recognizeScenes(useSprites bool, scanPartial bool, limit int) error {
	// Check if Vision Service is configured
	if s.config.VisionServiceURL == "" {
		return fmt.Errorf("vision service URL not configured")
	}

	// Initialize Vision Service client
	visionClient := vision.NewVisionServiceClient(s.config.VisionServiceURL)

	// Health check
	if err := visionClient.HealthCheck(); err != nil {
		log.Errorf("Health check failed: %v", err)
		return fmt.Errorf("vision service health check failed: %w", err)
	}

	filterTagName := s.config.ScannedTagName

	log.Debugf("Starting scene recognition (useSprites=%t, scanPartial=%t, limit=%d)", useSprites, scanPartial, limit)

	// Get or create tags
	scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, filterTagName, "Compreface Scanned")
	if err != nil {
		return fmt.Errorf("failed to get scanned tag: %w", err)
	}

	matchedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.MatchedTagName, "Compreface Matched")
	if err != nil {
		return fmt.Errorf("failed to get matched tag: %w", err)
	}

	// Fetch scenes in batches
	page := 0
	batchSize := s.config.MaxBatchSize
	processedCount := 0
	total := 0

	for {
		if s.stopping {
			return fmt.Errorf("task cancelled")
		}

		page++

		// Query scenes
		var scenes []stash.Scene
		var sceneCount int
		var err error
		if scanPartial {
			scenes, sceneCount, err = findScenes(s.graphqlClient, nil, 1, batchSize)
		} else {
			scenes, sceneCount, err = findScenes(s.graphqlClient, &scannedTagID, 1, batchSize)
		}
		if err != nil {
			return fmt.Errorf("failed to query scenes: %w", err)
		}

		if page == 1 {
			total = sceneCount

			// Apply limit if specified
			if limit > 0 && limit < total {
				total = limit
				log.Infof("Found %d scenes, limiting to %d", sceneCount, limit)
			} else {
				log.Infof("Found %d scenes to process", total)
			}
		}

		if len(scenes) == 0 {
			break
		}

		log.Infof("Processing batch %d: %d scenes", page, len(scenes))

		// Process each scene
		for _, scene := range scenes {
			if s.stopping {
				return fmt.Errorf("task cancelled")
			}

			// Check if limit reached
			if limit > 0 && processedCount >= limit {
				log.Infof("Reached limit of %d scenes, stopping", limit)
				break
			}

			processedCount++
			progress := float64(processedCount) / float64(total)
			log.Progress(progress)

			log.Infof("[%d/%d] Processing scene %s", processedCount, total, scene.ID)

			err := s.processScene(visionClient, scene, scannedTagID, matchedTagID, useSprites)
			if err != nil {
				log.Warnf("Failed to process scene %s: %v", scene.ID, err)
				continue
			}
		}

		// Break outer loop if limit reached
		if limit > 0 && processedCount >= limit {
			break
		}

		// Apply cooldown after batch
		if len(scenes) == batchSize && processedCount < total {
			s.applyCooldown()
		}

		if len(scenes) < batchSize {
			break
		}
	}

	log.Progress(1.0)
	log.Infof("Scene recognition completed: %d scenes processed", processedCount)

	// Trigger metadata scan
	if err := stash.TriggerMetadataScan(s.graphqlClient); err != nil {
		log.Warnf("Failed to trigger metadata scan: %v", err)
	}

	return nil
}

// processScene processes a single scene through Vision Service
func (s *Service) processScene(visionClient *vision.VisionServiceClient, scene stash.Scene, scannedTagID, matchedTagID graphql.ID, useSprites bool) error {
	// Get video path from files
	if len(scene.Files) == 0 {
		return fmt.Errorf("scene %s has no files", scene.ID)
	}
	videoPath := scene.Files[0].Path

	// Build Vision Service request
	var spriteVTT, spriteImage string
	if useSprites {
		spriteVTT = s.NormalizeHost(scene.Paths.VTT)
		spriteImage = s.NormalizeHost(scene.Paths.Sprite)
	}

	minConfidence := s.config.MinSceneConfidenceScore
	minQuality := s.config.MinSceneProcessingQualityScore
	qualityTrigger := s.config.EnhanceQualityScoreTrigger

	enhancementParams := vision.EnhancementParameters{
		Enabled:        true,
		QualityTrigger: qualityTrigger,
		Model:          "codeformer",
		FidelityWeight: 0.25,
	}

	parameters := vision.FacesParameters{
		FaceMinConfidence:            minConfidence, // Mid-High confidence detections only
		FaceMinQuality:               minQuality,    // Minimum quality threshold
		MaxFaces:                     50,            // Maximum unique faces to extract
		SamplingInterval:             2.0,           // Sample every 2 seconds initially
		UseSprites:                   useSprites,
		SpriteVTTURL:                 spriteVTT,
		SpriteImageURL:               spriteImage,
		EnableDeduplication:          true,               // De-duplicate faces across video
		EmbeddingSimilarityThreshold: 0.6,                // Cosine similarity threshold for clustering
		DetectDemographics:           true,               // Detect age, gender, emotion
		CacheDuration:                3600,               // Cache for 1 hour
		Enhancement:                  &enhancementParams, // Enable face enhancement
	}

	request := vision.BuildAnalyzeRequest(videoPath, string(scene.ID), parameters)

	// marshall request into json for logging
	requestData, _ := json.Marshal(request)

	log.Debugf("Scene %s: Submitting request to Vision Service: %s", scene.ID, string(requestData))

	// Submit job
	jobResp, err := visionClient.SubmitJob(request)
	if err != nil {
		return fmt.Errorf("failed to submit job: %w", err)
	}

	log.Debugf("Scene %s: Vision Service job submitted (job_id=%s)", scene.ID, jobResp.JobID)

	// Wait for completion with progress updates
	results, err := visionClient.WaitForCompletion(jobResp.JobID, func(p float64) {
		log.Debugf("Scene %s: Vision Service progress: %.1f%%", scene.ID, p*100)
	})
	log.Debugf("Error from Vision Service: %v", err)
	if err != nil {
		return fmt.Errorf("vision service job failed: %w", err)
	}

	// Check if faces were found
	if results.Faces == nil || len(results.Faces.Faces) == 0 {
		log.Infof("Scene %s: No faces detected", scene.ID)
		// Add scanned tag
		if err := addTagToScene(s.graphqlClient, scene.ID, scannedTagID); err != nil {
			log.Warnf("Failed to add scanned tag to scene %s: %v", scene.ID, err)
		}
		return nil
	}

	facesDetected := 0
	for _, face := range results.Faces.Faces {
		det := face.RepresentativeDetection
		if det.QualityScore >= s.config.MinSceneProcessingQualityScore {
			facesDetected++
		}
	}
	log.Infof("Scene %s: Found %d processable faces out of %d total faces", scene.ID, facesDetected, len(results.Faces.Faces))

	// Get result requestMetadata
	requestMetadata := results.Faces.Metadata

	// Process each face and track results
	matchedPerformers := []graphql.ID{}
	facesProcessed := 0 // Faces that were either matched or created as new subjects

	for _, face := range results.Faces.Faces {
		performerID, err := s.processSceneFace(visionClient, scene, face, requestMetadata)
		if err != nil {
			log.Warnf("Failed to process face %s: %v", face.FaceID, err)
			continue
		}
		if performerID != "" {
			matchedPerformers = append(matchedPerformers, performerID)
			facesProcessed++
		}
	}

	// Update scene with matched performers
	if len(matchedPerformers) > 0 {
		log.Infof("Scene %s: Matched/created %d performers", scene.ID, len(matchedPerformers))
		if err := updateScenePerformers(s.graphqlClient, scene.ID, matchedPerformers); err != nil {
			log.Warnf("Failed to update scene performers: %v", err)
		}

		// Add matched tag
		if err := addTagToScene(s.graphqlClient, scene.ID, matchedTagID); err != nil {
			log.Warnf("Failed to add matched tag: %v", err)
		}
	}

	// Add scanned tag
	if err := addTagToScene(s.graphqlClient, scene.ID, scannedTagID); err != nil {
		log.Warnf("Failed to add scanned tag: %v", err)
	}

	// Apply partial/complete tagging logic
	if err := s.applySceneCompletionTags(scene.ID, facesDetected, facesProcessed); err != nil {
		log.Warnf("Failed to apply completion tags: %v", err)
	}

	return nil
}

// processSceneFace processes a single detected face from Vision Service
func (s *Service) processSceneFace(visionClient *vision.VisionServiceClient, scene stash.Scene, face vision.VisionFace, metadata vision.ResultMetadata) (graphql.ID, error) {
	// Get the representative detection (best quality frame)
	det := face.RepresentativeDetection

	// check for null
	isEnhancedFace := det.Enhanced
	var frameEnhancement *vision.EnhancementParameters
	if metadata.FrameEnhancement != nil && isEnhancedFace {
		frameEnhancement = metadata.FrameEnhancement
	}

	log.Debugf("Processing face %s: timestamp=%.2fs, confidence=%.2f, quality=%.2f enhanced=%v occluded=%v method=%s",
		face.FaceID, det.Timestamp, det.Confidence, det.QualityScore, isEnhancedFace, det.Occluded, metadata.Method)

	if det.Occluded {
		log.Debugf("Detected occluded face %s (occlusion_probability=%.2f)", face.FaceID, det.OcclusionProbability)
	}

	// Extract frame/thumbnail based on detection method
	var frameBytes []byte
	var err error

	spriteVTT := s.NormalizeHost(scene.Paths.VTT)
	spriteImage := s.NormalizeHost(scene.Paths.Sprite)

	if metadata.Method == "sprites" {
		// Extract thumbnail from sprite image
		log.Debugf("Extracting face from sprite: vtt=%s, sprite=%s, timestamp=%.2f",
			spriteVTT, spriteImage, det.Timestamp)
		frameBytes, err = ExtractFromSprite(spriteImage, spriteVTT, det.Timestamp)
		if err != nil {
			return "", fmt.Errorf("failed to extract sprite thumbnail at %.2fs: %w", det.Timestamp, err)
		}
	} else {
		// Extract frame from video at the representative detection timestamp
		videoPath := scene.Files[0].Path
		frameBytes, err = visionClient.ExtractFrame(videoPath, det.Timestamp, frameEnhancement)
		if err != nil {
			return "", fmt.Errorf("failed to extract frame at %.2fs: %w", det.Timestamp, err)
		}
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
	// Check quality score before creating new subject
	// Only create subjects from high-quality unmatched faces
	if det.QualityScore < s.config.MinSceneQualityScore {
		log.Debugf("Skipping low-quality unmatched face %s (quality: %.2f < threshold: %.2f)",
			face.FaceID, det.QualityScore, s.config.MinSceneQualityScore)
		return "", nil
	}

	// No match - create new subject and performer
	subjectName := createSubjectName(string(scene.ID), face.FaceID)

	log.Debugf("Creating new subject for unmatched face %s (quality: %.2f >= threshold: %.2f)",
		face.FaceID, det.QualityScore, s.config.MinSceneQualityScore)

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
// Format: "Person {scene_id} {random}"
func createSubjectName(sceneID, _ string) string {
	return compreface.CreateSubjectName(sceneID)
}

// applySceneCompletionTags applies partial/complete tags based on face processing results
func (s *Service) applySceneCompletionTags(sceneID graphql.ID, facesDetected, facesProcessed int) error {
	// Skip completion tagging if no faces were processed (all skipped due to quality or errors)
	if facesProcessed == 0 {
		log.Debugf("Scene %s: No faces processed, skipping partial/complete tagging", sceneID)
		return nil
	}

	var completionTag string
	var removeTag string

	// Determine completion status
	if facesProcessed == facesDetected {
		// All faces matched or created - complete
		completionTag = s.config.CompleteTagName
		removeTag = s.config.PartialTagName
		log.Infof("Scene %s: All %d face(s) processed - marking as Complete", sceneID, facesDetected)
	} else {
		// Some faces skipped (low quality) - partial
		completionTag = s.config.PartialTagName
		removeTag = s.config.CompleteTagName
		log.Infof("Scene %s: %d/%d face(s) processed - marking as Partial", sceneID, facesProcessed, facesDetected)
	}

	// Get current scene to retrieve existing tags
	scene, err := stash.GetScene(s.graphqlClient, sceneID)
	if err != nil {
		return fmt.Errorf("failed to get scene: %w", err)
	}

	// Build list of tag IDs, removing the opposite completion tag
	removeTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, removeTag, removeTag)
	if err != nil {
		return fmt.Errorf("failed to get remove tag: %w", err)
	}

	// Get completion tag ID
	completionTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, completionTag, completionTag)
	if err != nil {
		return fmt.Errorf("failed to get completion tag: %w", err)
	}

	// Build new tag list: existing tags minus removeTag, plus completionTag
	tagIDs := []graphql.ID{}
	hasCompletionTag := false

	for _, tag := range scene.Tags {
		if tag.ID == removeTagID {
			// Skip the tag we want to remove
			continue
		}
		if tag.ID == completionTagID {
			hasCompletionTag = true
		}
		tagIDs = append(tagIDs, tag.ID)
	}

	// Add completion tag if not already present
	if !hasCompletionTag {
		tagIDs = append(tagIDs, completionTagID)
	}

	// Update scene tags
	return stash.UpdateSceneTags(s.graphqlClient, sceneID, tagIDs)
}

// Helper functions for scene GraphQL operations

// Find scenes with filtering
func findScenes(client *graphql.Client, scannedTagID *graphql.ID, page, perPage int) ([]stash.Scene, int, error) {
	var tagsFilter stash.HierarchicalMultiCriterionInput
	var filter stash.SceneFilterType = stash.SceneFilterType{}

	// Build filter to exclude already scanned scenes
	if scannedTagID != nil {
		tagsFilter = stash.HierarchicalMultiCriterionInput{
			Value:    []string{string(*scannedTagID)},
			Modifier: stash.CriterionModifierExcludes,
		}
		filter.Tags = &tagsFilter
	}

	return stash.FindScenes(client, &filter, page, perPage)
}

// Add tag to scene (preserving existing tags)
func addTagToScene(client *graphql.Client, sceneID graphql.ID, tagID graphql.ID) error {
	return stash.AddTagToScene(client, sceneID, tagID)
}

// Update scene performers (preserving existing performers)
func updateScenePerformers(client *graphql.Client, sceneID graphql.ID, performerIDs []graphql.ID) error {
	return stash.UpdateScenePerformers(client, sceneID, performerIDs)
}

// createPerformerWithDetails creates a performer with the given subject details
func (s *Service) createPerformerWithDetails(performerSubject stash.PerformerSubject) (*stash.Performer, error) {
	performerID, err := stash.CreatePerformer(s.graphqlClient, performerSubject)
	if err != nil {
		return nil, err
	}

	return &stash.Performer{
		ID:   performerID,
		Name: performerSubject.Name,
	}, nil
}

// resetUnmatchedScenes removes scanned tags from unmatched scenes
func (s *Service) resetUnmatchedScenes(limit int) error {
	if s.stopping {
		return fmt.Errorf("operation cancelled")
	}

	log.Infof("Starting reset of unmatched scenes (limit=%d)", limit)

	// Step 1: Get tag IDs
	scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.ScannedTagName, "Compreface Scanned")
	if err != nil {
		return fmt.Errorf("failed to get scanned tag: %w", err)
	}

	matchedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.MatchedTagName, "Compreface Matched")
	if err != nil {
		return fmt.Errorf("failed to get matched tag: %w", err)
	}

	log.Infof("Searching for unmatched scenes (scanned but not matched)")

	// Step 2: Find scenes with scanned tag but no matched tag
	tagsFilter := stash.HierarchicalMultiCriterionInput{
		Value:    []string{string(scannedTagID)},
		Modifier: stash.CriterionModifierIncludesAll,
	}
	filter := stash.SceneFilterType{
		Tags: &tagsFilter,
	}

	scenes, count, err := stash.FindScenes(s.graphqlClient, &filter, 1, -1)
	if err != nil {
		return fmt.Errorf("failed to query scenes: %w", err)
	}

	log.Infof("Found %d scanned scenes", count)

	// Step 3: Filter for images without matched tag
	var unmatchedScenes []graphql.ID
	for _, scene := range scenes {
		hasMatchedTag := false
		for _, tag := range scene.Tags {
			if tag.ID == matchedTagID {
				hasMatchedTag = true
				break
			}
		}

		if !hasMatchedTag {
			unmatchedScenes = append(unmatchedScenes, scene.ID)
		}
	}

	if len(unmatchedScenes) == 0 {
		log.Info("No unmatched scenes found")
		return nil
	}

	total := len(unmatchedScenes)

	// Apply limit if specified
	if limit > 0 && limit < total {
		unmatchedScenes = unmatchedScenes[:limit]
		log.Infof("Found %d unmatched scenes, limiting to %d", total, limit)
	} else {
		log.Infof("Found %d unmatched scenes to reset", total)
	}

	// Step 4: Remove scanned tag from unmatched scenes
	resetCount := 0
	for i, sceneID := range unmatchedScenes {
		if s.stopping {
			return fmt.Errorf("operation cancelled")
		}

		progress := float64(i) / float64(len(unmatchedScenes))
		log.Progress(progress)

		err := stash.RemoveTagFromScene(s.graphqlClient, sceneID, scannedTagID)
		if err != nil {
			log.Warnf("Failed to remove tag from scene %s: %v", sceneID, err)
			continue
		}

		resetCount++
		log.Debugf("Reset scene %s (%d/%d)", sceneID, i+1, len(unmatchedScenes))
	}

	log.Progress(1.0)
	log.Infof("Reset complete: %d scenes processed", resetCount)

	return nil
}
