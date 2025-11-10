package stash

import (
	"context"
	"fmt"
	"strings"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// ============================================================================
// Image Data Operations (Repository Layer)
// ============================================================================

// FindImages finds images with optional filtering
func FindImages(client *graphql.Client, filter map[string]interface{}, page int, perPage int) ([]Image, int, error) {
	var query struct {
		FindImages struct {
			Count  int
			Images []Image
		} `graphql:"findImages(filter: $page_filter)"`
	}

	pageInt := graphql.Int(page)
	perPageInt := graphql.Int(perPage)
	pageFilter := &FindFilterType{
		Page:    &pageInt,
		PerPage: &perPageInt,
	}

	variables := map[string]interface{}{
		"page_filter": pageFilter,
	}

	err := client.Query(context.Background(), &query, variables)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query images: %w", err)
	}

	log.Debugf("Found %d images (page %d, per_page %d)", len(query.FindImages.Images), page, perPage)
	return query.FindImages.Images, query.FindImages.Count, nil
}

// GetImage retrieves a single image by ID
func GetImage(client *graphql.Client, imageID graphql.ID) (*Image, error) {
	var query struct {
		FindImage Image `graphql:"findImage(id: $id)"`
	}

	variables := map[string]interface{}{
		"id": imageID,
	}

	err := client.Query(context.Background(), &query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to query image: %w", err)
	}

	return &query.FindImage, nil
}

// UpdateImage updates image tags and performers
func UpdateImage(client *graphql.Client, imageID graphql.ID, tagIDs []graphql.ID, performerIDs []graphql.ID) error {
	var mutation struct {
		ImageUpdate struct {
			ID graphql.ID
		} `graphql:"imageUpdate(input: $input)"`
	}

	input := &ImageUpdateInput{
		ID: imageID,
	}

	if tagIDs != nil {
		input.TagIds = tagIDs
	}
	if performerIDs != nil {
		input.PerformerIds = performerIDs
	}

	variables := map[string]interface{}{
		"input": input,
	}

	err := client.Mutate(context.Background(), &mutation, variables)
	if err != nil {
		return fmt.Errorf("failed to update image: %w", err)
	}

	log.Debugf("Updated image %s", imageID)
	return nil
}

// AddTagToImage adds a tag to an image
func AddTagToImage(client *graphql.Client, imageID graphql.ID, tagID graphql.ID) error {
	ctx := context.Background()

	// First get current tags
	image, err := GetImage(client, imageID)
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	// Build tag ID list (existing + new)
	tagIDs := []graphql.ID{}
	hasTag := false
	for _, tag := range image.Tags {
		tagIDs = append(tagIDs, tag.ID)
		if tag.ID == tagID {
			hasTag = true
		}
	}

	// If already has tag, nothing to do
	if hasTag {
		log.Tracef("Image %s already has tag %s", imageID, tagID)
		return nil
	}

	tagIDs = append(tagIDs, tagID)

	// Build tag_ids array as JSON
	tagIDStrs := make([]string, len(tagIDs))
	for i, id := range tagIDs {
		tagIDStrs[i] = fmt.Sprintf("\"%s\"", id)
	}
	tagIDsJSON := "[" + strings.Join(tagIDStrs, ",") + "]"

	// Use ExecRaw to update image
	query := fmt.Sprintf(`mutation {
		imageUpdate(input: {id: "%s", tag_ids: %s}) {
			id
		}
	}`, imageID, tagIDsJSON)

	_, err = client.ExecRaw(ctx, query, nil)
	if err != nil {
		return fmt.Errorf("failed to add tag to image: %w", err)
	}

	log.Tracef("Added tag %s to image %s", tagID, imageID)
	return nil
}

// RemoveTagFromImage removes a tag from an image
func RemoveTagFromImage(client *graphql.Client, imageID graphql.ID, tagID graphql.ID) error {
	ctx := context.Background()

	// Get current tags
	image, err := GetImage(client, imageID)
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	// Filter out the tag to remove
	tagIDs := []graphql.ID{}
	for _, tag := range image.Tags {
		if tag.ID != tagID {
			tagIDs = append(tagIDs, tag.ID)
		}
	}

	// Build tag_ids array as JSON
	tagIDsJSON := "[]"
	if len(tagIDs) > 0 {
		tagIDStrs := make([]string, len(tagIDs))
		for i, id := range tagIDs {
			tagIDStrs[i] = fmt.Sprintf("\"%s\"", id)
		}
		tagIDsJSON = "[" + strings.Join(tagIDStrs, ",") + "]"
	}

	// Use ExecRaw to update image
	query := fmt.Sprintf(`mutation {
		imageUpdate(input: {id: "%s", tag_ids: %s}) {
			id
		}
	}`, imageID, tagIDsJSON)

	_, err = client.ExecRaw(ctx, query, nil)
	if err != nil {
		return fmt.Errorf("failed to remove tag from image: %w", err)
	}

	log.Tracef("Removed tag %s from image %s", tagID, imageID)
	return nil
}
