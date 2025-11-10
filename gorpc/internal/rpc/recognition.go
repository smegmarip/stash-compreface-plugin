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

// recognizeImages performs batch face recognition on images
func (s *Service) recognizeImages(lowQuality bool) error {
	if s.stopping {
		return fmt.Errorf("operation cancelled")
	}

	mode := "high quality"
	if lowQuality {
		mode = "low quality"
	}
	log.Infof("Starting batch image recognition (%s mode)", mode)

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

		// Fetch unscanned images
		ctx := context.Background()
		query := fmt.Sprintf(`query {
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

			err := s.recognizeImageFaces(string(image.ID), lowQuality)
			if err != nil {
				log.Warnf("Failed to recognize faces in image %s: %v", image.ID, err)
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
	log.Infof("Batch recognition complete: %d processed, %d succeeded, %d failed", processedCount, successCount, failureCount)

	return nil
}

// recognizeImageFaces detects and recognizes faces in an image, creating subjects as needed
func (s *Service) recognizeImageFaces(imageID string, lowQuality bool) error {
	// Step 1: Get image from Stash
	image, err := stash.GetImage(s.graphqlClient, graphql.ID(imageID))
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	if len(image.Files) == 0 {
		return fmt.Errorf("image %s has no files", imageID)
	}

	imagePath := image.Files[0].Path

	// Step 2: Recognize faces using Compreface
	recognitionResp, err := s.comprefaceClient.RecognizeFaces(imagePath)
	if err != nil {
		return fmt.Errorf("failed to recognize faces: %w", err)
	}

	// Step 3: Add scanned tag regardless of results
	scannedTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, s.config.ScannedTagName, "Compreface Scanned")
	if err == nil {
		stash.AddTagToImage(s.graphqlClient, graphql.ID(imageID), scannedTagID)
	}

	if len(recognitionResp.Result) == 0 {
		log.Debugf("No faces detected in image %s", imageID)
		return nil
	}

	log.Infof("Detected %d face(s) in image %s", len(recognitionResp.Result), imageID)

	// Step 4: Process each detected face
	createdSubjects := 0
	matchedSubjects := 0

	for i, result := range recognitionResp.Result {
		log.Debugf("Processing face %d/%d", i+1, len(recognitionResp.Result))

		if len(result.Subjects) > 0 {
			// Face matched existing subject
			bestMatch := result.Subjects[0]
			if bestMatch.Similarity >= s.config.MinSimilarity {
				log.Infof("Face %d matched subject '%s' with similarity %.2f", i, bestMatch.Subject, bestMatch.Similarity)
				matchedSubjects++
			} else {
				log.Debugf("Face %d: Best match '%s' below threshold (%.2f < %.2f)", i, bestMatch.Subject, bestMatch.Similarity, s.config.MinSimilarity)
			}
		} else {
			// No match - create new subject
			// Check face dimensions
			if !utils.IsFaceSizeValid(result.Box, s.config.MinFaceSize) {
				width, height := utils.GetFaceDimensions(result.Box)
				minDim := width
				if height < minDim {
					minDim = height
				}
				log.Debugf("Face %d too small (min dimension: %d < %d), skipping", i, minDim, s.config.MinFaceSize)
				continue
			}

			// Generate subject name
			subjectName := compreface.CreateSubjectName(imageID)

			// Add subject to Compreface with the full image
			// Compreface will detect and extract the face automatically
			log.Infof("Creating new subject '%s' for face %d", subjectName, i)
			addResp, err := s.comprefaceClient.AddSubject(subjectName, imagePath)
			if err != nil {
				log.Warnf("Failed to add subject for face %d: %v", i, err)
				continue
			}

			log.Infof("Created subject '%s' (image_id: %s)", addResp.Subject, addResp.ImageID)
			createdSubjects++

			// TODO: Create performer in Stash and link to subject
		}
	}

	log.Infof("Image %s: %d subjects created, %d existing subjects matched", imageID, createdSubjects, matchedSubjects)

	return nil
}
