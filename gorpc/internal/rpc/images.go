package rpc

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif" // Register GIF format
	"image/jpeg"
	_ "image/png" // Register PNG format
	"os"
	"strings"

	_ "golang.org/x/image/bmp"  // Register BMP format
	_ "golang.org/x/image/webp" // Register WEBP format

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
	"github.com/smegmarip/stash-compreface-plugin/internal/vision"
	"github.com/smegmarip/stash-compreface-plugin/pkg/utils"
)

// ============================================================================
// Image Business Logic (Service Layer)
// ============================================================================

// recognizeImages performs batch face recognition on images using Vision Service
func (s *Service) recognizeImages(limit int) error {
	if s.stopping {
		return fmt.Errorf("operation cancelled")
	}

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

	log.Infof("Starting batch image recognition")

	// Get scanned tag ID for filtering
	scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.ScannedTagName, "Compreface Scanned")
	if err != nil {
		return fmt.Errorf("failed to get scanned tag: %w", err)
	}

	// Get completion tag ID for filtering (exclude already-complete images)
	completeTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.CompleteTagName, "Compreface Complete")
	if err != nil {
		return fmt.Errorf("failed to get complete tag: %w", err)
	}

	batchSize := s.config.MaxBatchSize
	page := 0
	total := 0
	processedCount := 0
	successCount := 0
	failureCount := 0

	for {
		if s.stopping {
			return fmt.Errorf("operation cancelled")
		}

		page++

		// Fetch unscanned images (excluding scanned AND complete)
		tagsFilter := stash.HierarchicalMultiCriterionInput{
			Value:    []string{string(scannedTagID), string(completeTagID)},
			Modifier: stash.CriterionModifierExcludes,
		}
		filter := &stash.ImageFilterType{
			Tags: &tagsFilter,
		}
		images, count, err := stash.FindImages(s.graphqlClient, filter, page, batchSize)
		if err != nil {
			return fmt.Errorf("failed to query images: %w", err)
		}

		if page == 1 {
			total = count

			// Apply limit if specified
			if limit > 0 && limit < total {
				total = limit
				log.Infof("Found %d images, limiting to %d", count, limit)
			} else {
				log.Infof("Found %d images to process", total)
			}
		}

		if len(images) == 0 {
			break
		}

		log.Infof("Processing batch %d: %d images", page, len(images))

		// Process each image in the batch
		for _, img := range images {
			if s.stopping {
				return fmt.Errorf("operation cancelled")
			}

			// Check if limit reached
			if limit > 0 && processedCount >= limit {
				log.Infof("Reached limit of %d images, stopping", limit)
				break
			}

			processedCount++
			progress := float64(processedCount) / float64(total)
			log.Progress(progress)

			log.Infof("Processing image %d/%d: %s", processedCount, total, img.ID)

			err := s.recognizeImageFaces(visionClient, string(img.ID))
			if err != nil {
				log.Warnf("Failed to recognize faces in image %s: %v", img.ID, err)
				failureCount++
			} else {
				successCount++
			}
		}

		// Break outer loop if limit reached
		if limit > 0 && processedCount >= limit {
			break
		}

		// Apply cooldown after processing batch
		if len(images) == batchSize && processedCount < total {
			s.applyCooldown()
		}
	}

	log.Progress(1.0)
	log.Infof("Batch recognition complete: %d processed, %d succeeded, %d failed", processedCount, successCount, failureCount)

	return nil
}

