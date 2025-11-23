package stash

import (
	"context"
	"fmt"
	"io"
	"net/http"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// ============================================================================
// Image Data Operations (Repository Layer)
// ============================================================================

// FindImages finds images with optional filtering
func FindImages(client *graphql.Client, filter *ImageFilterType, page int, perPage int) ([]Image, int, error) {
	var query struct {
		FindImages struct {
			Count  int
			Images []Image
		} `graphql:"findImages(filter: $filter, image_filter: $image_filter)"`
	}

	pageInt := int(page)
	perPageInt := int(perPage)
	filterInput := &FindFilterType{
		Page:    &pageInt,
		PerPage: &perPageInt,
	}

	variables := map[string]interface{}{
		"filter": filterInput,
	}

	if filter != nil {
		variables["image_filter"] = filter
	} else {
		variables["image_filter"] = ImageFilterType{}
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
func UpdateImage(client *graphql.Client, imageID graphql.ID, input ImageUpdateInput) error {
	ctx := context.Background()

	var mutation struct {
		ImageUpdate ImageUpdate `graphql:"imageUpdate(input: $input)"`
	}

	variables := map[string]interface{}{
		"input": input,
	}

	err := client.Mutate(ctx, &mutation, variables)
	if err != nil {
		return fmt.Errorf("failed to update image: %w", err)
	}

	log.Debugf("Updated image %s", imageID)
	return nil
}

// AddTagToImage adds a tag to an image
func AddTagToImage(client *graphql.Client, imageID graphql.ID, tagID graphql.ID) error {
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
		tagIDStrs[i] = string(id)
	}

	input := ImageUpdateInput{
		ID:     string(imageID),
		TagIds: tagIDStrs,
	}

	err = UpdateImage(client, imageID, input)
	if err != nil {
		return fmt.Errorf("failed to add tag to image: %w", err)
	}

	log.Tracef("Added tag %s to image %s", tagID, imageID)
	return nil
}

// RemoveTagFromImage removes a tag from an image
func RemoveTagFromImage(client *graphql.Client, imageID graphql.ID, tagID graphql.ID) error {
	// Get current tags
	image, err := GetImage(client, imageID)
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	// Filter out the tag to remove
	tagIDs := []string{}
	for _, tag := range image.Tags {
		if tag.ID != tagID {
			tagIDs = append(tagIDs, string(tag.ID))
		}
	}

	input := ImageUpdateInput{
		ID:     string(imageID),
		TagIds: tagIDs,
	}

	err = UpdateImage(client, imageID, input)
	if err != nil {
		return fmt.Errorf("failed to remove tag from image: %w", err)
	}

	log.Tracef("Removed tag %s from image %s", tagID, imageID)
	return nil
}

// DownloadImage downloads an image from Stash HTTP endpoint
func DownloadImage(imageURL string, sessionCookie *http.Cookie) ([]byte, error) {
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if sessionCookie != nil {
		req.AddCookie(sessionCookie)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	return imageBytes, nil
}
