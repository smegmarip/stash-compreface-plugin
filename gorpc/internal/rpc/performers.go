package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
)

// ============================================================================
// Performer Business Logic (Service Layer)
// ============================================================================

// synchronizePerformers syncs performers with Compreface subjects
// It finds performers with "Person ..." aliases and adds their images to Compreface
func (s *Service) synchronizePerformers() error {
	if s.stopping {
		return fmt.Errorf("operation cancelled")
	}

	log.Info("Starting performer synchronization with Compreface")

	// Get sync tag to track which performers have been processed
	syncTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, "Compreface Synced", "Compreface Synced")
	if err != nil {
		return fmt.Errorf("failed to get sync tag: %w", err)
	}

	batchSize := s.config.MaxBatchSize
	page := 0
	total := 0

	for {
		if s.stopping {
			return fmt.Errorf("operation cancelled")
		}

		page++

		// Fetch performers with images that haven't been synced yet
		// We use ExecRaw since we need complex filtering
		ctx := context.Background()
		query := fmt.Sprintf(`query {
			findPerformers(
				performer_filter: {
					tags: {
						value: ["%s"],
						modifier: EXCLUDES
					},
					is_missing: "image"
				},
				filter: {per_page: %d, page: %d}
			) {
				count
				performers {
					id
					name
					alias_list
					image_path
					tags {
						id
					}
				}
			}
		}`, syncTagID, batchSize, page)

		data, err := s.graphqlClient.ExecRaw(ctx, query, nil)
		if err != nil {
			return fmt.Errorf("failed to query performers: %w", err)
		}

		// Parse response
		var response struct {
			FindPerformers struct {
				Count      int               `json:"count"`
				Performers []stash.Performer `json:"performers"`
			} `json:"findPerformers"`
		}

		if err := json.Unmarshal(data, &response); err != nil {
			return fmt.Errorf("failed to parse query response: %w", err)
		}

		if page == 1 {
			total = response.FindPerformers.Count
			log.Infof("Found %d performers to sync", total)
		}

		performers := response.FindPerformers.Performers
		if len(performers) == 0 {
			break
		}

		log.Infof("Processing batch %d: %d performers", page, len(performers))

		// Process each performer in the batch
		for i, performer := range performers {
			if s.stopping {
				return fmt.Errorf("operation cancelled")
			}

			current := (page-1)*batchSize + i + 1
			progress := float64(current) / float64(total)
			log.Progress(progress)

			log.Infof("Processing performer %d/%d: %s (ID: %s)", current, total, performer.Name, performer.ID)

			err := s.syncPerformer(performer, syncTagID)
			if err != nil {
				log.Warnf("Failed to sync performer %s: %v", performer.ID, err)
				// Continue with next performer
				continue
			}
		}

		// Apply cooldown after processing batch
		if len(performers) == batchSize {
			s.applyCooldown()
		}
	}

	log.Progress(1.0)
	log.Infof("Performer synchronization complete: %d performers processed", total)

	return nil
}

// syncPerformer syncs a single performer with Compreface
func (s *Service) syncPerformer(performer stash.Performer, syncTagID graphql.ID) error {
	// Step 1: Find the "Person ..." alias
	alias := compreface.FindPersonAlias(&performer)
	if alias == "" {
		log.Debugf("No 'Person ...' alias found for performer %s, skipping", performer.Name)
		// Still add sync tag to mark as processed
		return stash.AddTagToPerformer(s.graphqlClient, performer.ID, syncTagID)
	}

	log.Infof("Found alias '%s' for performer %s", alias, performer.Name)

	// Step 2: Check if subject already exists in Compreface
	subjects, err := s.comprefaceClient.ListSubjects()
	if err != nil {
		return fmt.Errorf("failed to list subjects: %w", err)
	}

	subjectExists := false
	for _, subject := range subjects {
		if subject == alias {
			subjectExists = true
			break
		}
	}

	if subjectExists {
		log.Infof("Subject '%s' already exists in Compreface", alias)
		// Add sync tag and return
		return stash.AddTagToPerformer(s.graphqlClient, performer.ID, syncTagID)
	}

	// Step 3: Download performer image
	if performer.ImagePath == "" {
		log.Warnf("Performer %s has no image path", performer.Name)
		return stash.AddTagToPerformer(s.graphqlClient, performer.ID, syncTagID)
	}

	log.Debugf("Downloading performer image from %s", performer.ImagePath)
	imagePath := performer.ImagePath

	// Step 4: Detect faces in performer image
	recognitionResp, err := s.comprefaceClient.RecognizeFaces(imagePath)
	if err != nil {
		return fmt.Errorf("failed to recognize faces in performer image: %w", err)
	}

	if len(recognitionResp.Result) == 0 {
		log.Warnf("No faces detected in performer %s image", performer.Name)
		return stash.AddTagToPerformer(s.graphqlClient, performer.ID, syncTagID)
	}

	// Use the first detected face
	faceResult := recognitionResp.Result[0]
	log.Infof("Detected face in performer image (box: %+v)", faceResult.Box)

	// Step 5: Add subject to Compreface with alias
	// Pass the full image - Compreface will detect and extract the face
	log.Infof("Adding subject '%s' to Compreface", alias)
	addResp, err := s.comprefaceClient.AddSubject(alias, imagePath)
	if err != nil {
		return fmt.Errorf("failed to add subject: %w", err)
	}

	log.Infof("Successfully added subject '%s' to Compreface (image_id: %s)", addResp.Subject, addResp.ImageID)

	// Step 7: Add sync tag to performer
	err = stash.AddTagToPerformer(s.graphqlClient, performer.ID, syncTagID)
	if err != nil {
		return fmt.Errorf("failed to add sync tag to performer: %w", err)
	}

	return nil
}