// recognizeImageFaces detects and recognizes faces in an image using Vision Service
func (s *Service) recognizeImageFaces(visionClient *vision.VisionServiceClient, imageID string) error {
	// Step 1: Get image from Stash
	img, err := stash.GetImage(s.graphqlClient, graphql.ID(imageID))
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	if len(img.Files) == 0 {
		return fmt.Errorf("image %s has no files", imageID)
	}

	imagePath := img.Files[0].Path

	// Step 2: Submit to Vision Service for face detection
	results, err := s.SubmitImageJob(visionClient, imagePath, imageID)
	if err != nil {
		return fmt.Errorf("vision service failed: %w", err)
	}

	// Step 3: Add scanned tag regardless of results
	scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.ScannedTagName, "Compreface Scanned")
	if err == nil {
		stash.AddTagToImage(s.graphqlClient, graphql.ID(imageID), scannedTagID)
	}

	// Check if faces were found
	if results.Faces == nil || len(results.Faces.Faces) == 0 {
		log.Debugf("No faces detected in image %s", imageID)
		// Mark as complete (no faces to match)
		s.updateImageCompletionStatus(graphql.ID(imageID), 0, 0)
		return nil
	}

	// Count processable faces
	facesDetected := 0
	for _, face := range results.Faces.Faces {
		det := face.RepresentativeDetection
		qr := s.assessFaceQuality(det.Quality, s.config.MinProcessingQualityScore)
		if qr.Acceptable {
			facesDetected++
		}
	}
	log.Infof("Image %s: Found %d processable faces out of %d total faces", imageID, facesDetected, len(results.Faces.Faces))

	// Step 4: Load image bytes for face cropping
	imageBytes, err := LoadImageBytes(imagePath)
	if err != nil {
		return fmt.Errorf("failed to load image bytes: %w", err)
	}

	// Step 5: Process each face
	requestMetadata := results.Faces.Metadata
	matchedPerformers := []graphql.ID{}
	facesProcessed := 0

	for _, face := range results.Faces.Faces {
		ctx := FaceProcessingContext{
			ImageBytes: imageBytes,
			SourceID:   imageID,
		}
		performerID, err := s.processFace(visionClient, ctx, face, requestMetadata)
		if err != nil {
			log.Warnf("Failed to process face %s: %v", face.FaceID, err)
			continue
		}
		if performerID != "" {
			matchedPerformers = append(matchedPerformers, performerID)
			facesProcessed++
		}
	}

	// Step 6: Update image with matched performers
	if len(matchedPerformers) > 0 {
		log.Infof("Image %s: Matched/created %d performers", imageID, len(matchedPerformers))

		// Get existing performers and merge
		existingPerformerIDs := make([]graphql.ID, len(img.Performers))
		for i, p := range img.Performers {
			existingPerformerIDs[i] = p.ID
		}

		// Merge and deduplicate
		allPerformerIDs := append(existingPerformerIDs, matchedPerformers...)
		allPerformerIDs = utils.DeduplicateIDs(allPerformerIDs)

		var performerIDStrs []string = make([]string, len(allPerformerIDs))
		for i, id := range allPerformerIDs {
			performerIDStrs[i] = string(id)
		}

		input := stash.ImageUpdateInput{
			ID:           imageID,
			PerformerIds: performerIDStrs,
		}
		err = stash.UpdateImage(s.graphqlClient, graphql.ID(imageID), input)
		if err != nil {
			log.Warnf("Failed to update image performers: %v", err)
		}

		// Add matched tag
		matchedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.MatchedTagName, "Compreface Matched")
		if err == nil {
			stash.AddTagToImage(s.graphqlClient, graphql.ID(imageID), matchedTagID)
		}
	}

	// Step 7: Update completion status
	err = s.updateImageCompletionStatus(graphql.ID(imageID), facesDetected, facesProcessed)
	if err != nil {
		log.Warnf("Failed to update completion status: %v", err)
	}

	log.Infof("Image %s: %d subjects processed", imageID, facesProcessed)

	return nil
}

