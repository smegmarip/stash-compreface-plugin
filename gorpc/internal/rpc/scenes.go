package rpc

import (
	"fmt"
	"io"
	"net/http"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
	"github.com/smegmarip/stash-compreface-plugin/internal/vision"
)

// recognizeScenes performs face recognition on scenes using Vision Service
func (s *Service) recognizeScenes(useSprites bool) error {
	// Check if Vision Service is configured
	if s.config.VisionServiceURL == "" {
		return fmt.Errorf("vision service URL not configured")
	}

	// Initialize Vision Service client
	visionClient := vision.NewVisionServiceClient(s.config.VisionServiceURL)

	// Health check
	if err := visionClient.HealthCheck(); err != nil {
		return fmt.Errorf("vision service health check failed: %w", err)
	}

	log.Info("Vision Service is healthy, starting scene recognition")

	// Get or create tags
	scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.ScannedTagName, "Compreface Scanned")
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

	for {
		if s.stopping {
			return fmt.Errorf("task cancelled")
		}

		page++

		// Query scenes
		scenes, total, err := findScenes(s.graphqlClient, page, batchSize)
		if err != nil {
			return fmt.Errorf("failed to query scenes: %w", err)
		}

		if len(scenes) == 0 {
			break
		}

		log.Infof("Processing batch %d: %d scenes (total: %d)", page, len(scenes), total)

		// Process each scene
		for i, scene := range scenes {
			if s.stopping {
				return fmt.Errorf("task cancelled")
			}

			progress := float64((page-1)*batchSize+i) / float64(total)
			log.Progress(progress)

			log.Infof("[%d/%d] Processing scene %s", (page-1)*batchSize+i+1, total, scene.ID)

			err := s.processScene(visionClient, scene, scannedTagID, matchedTagID, useSprites)
			if err != nil {
				log.Warnf("Failed to process scene %s: %v", scene.ID, err)
				continue
			}
		}

		// Apply cooldown after batch
		if len(scenes) == batchSize {
			s.applyCooldown()
		}

		if len(scenes) < batchSize {
			break
		}
	}

	log.Progress(1.0)
	log.Info("Scene recognition completed")

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
		spriteVTT = scene.Paths.VTT
		spriteImage = scene.Paths.Sprite
	}

	request := vision.BuildAnalyzeRequest(videoPath, string(scene.ID), useSprites, spriteVTT, spriteImage)

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

	log.Infof("Scene %s: Found %d unique faces", scene.ID, len(results.Faces.Faces))

	// Process each face
	matchedPerformers := []graphql.ID{}
	for _, face := range results.Faces.Faces {
		performerID, err := s.processSceneFace(scene, face)
		if err != nil {
			log.Warnf("Failed to process face %s: %v", face.FaceID, err)
			continue
		}
		if performerID != "" {
			matchedPerformers = append(matchedPerformers, performerID)
		}
	}

	// Update scene with matched performers
	if len(matchedPerformers) > 0 {
		log.Infof("Scene %s: Matched %d performers", scene.ID, len(matchedPerformers))
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

	return nil
}

