package rpc

import (
	"fmt"
	"strings"

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
func (s *Service) synchronizePerformers(limit int) error {
	if s.stopping {
		return fmt.Errorf("operation cancelled")
	}

	log.Info("Starting performer synchronization with Compreface")

	// Get sync tag to track which performers have been processed
	syncedTagName := s.config.SyncedTagName
	syncTagID, err := stash.GetOrCreateTag(s.graphqlClient, s.tagCache, syncedTagName, "Compreface Synced")
	if err != nil {
		return fmt.Errorf("failed to get sync tag: %w", err)
	}

	batchSize := s.config.MaxBatchSize
	page := 0
	total := 0
	processedCount := 0

	for {
		if s.stopping {
			return fmt.Errorf("operation cancelled")
		}

		page++

		subjectCriterion := stash.StringCriterionInput{
			Value:    "Person ",
			Modifier: stash.CriterionModifierIncludes,
		}
		tagsFilter := stash.HierarchicalMultiCriterionInput{
			Value:    []string{string(syncTagID)},
			Modifier: stash.CriterionModifierExcludes,
		}

		// Fetch performers with images that haven't been synced yet
		filter := &stash.PerformerFilterType{
			Tags: &tagsFilter,
			OperatorFilter: stash.OperatorFilter[stash.PerformerFilterType]{
				And: &stash.PerformerFilterType{
					OperatorFilter: stash.OperatorFilter[stash.PerformerFilterType]{
						Or: &stash.PerformerFilterType{
							Name: &subjectCriterion,
							OperatorFilter: stash.OperatorFilter[stash.PerformerFilterType]{
								Or: &stash.PerformerFilterType{
									Aliases: &subjectCriterion,
								},
							},
						},
					},
				},
			},
		}

		unfiltered, count, err := stash.FindPerformers(s.graphqlClient, filter, page, batchSize)
		if err != nil {
			return fmt.Errorf("failed to query performers: %w", err)
		}

		performers := []stash.Performer{}
		// Filter out performers without images
		for _, performer := range unfiltered {
			if performer.ImagePath != "" && !strings.Contains(performer.ImagePath, "default=true") {
				performers = append(performers, performer)
			}
		}

		if page == 1 {
			total = count

			// Apply limit if specified
			if limit > 0 && limit < total {
				total = limit
				log.Infof("Found %d performers, limiting to %d", count, limit)
			} else {
				log.Infof("Found %d performers to sync", total)
			}
		}

		if len(unfiltered) == 0 {
			break
		}

		log.Infof("Processing batch %d: %d performers", page, len(unfiltered))

		// Process each performer in the batch
		for _, performer := range performers {
			if s.stopping {
				return fmt.Errorf("operation cancelled")
			}

			// Check if limit reached
			if limit > 0 && processedCount >= limit {
				log.Infof("Reached limit of %d performers, stopping", limit)
				break
			}

			processedCount++
			progress := float64(processedCount) / float64(total)
			log.Progress(progress)

			log.Infof("Processing performer %d/%d: %s (ID: %s)", processedCount, total, performer.Name, performer.ID)

			err := s.syncPerformer(performer, syncTagID)
			if err != nil {
				log.Warnf("Failed to sync performer %s: %v", performer.ID, err)
				// Continue with next performer
				continue
			}
		}

		// Break outer loop if limit reached
		if limit > 0 && processedCount >= limit {
			break
		}

		// Apply cooldown after processing batch
		if len(performers) == batchSize && processedCount < total {
			s.applyCooldown()
		}
	}

	log.Progress(1.0)
	log.Infof("Performer synchronization complete: %d performers processed", processedCount)

	return nil
}

// syncPerformer syncs a single performer with Compreface
func (s *Service) syncPerformer(performer stash.Performer, syncTagID graphql.ID) error {
	// Step 1: Find or create the "Person ..." alias
	alias := compreface.FindPersonAlias(&performer)
	createdAlias := false
	if alias == "" {
		// No alias found - create one
		alias = compreface.CreateSubjectName(string(performer.ID))
		log.Infof("No 'Person ...' alias found for performer %s, creating new alias: %s", performer.Name, alias)
		createdAlias = true
	} else {
		log.Infof("Found existing alias '%s' for performer %s", alias, performer.Name)
	}

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

	// Step 3: Get performer image URL and download image bytes
	// Performer images are stored as blobs in Stash, accessible via /performer/{id}/image endpoint
	imageURL := fmt.Sprintf("%s://%s:%d/performer/%s/image",
		s.serverConnection.Scheme,
		s.serverConnection.Host,
		s.serverConnection.Port,
		performer.ID)

	log.Debugf("Downloading performer image from %s", imageURL)
	imageBytes, err := stash.DownloadImage(imageURL, s.serverConnection.SessionCookie)
	if err != nil {
		log.Warnf("Failed to download performer %s image: %v", performer.Name, err)
		return stash.AddTagToPerformer(s.graphqlClient, performer.ID, syncTagID)
	}

	if len(imageBytes) == 0 {
		log.Warnf("Performer %s image is empty", performer.Name)
		return stash.AddTagToPerformer(s.graphqlClient, performer.ID, syncTagID)
	}

	log.Debugf("Downloaded %d bytes for performer %s", len(imageBytes), performer.Name)

	// Step 4: Add subject to Compreface with alias using image bytes
	log.Infof("Adding subject '%s' to Compreface", alias)
	addResp, err := s.comprefaceClient.AddSubjectFromBytes(alias, imageBytes, fmt.Sprintf("performer_%s.jpg", performer.ID))
	if err != nil {
		return fmt.Errorf("failed to add subject: %w", err)
	}

	log.Infof("Successfully added subject '%s' to Compreface (image_id: %s)", addResp.Subject, addResp.ImageID)

	// Step 6: If we created a new alias, add it to the performer
	if createdAlias {
		// Get current aliases
		currentAliases := performer.AliasList
		if currentAliases == nil {
			currentAliases = []string{}
		}

		// Add new alias
		newAliases := append(currentAliases, alias)

		input := stash.PerformerUpdateInput{
			ID:        string(performer.ID),
			AliasList: newAliases,
		}

		// Update performer with new alias list (pass nil for name and tagIDs to only update aliases)
		err = stash.UpdatePerformer(s.graphqlClient, performer.ID, input)
		if err != nil {
			return fmt.Errorf("failed to add alias to performer: %w", err)
		}

		log.Infof("Added alias '%s' to performer %s", alias, performer.Name)
	}

	// Step 7: Add sync tag to performer
	err = stash.AddTagToPerformer(s.graphqlClient, performer.ID, syncTagID)
	if err != nil {
		return fmt.Errorf("failed to add sync tag to performer: %w", err)
	}

	return nil
}