// identifyImage identifies faces in a single image and optionally creates performers
func (s *Service) identifyImage(imageID string, createPerformer bool, faceIndex *int) (*[]FaceIdentity, error) {
	if s.stopping {
		return nil, fmt.Errorf("operation cancelled")
	}

	// Step 1: Get image from Stash
	log.Infof("Fetching image: %s", imageID)
	image, err := stash.GetImage(s.graphqlClient, graphql.ID(imageID))
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	if len(image.Files) == 0 {
		return nil, fmt.Errorf("image %s has no files", imageID)
	}
	imagePath := image.Files[0].Path
	log.Debugf("Image path: %s", imagePath)

	// Step 2: Recognize faces using Compreface
	log.Infof("Recognizing faces in image: %s", imagePath)
	recognitionResp, err := s.comprefaceClient.RecognizeFaces(imagePath)
	identities := &[]FaceIdentity{}
	if err != nil {
		// Check if error is "No face is found" (code 28)
		if strings.Contains(err.Error(), "No face is found") || strings.Contains(err.Error(), "code\" : 28") {
			log.Infof("No faces detected in image %s", imageID)
			// Still add scanned tag
			scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.ScannedTagName, "Compreface Scanned")
			if err == nil {
				stash.AddTagToImage(s.graphqlClient, graphql.ID(imageID), scannedTagID)
			}
			// Mark as complete (no faces to match)
			s.updateImageCompletionStatus(graphql.ID(imageID), 0, 0)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to recognize faces: %w", err)
	}

	if len(recognitionResp.Result) == 0 {
		log.Infof("No faces detected in image %s", imageID)
		// Still add scanned tag
		scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.ScannedTagName, "Compreface Scanned")
		if err == nil {
			stash.AddTagToImage(s.graphqlClient, graphql.ID(imageID), scannedTagID)
		}
		// Mark as complete (no faces to match)
		s.updateImageCompletionStatus(graphql.ID(imageID), 0, 0)
		return nil, nil
	}

	log.Infof("Found %d face(s) in image %s", len(recognitionResp.Result), imageID)

	// Step 3: Process faces (or specific face if faceIndex is provided)
	facesToProcess := recognitionResp.Result
	if faceIndex != nil {
		if *faceIndex >= len(facesToProcess) {
			return nil, fmt.Errorf("face index %d out of range (detected %d faces)", *faceIndex, len(facesToProcess))
		}
		facesToProcess = []compreface.RecognitionResult{facesToProcess[*faceIndex]}
		log.Infof("Processing only face index %d", *faceIndex)
	}

	var performerIDs []graphql.ID
	foundMatch := false

	for i, result := range facesToProcess {
		log.Debugf("Processing face %d/%d", i+1, len(facesToProcess))

		// Check if we have a match above threshold
		// Note: Compreface ALWAYS returns results even for low similarities
		// We must check the similarity score to determine if it's a valid match
		var matchedSubject string
		var matchedSimilarity float64

		if len(result.Subjects) > 0 {
			bestMatch := result.Subjects[0]
			matchedSimilarity = bestMatch.Similarity

			// Only consider it a match if similarity is above threshold
			if bestMatch.Similarity >= s.config.MinSimilarity {
				matchedSubject = bestMatch.Subject
				log.Infof("Face %d: Matched subject '%s' with similarity %.2f",
					i, matchedSubject, matchedSimilarity)
			} else {
				log.Debugf("Face %d: Best match '%s' below threshold (%.2f < %.2f)",
					i, bestMatch.Subject, bestMatch.Similarity, s.config.MinSimilarity)
			}
		} else {
			log.Debugf("Face %d: No subjects returned from Compreface", i)
		}

		// Initialize performer identity record
		performer := PerformerData{
			Age:    int((result.Age.Low + result.Age.High) / 2),
			Gender: result.Gender.Value,
		}

		// Extract base64 face image for identity record
		imageBase64Str, err := s.extractBase64FaceImage(imagePath, result.Box, 20)
		if err != nil {
			imageBase64Str = nil
			log.Warnf("Failed to extract base64 face image for face %d: %v", i, err)
		}

		// Calculate confidence as percentage
		confidence := matchedSimilarity * 100

		// If no match above threshold and createPerformer is true, create new subject/performer
		if matchedSubject == "" {
			// Generate subject name
			subjectName := compreface.CreateSubjectName(imageID)
			performer.Name = subjectName
			if createPerformer {
				log.Infof("Creating new performer for face %d (no match above threshold)", i)

				// Add subject to Compreface with the full image
				log.Debugf("Adding subject '%s' to Compreface", subjectName)
				addResp, err := s.comprefaceClient.AddSubject(subjectName, imagePath)
				if err != nil {
					log.Warnf("Failed to add subject for face %d: %v", i, err)
					continue
				}
				log.Infof("Created Compreface subject '%s' (image_id: %s)", addResp.Subject, addResp.ImageID)

				// Construct Compreface image URL
				imageURL := s.comprefaceClient.SubjectImageURL(addResp.ImageID)
				log.Debugf("Compreface face image URL: %s", imageURL)

				// Create performer in Stash with face image from Compreface
				performerSubject := stash.PerformerSubject{
					Name:    subjectName,
					Aliases: []string{subjectName},
					Age:     performer.Age,
					Image:   imageURL,
					Gender:  performer.Gender,
				}

				performerID, err := stash.CreatePerformerWithImage(s.graphqlClient, performerSubject)
				performerIDStr := string(performerID)
				performer.ID = &performerIDStr
				if err != nil {
					log.Warnf("Failed to create performer for subject '%s': %v", subjectName, err)
					continue
				}

				performerIDs = append(performerIDs, performerID)
				foundMatch = true
				log.Infof("Created performer %s for face %d", performerID, i)
			}
			identity := FaceIdentity{
				ImageID:    imageID,
				Performer:  performer,
				Image:      imageBase64Str,
				Confidence: &confidence,
			}
			*identities = append(*identities, identity)
			continue
		}

		// If we have a matched subject above threshold, find the performer
		if matchedSubject != "" {
			// Find performer by subject name/alias
			performerID, err := stash.FindPerformerBySubjectName(s.graphqlClient, matchedSubject)
			if err != nil {
				log.Warnf("Failed to find performer for subject '%s': %v", matchedSubject, err)
				continue
			}

			if performerID != "" {
				performerIDs = append(performerIDs, performerID)
				foundMatch = true
				log.Infof("Face %d: Associated with performer %s", i, performerID)
				performerIDStr := string(performerID)
				performer.ID = &performerIDStr
				performer.Name = matchedSubject
				identity := FaceIdentity{
					ImageID:    imageID,
					Performer:  performer,
					Image:      imageBase64Str,
					Confidence: &confidence,
				}
				*identities = append(*identities, identity)
			} else {
				log.Warnf("Face %d: Subject '%s' exists in Compreface but no matching performer found in Stash", i, matchedSubject)
			}
		}
	}

	// Step 4: Update image with matched performers
	if len(performerIDs) > 0 {
		log.Infof("Updating image %s with %d performer(s)", imageID, len(performerIDs))

		// Get existing performers and merge
		existingPerformerIDs := make([]graphql.ID, len(image.Performers))
		for i, p := range image.Performers {
			existingPerformerIDs[i] = p.ID
		}

		// Merge and deduplicate
		allPerformerIDs := append(existingPerformerIDs, performerIDs...)
		allPerformerIDs = utils.DeduplicateIDs(allPerformerIDs)

		var performerIDStrs []string = make([]string, len(allPerformerIDs))
		for i, id := range allPerformerIDs {
			performerIDStrs[i] = string(id)
		}

		input := stash.ImageUpdateInput{
			ID: string(imageID),
		}
		if len(performerIDs) > 0 {
			input.PerformerIds = performerIDStrs
		}
		err = stash.UpdateImage(s.graphqlClient, graphql.ID(imageID), input)
		if err != nil {
			log.Warnf("Failed to update image performers: %v", err)
		}
	}

	// Step 5: Add scanned tag
	scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.ScannedTagName, "Compreface Scanned")
	if err == nil {
		stash.AddTagToImage(s.graphqlClient, graphql.ID(imageID), scannedTagID)
	}

	// Step 6: Add matched tag if performers were found
	if foundMatch {
		matchedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.MatchedTagName, "Compreface Matched")
		if err == nil {
			stash.AddTagToImage(s.graphqlClient, graphql.ID(imageID), matchedTagID)
		}
	}

	// Step 7: Update completion status
	facesDetected := len(recognitionResp.Result)
	facesMatched := len(performerIDs)
	err = s.updateImageCompletionStatus(graphql.ID(imageID), facesDetected, facesMatched)
	if err != nil {
		log.Warnf("Failed to update completion status: %v", err)
	}

	log.Infof("Successfully processed image %s (%d performer(s) matched)", imageID, len(performerIDs))
	return identities, nil
}