// processSceneFace processes a single detected face from Vision Service
func (s *Service) processSceneFace(scene stash.Scene, face vision.VisionFace) (graphql.ID, error) {
	// Get the representative detection (best quality frame)
	det := face.RepresentativeDetection

	log.Debugf("Processing face %s: timestamp=%.2fs, confidence=%.2f, quality=%.2f",
		face.FaceID, det.Timestamp, det.Confidence, det.QualityScore)

	// Extract frame from video at the representative detection timestamp
	videoPath := scene.Files[0].Path
	frameBytes, err := s.extractFrameAtTimestamp(videoPath, det.Timestamp)
	if err != nil {
		return "", fmt.Errorf("failed to extract frame at %.2fs: %w", det.Timestamp, err)
	}

	// Crop face from frame using bounding box
	faceCrop, err := s.cropFaceFromFrame(frameBytes, det.BBox)
	if err != nil {
		return "", fmt.Errorf("failed to crop face: %w", err)
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

		// Find performer with matching alias
		performerID, err := stash.FindPerformerBySubjectName(s.graphqlClient, subject)
		if err != nil {
			return "", fmt.Errorf("failed to find performer for subject %s: %w", subject, err)
		}

		if performerID != "" {
			// Get performer details for logging
			// TODO: Add a GetPerformer method to get name for logging
			log.Infof("Matched face %s to performer (subject: %s, similarity: %.2f)",
				face.FaceID, subject, similarity)
			return performerID, nil
		}

		log.Warnf("Subject %s exists in Compreface but no matching performer found", subject)
		return "", nil
	}

createNewSubject:
	// No match - create new subject and performer
	subjectName := createSubjectName(string(scene.ID), face.FaceID)

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

	performer, err := s.createPerformerWithDetails(subjectName, []string{subjectName}, gender, age)
	if err != nil {
		return "", fmt.Errorf("failed to create performer: %w", err)
	}

	log.Infof("Created new performer %s for unknown face %s (subject: %s, age: %d, gender: %s)",
		performer.Name, face.FaceID, subjectName, age, gender)

	return performer.ID, nil
}

// extractFrameAtTimestamp extracts a frame from a video file at the specified timestamp
// Uses Vision Service's extract-frame endpoint for on-demand frame extraction
func (s *Service) extractFrameAtTimestamp(videoPath string, timestamp float64) ([]byte, error) {
	// Use Vision Service frame extraction endpoint
	// Note: This endpoint is on the Frame Server (port 5001), not the Vision API
	// For now, we'll use a direct HTTP request. In the future, this could be added to the VisionServiceClient

	frameServerURL := "http://localhost:5001" // Frame server port
	url := fmt.Sprintf("%s/extract-frame?video_path=%s&timestamp=%.2f&output_format=jpeg&quality=95",
		frameServerURL, videoPath, timestamp)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to request frame: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("frame extraction failed: status %d", resp.StatusCode)
	}

	frameBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read frame: %w", err)
	}

	return frameBytes, nil
}

// cropFaceFromFrame crops a face region from a frame using the bounding box
func (s *Service) cropFaceFromFrame(frameBytes []byte, bbox vision.VisionBoundingBox) ([]byte, error) {
	// TODO: Implement proper image cropping
	// For now, return the full frame - the Vision Service already provides good face crops
	// In the future, we could use an image processing library to crop precisely
	return frameBytes, nil
}

// createSubjectName creates a unique subject name for Compreface
// Format: "Person {scene_id}_{face_id} {random}"
func createSubjectName(sceneID, faceID string) string {
	return compreface.CreateSubjectName(fmt.Sprintf("%s_%s", sceneID, faceID))
}

// Helper functions for scene GraphQL operations

func findScenes(client *graphql.Client, page, perPage int) ([]stash.Scene, int, error) {
	// TODO: Implement scene query
	// For now, return empty to allow compilation
	return []stash.Scene{}, 0, fmt.Errorf("scene query not yet implemented")
}

func addTagToScene(client *graphql.Client, sceneID graphql.ID, tagID graphql.ID) error {
	// TODO: Implement scene tag mutation
	return fmt.Errorf("scene tag mutation not yet implemented")
}

func updateScenePerformers(client *graphql.Client, sceneID graphql.ID, performerIDs []graphql.ID) error {
	// TODO: Implement scene performer mutation
	return fmt.Errorf("scene performer mutation not yet implemented")
}

func (s *Service) createPerformerWithDetails(name string, aliases []string, gender string, age int) (*stash.Performer, error) {
	// TODO: Implement performer creation with demographics
	// For now, use simple creation
	performerID, err := stash.CreatePerformer(s.graphqlClient, name, aliases)
	if err != nil {
		return nil, err
	}

	return &stash.Performer{
		ID:   performerID,
		Name: name,
	}, nil
}
