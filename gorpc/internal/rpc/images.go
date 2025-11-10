package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
	"github.com/smegmarip/stash-compreface-plugin/pkg/utils"
)

// ============================================================================
// Image Business Logic (Service Layer)
// ============================================================================

// identifyImage identifies faces in a single image and optionally creates performers
func (s *Service) identifyImage(imageID string, createPerformer bool, faceIndex *int) error {
	if s.stopping {
		return fmt.Errorf("operation cancelled")
	}

	// Step 1: Get image from Stash
	log.Infof("Fetching image: %s", imageID)
	image, err := stash.GetImage(s.graphqlClient, graphql.ID(imageID))
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	if len(image.Files) == 0 {
		return fmt.Errorf("image %s has no files", imageID)
	}

	imagePath := image.Files[0].Path
	log.Debugf("Image path: %s", imagePath)

	// Step 2: Recognize faces using Compreface
	log.Infof("Recognizing faces in image: %s", imagePath)
	recognitionResp, err := s.comprefaceClient.RecognizeFaces(imagePath)
	if err != nil {
		return fmt.Errorf("failed to recognize faces: %w", err)
	}

	if len(recognitionResp.Result) == 0 {
		log.Infof("No faces detected in image %s", imageID)
		// Still add scanned tag
		scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.ScannedTagName, "Compreface Scanned")
		if err == nil {
			stash.AddTagToImage(s.graphqlClient, graphql.ID(imageID), scannedTagID)
		}
		return nil
	}

	log.Infof("Found %d face(s) in image %s", len(recognitionResp.Result), imageID)

	// Step 3: Process faces (or specific face if faceIndex is provided)
	facesToProcess := recognitionResp.Result
	if faceIndex != nil {
		if *faceIndex >= len(facesToProcess) {
			return fmt.Errorf("face index %d out of range (detected %d faces)", *faceIndex, len(facesToProcess))
		}
		facesToProcess = []compreface.RecognitionResult{facesToProcess[*faceIndex]}
		log.Infof("Processing only face index %d", *faceIndex)
	}

	var performerIDs []graphql.ID
	foundMatch := false

	for i, result := range facesToProcess {
		log.Debugf("Processing face %d/%d", i+1, len(facesToProcess))

		if len(result.Subjects) == 0 {
			log.Debugf("Face %d: No subjects matched", i)
			if createPerformer {
				log.Infof("Creating new performer for unmatched face %d", i)
				// TODO: Implement performer creation
			}
			continue
		}

		// Find best match above similarity threshold
		bestMatch := result.Subjects[0]
		if bestMatch.Similarity < s.config.MinSimilarity {
			log.Debugf("Face %d: Best match '%s' below threshold (%.2f < %.2f)",
				i, bestMatch.Subject, bestMatch.Similarity, s.config.MinSimilarity)
			if createPerformer {
				log.Infof("Creating new performer for low-similarity face %d", i)
				// TODO: Implement performer creation
			}
			continue
		}

		log.Infof("Face %d: Matched subject '%s' with similarity %.2f",
			i, bestMatch.Subject, bestMatch.Similarity)

		// Find performer by subject name/alias
		performerID, err := stash.FindPerformerBySubjectName(s.graphqlClient, bestMatch.Subject)
		if err != nil {
			log.Warnf("Failed to find performer for subject '%s': %v", bestMatch.Subject, err)
			continue
		}

		if performerID != "" {
			performerIDs = append(performerIDs, performerID)
			foundMatch = true
			log.Infof("Face %d: Associated with performer %s", i, performerID)
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

		err = stash.UpdateImage(s.graphqlClient, graphql.ID(imageID), nil, allPerformerIDs)
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

	log.Infof("Successfully processed image %s (%d performer(s) matched)", imageID, len(performerIDs))
	return nil
}

// identifyGallery processes all images in a gallery
func (s *Service) identifyGallery(galleryID string, createPerformer bool) error {
	if s.stopping {
		return fmt.Errorf("operation cancelled")
	}

	log.Infof("Starting gallery identification: %s (createPerformer=%v)", galleryID, createPerformer)

	// Step 1: Get gallery and its images
	ctx := context.Background()
	query := fmt.Sprintf(`query {
		findGallery(id: "%s") {
			id
			title
			images {
				id
			}
		}
	}`, galleryID)

	data, err := s.graphqlClient.ExecRaw(ctx, query, nil)
	if err != nil {
		return fmt.Errorf("failed to query gallery: %w", err)
	}

	// Parse response
	var response struct {
		FindGallery struct {
			ID     graphql.ID `json:"id"`
			Title  string     `json:"title"`
			Images []struct {
				ID graphql.ID `json:"id"`
			} `json:"images"`
		} `json:"findGallery"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to parse query response: %w", err)
	}

	gallery := response.FindGallery
	if len(gallery.Images) == 0 {
		log.Infof("Gallery %s has no images", galleryID)
		return nil
	}

	log.Infof("Gallery '%s' has %d images", gallery.Title, len(gallery.Images))

	// Step 2: Process each image in the gallery
	successCount := 0
	failureCount := 0

	for i, image := range gallery.Images {
		if s.stopping {
			return fmt.Errorf("operation cancelled")
		}

		progress := float64(i+1) / float64(len(gallery.Images))
		log.Progress(progress)

		log.Infof("Processing image %d/%d: %s", i+1, len(gallery.Images), image.ID)

		err := s.identifyImage(string(image.ID), createPerformer, nil)
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
func (s *Service) identifyImages(newOnly bool) error {
	if s.stopping {
		return fmt.Errorf("operation cancelled")
	}

	mode := "all images"
	if newOnly {
		mode = "unscanned images only"
	}
	log.Infof("Starting batch image identification (%s)", mode)

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
		var query string
		ctx := context.Background()

		if newOnly {
			// Only images without scanned tag
			query = fmt.Sprintf(`query {
				findImages(
					image_filter: {
						tags: {
							value: ["%s"],
							modifier: EXCLUDES
						}
					},
					filter: {per_page: %d, page: %d}
				) {
					count
					images {
						id
					}
				}
			}`, scannedTagID, batchSize, page)
		} else {
			// All images
			query = fmt.Sprintf(`query {
				findImages(
					filter: {per_page: %d, page: %d}
				) {
					count
					images {
						id
					}
				}
			}`, batchSize, page)
		}

		data, err := s.graphqlClient.ExecRaw(ctx, query, nil)
		if err != nil {
			return fmt.Errorf("failed to query images: %w", err)
		}

		// Parse response
		var response struct {
			FindImages struct {
				Count  int `json:"count"`
				Images []struct {
					ID graphql.ID `json:"id"`
				} `json:"images"`
			} `json:"findImages"`
		}

		if err := json.Unmarshal(data, &response); err != nil {
			return fmt.Errorf("failed to parse query response: %w", err)
		}

		if page == 1 {
			total = response.FindImages.Count
			log.Infof("Found %d images to process", total)
		}

		images := response.FindImages.Images
		if len(images) == 0 {
			break
		}

		log.Infof("Processing batch %d: %d images", page, len(images))

		// Process each image in the batch
		for _, image := range images {
			if s.stopping {
				return fmt.Errorf("operation cancelled")
			}

			processedCount++
			progress := float64(processedCount) / float64(total)
			log.Progress(progress)

			log.Infof("Processing image %d/%d: %s", processedCount, total, image.ID)

			err := s.identifyImage(string(image.ID), false, nil)
			if err != nil {
				log.Warnf("Failed to identify image %s: %v", image.ID, err)
				failureCount++
			} else {
				successCount++
			}
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
func (s *Service) resetUnmatchedImages() error {
	if s.stopping {
		return fmt.Errorf("operation cancelled")
	}

	// Step 1: Get tag IDs
	scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.ScannedTagName, "Compreface Scanned")
	if err != nil {
		return fmt.Errorf("failed to get scanned tag: %w", err)
	}

	matchedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.MatchedTagName, "Compreface Matched")
	if err != nil {
		return fmt.Errorf("failed to get matched tag: %w", err)
	}

	log.Infof("Searching for unmatched images (scanned but not matched)")

	// Step 2: Find images with scanned tag but no matched tag
	// We'll use GraphQL directly since we need tag filtering
	ctx := context.Background()
	query := fmt.Sprintf(`query {
		findImages(
			image_filter: {
				tags: {
					value: ["%s"],
					modifier: INCLUDES_ALL
				}
			},
			filter: {per_page: -1}
		) {
			count
			images {
				id
				tags {
					id
				}
			}
		}
	}`, scannedTagID)

	data, err := s.graphqlClient.ExecRaw(ctx, query, nil)
	if err != nil {
		return fmt.Errorf("failed to query images: %w", err)
	}

	// Parse response
	var response struct {
		FindImages struct {
			Count  int `json:"count"`
			Images []struct {
				ID   graphql.ID `json:"id"`
				Tags []struct {
					ID graphql.ID `json:"id"`
				} `json:"tags"`
			} `json:"images"`
		} `json:"findImages"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to parse query response: %w", err)
	}

	log.Infof("Found %d scanned images", response.FindImages.Count)

	// Step 3: Filter for images without matched tag
	var unmatchedImages []graphql.ID
	for _, image := range response.FindImages.Images {
		hasMatchedTag := false
		for _, tag := range image.Tags {
			if tag.ID == matchedTagID {
				hasMatchedTag = true
				break
			}
		}

		if !hasMatchedTag {
			unmatchedImages = append(unmatchedImages, image.ID)
		}
	}

	if len(unmatchedImages) == 0 {
		log.Info("No unmatched images found")
		return nil
	}

	log.Infof("Found %d unmatched images to reset", len(unmatchedImages))

	// Step 4: Remove scanned tag from unmatched images
	resetCount := 0
	for i, imageID := range unmatchedImages {
		if s.stopping {
			return fmt.Errorf("operation cancelled")
		}

		progress := float64(i) / float64(len(unmatchedImages))
		log.Progress(progress)

		err := stash.RemoveTagFromImage(s.graphqlClient, imageID, scannedTagID)
		if err != nil {
			log.Warnf("Failed to remove tag from image %s: %v", imageID, err)
			continue
		}

		resetCount++
		log.Debugf("Reset image %s (%d/%d)", imageID, i+1, len(unmatchedImages))
	}

	log.Progress(1.0)
	log.Infof("Reset complete: %d images processed", resetCount)

	return nil
}