// identifyGallery processes all images in a gallery
func (s *Service) identifyGallery(galleryID string, createPerformer bool, limit int) error {
	if s.stopping {
		return fmt.Errorf("operation cancelled")
	}

	log.Infof("Starting gallery identification: %s (createPerformer=%v, limit=%d)", galleryID, createPerformer, limit)

	// Step 1: Get gallery info first
	gallery, err := stash.GetGallery(s.graphqlClient, graphql.ID(galleryID))
	if err != nil {
		return fmt.Errorf("failed to get gallery: %w", err)
	}

	if gallery.ImageCount == 0 {
		log.Infof("Gallery %s has no images", galleryID)
		return nil
	}

	page := 1
	totalImages := gallery.ImageCount
	if limit > 0 && limit < totalImages {
		totalImages = limit
	}

	log.Infof("Gallery '%s' has %d images (will process %d)", gallery.Title, gallery.ImageCount, totalImages)

	// Step 2: Query images in gallery using findImages with gallery filter
	// Only images without scanned tag
	galleryFilter := stash.MultiCriterionInput{
		Value:    []string{string(galleryID)},
		Modifier: stash.CriterionModifierIncludes,
	}
	filter := &stash.ImageFilterType{
		Galleries: &galleryFilter,
	}
	images, _, err := stash.FindImages(s.graphqlClient, filter, page, totalImages)
	if err != nil {
		return fmt.Errorf("failed to query gallery images: %w", err)
	}

	if len(images) == 0 {
		log.Infof("Gallery %s has no images to process", galleryID)
		return nil
	}

	log.Infof("Processing %d images from gallery '%s'", len(images), gallery.Title)

	// Step 3: Process each image in the gallery
	successCount := 0
	failureCount := 0

	for i, image := range images {
		if s.stopping {
			return fmt.Errorf("operation cancelled")
		}

		progress := float64(i+1) / float64(len(images))
		log.Progress(progress)

		log.Infof("Processing image %d/%d: %s", i+1, len(images), image.ID)

		_, err := s.identifyImage(string(image.ID), createPerformer, nil)
		if err != nil {
			log.Warnf("Failed to identify image %s: %v", image.ID, err)
			failureCount++
		} else {
			successCount++
		}
	}

	log.Progress(1.0)
	log.Infof("Gallery identification complete: %d succeeded, %d failed", successCount, failureCount)

	return nil
}

// identifyImages performs batch identification of images
func (s *Service) identifyImages(newOnly bool, limit int) error {
	if s.stopping {
		return fmt.Errorf("operation cancelled")
	}

	mode := "all images"
	if newOnly {
		mode = "unscanned images only"
	}
	log.Infof("Starting batch image identification (%s, limit=%d)", mode, limit)

	// Get scanned tag ID for filtering
	scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.ScannedTagName, "Compreface Scanned")
	if err != nil {
		return fmt.Errorf("failed to get scanned tag: %w", err)
	}

	batchSize := s.config.MaxBatchSize
	page := 0
	total := 0
	processedCount := 0
	successCount := 0
	failureCount := 0

	for {
		if s.stopping {
			return fmt.Errorf("operation cancelled")
		}

		page++

		// Build query based on mode
		var filter *stash.ImageFilterType
		if newOnly {
			// Only images without scanned tag
			tagsFilter := stash.HierarchicalMultiCriterionInput{
				Value:    []string{string(scannedTagID)},
				Modifier: stash.CriterionModifierExcludes,
			}
			filter = &stash.ImageFilterType{
				Tags: &tagsFilter,
			}
		}

		images, count, err := stash.FindImages(s.graphqlClient, filter, page, batchSize)
		if err != nil {
			return fmt.Errorf("failed to query images: %w", err)
		}

		if page == 1 {
			total = count

			// Apply limit if specified
			if limit > 0 && limit < total {
				total = limit
				log.Infof("Found %d images, limiting to %d", count, limit)
			} else {
				log.Infof("Found %d images to process", total)
			}
		}

		if len(images) == 0 {
			break
		}

		log.Infof("Processing batch %d: %d images", page, len(images))

		// Process each image in the batch
		for _, image := range images {
			if s.stopping {
				return fmt.Errorf("operation cancelled")
			}

			// Check if limit reached
			if limit > 0 && processedCount >= limit {
				log.Infof("Reached limit of %d images, stopping", limit)
				break
			}

			processedCount++
			progress := float64(processedCount) / float64(total)
			log.Progress(progress)

			log.Infof("Processing image %d/%d: %s", processedCount, total, image.ID)

			_, err := s.identifyImage(string(image.ID), false, nil)
			if err != nil {
				log.Warnf("Failed to identify image %s: %v", image.ID, err)
				failureCount++
			} else {
				successCount++
			}
		}

		// Break outer loop if limit reached
		if limit > 0 && processedCount >= limit {
			break
		}

		// Apply cooldown after processing batch
		if len(images) == batchSize && processedCount < total {
			s.applyCooldown()
		}
	}

	log.Progress(1.0)
	log.Infof("Batch identification complete: %d processed, %d succeeded, %d failed", processedCount, successCount, failureCount)

	return nil
}

// resetUnmatchedImages removes scanned tags from unmatched images
func (s *Service) resetUnmatchedImages(limit int) error {
	if s.stopping {
		return fmt.Errorf("operation cancelled")
	}

	log.Infof("Starting reset of unmatched images (limit=%d)", limit)

	// Step 1: Get tag IDs
	scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.ScannedTagName, "Compreface Scanned")
	if err != nil {
		return fmt.Errorf("failed to get scanned tag: %w", err)
	}

	matchedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.MatchedTagName, "Compreface Matched")
	if err != nil {
		return fmt.Errorf("failed to get matched tag: %w", err)
	}

	var perPage int = -1
	if limit > 0 {
		perPage = limit
	}

	log.Infof("Searching for unmatched images (scanned but not matched)")

	// Step 2: Find images with scanned tag but no matched tag
	tagFilter := stash.HierarchicalMultiCriterionInput{
		Value:    []string{string(scannedTagID)},
		Modifier: stash.CriterionModifierIncludesAll,
		Excludes: []string{string(matchedTagID)},
	}
	input := stash.ImageFilterType{
		Tags: &tagFilter,
	}
	images, count, err := stash.FindImages(s.graphqlClient, &input, 1, perPage)
	if err != nil {
		return fmt.Errorf("failed to query images: %w", err)
	}

	log.Infof("Found %d scanned, unmatched images", count)

	if len(images) == 0 {
		log.Info("No unmatched images found")
		return nil
	}

	total := len(images)

	// Apply limit if specified
	if perPage > 0 && perPage < total {
		log.Infof("Found %d unmatched images out of %d", total, count)
	} else {
		log.Infof("Found %d unmatched images to reset", total)
	}

	// Step 4: Remove scanned tag from unmatched images
	resetCount := 0
	for i, image := range images {
		if s.stopping {
			return fmt.Errorf("operation cancelled")
		}

		imageID := image.ID

		progress := float64(i) / float64(len(images))
		log.Progress(progress)

		err := stash.RemoveTagFromImage(s.graphqlClient, imageID, scannedTagID)
		if err != nil {
			log.Warnf("Failed to remove tag from image %s: %v", imageID, err)
			continue
		}

		resetCount++
		log.Debugf("Reset image %s (%d/%d)", imageID, i+1, len(images))
	}

	log.Progress(1.0)
	log.Infof("Reset complete: %d images processed", resetCount)

	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// updateImageCompletionStatus updates the completion status tag for an image
// based on how many faces were detected vs matched
func (s *Service) updateImageCompletionStatus(imageID graphql.ID, facesDetected int, facesMatched int) error {
	var completionTag string
	var removeTag string

	// Determine completion status
	if facesDetected == 0 {
		// No faces detected - mark as complete (nothing more to match)
		completionTag = s.config.CompleteTagName
		removeTag = s.config.PartialTagName
	} else if facesMatched == facesDetected {
		// All faces matched - complete
		completionTag = s.config.CompleteTagName
		removeTag = s.config.PartialTagName
		log.Infof("Image %s: All %d face(s) matched - marking as Complete", imageID, facesDetected)
	} else {
		// Some faces unmatched - partial (may match in future with new subjects)
		completionTag = s.config.PartialTagName
		removeTag = s.config.CompleteTagName
		log.Infof("Image %s: %d/%d face(s) matched - marking as Partial", imageID, facesMatched, facesDetected)
	}

	// Remove the opposite status tag if it exists
	removeTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, removeTag, removeTag)
	if err == nil {
		// Try to remove, but don't fail if it doesn't exist
		stash.RemoveTagFromImage(s.graphqlClient, imageID, removeTagID)
	}

	// Add the appropriate completion tag
	completionTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, completionTag, completionTag)
	if err != nil {
		return fmt.Errorf("failed to get/create completion tag: %w", err)
	}

	err = stash.AddTagToImage(s.graphqlClient, imageID, completionTagID)
	if err != nil {
		return fmt.Errorf("failed to add completion tag: %w", err)
	}

	log.Debugf("Updated image %s with completion status: %s", imageID, completionTag)
	return nil
}

// convertToJPEG opens an image from disk and ensures it’s in JPEG format.
func (s *Service) convertToJPEG(imagePath string) (image.Image, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// extractBoxImage crops a region from the image with optional padding.
func (s *Service) extractBoxImage(img image.Image, box compreface.BoundingBox, padding int) (image.Image, error) {
	bounds := img.Bounds()

	xMin := utils.Max(bounds.Min.X, box.XMin-padding)
	yMin := utils.Max(bounds.Min.Y, box.YMin-padding)
	xMax := utils.Min(bounds.Max.X, box.XMax+padding)
	yMax := utils.Min(bounds.Max.Y, box.YMax+padding)

	rect := image.Rect(xMin, yMin, xMax, yMax)
	cropped := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(rect)

	return cropped, nil
}

// imageToBase64 encodes the image to JPEG and Base64.
func (s *Service) convertImageToBase64(img image.Image) (string, error) {
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// extractBase64FaceImage extracts a face image from the given image path and bounding box,
func (s *Service) extractBase64FaceImage(imagePath string, box compreface.BoundingBox, padding int) (*string, error) {
	img, err := s.convertToJPEG(imagePath)
	if err != nil {
		return nil, err
	}

	cropped, err := s.extractBoxImage(img, box, padding)
	if err != nil {
		return nil, err
	}

	base64Str, err := s.convertImageToBase64(cropped)
	if err != nil {
		return nil, err
	}

	return &base64Str, nil
}
