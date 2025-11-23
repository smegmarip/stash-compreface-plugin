package stash

import (
	"context"
	"fmt"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// FindGalleries queries galleries with pagination
func FindGalleries(client *graphql.Client, filter *GalleryFilterType, page, perPage int) ([]Gallery, int, error) {
	ctx := context.Background()

	var query struct {
		FindGalleries struct {
			Count     int
			Galleries []Gallery
		} `graphql:"findGalleries(filter: $filter, gallery_filter: $gallery_filter)"`
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
		variables["gallery_filter"] = filter
	} else {
		variables["gallery_filter"] = GalleryFilterType{}
	}

	err := client.Query(ctx, &query, variables)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query galleries: %w", err)
	}

	log.Debugf("FindGalleries returned %d galleries (total count: %d)", len(query.FindGalleries.Galleries), query.FindGalleries.Count)

	return query.FindGalleries.Galleries, query.FindGalleries.Count, nil
}

// GetGallery retrieves a single gallery by ID
func GetGallery(client *graphql.Client, galleryID graphql.ID) (*Gallery, error) {
	ctx := context.Background()

	var query struct {
		FindGallery *Gallery `graphql:"findGallery(id: $id)"`
	}

	variables := map[string]interface{}{
		"id": galleryID,
	}

	err := client.Query(ctx, &query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to query gallery: %w", err)
	}

	if query.FindGallery == nil {
		return nil, fmt.Errorf("gallery not found")
	}

	return query.FindGallery, nil
}

// UpdateGallery updates a gallery with the provided input
func UpdateGallery(client *graphql.Client, galleryID graphql.ID, input GalleryUpdateInput) error {
	ctx := context.Background()

	var mutation struct {
		GalleryUpdate GalleryUpdate `graphql:"galleryUpdate(input: $input)"`
	}

	variables := map[string]interface{}{
		"input": input,
	}

	err := client.Mutate(ctx, &mutation, variables)
	if err != nil {
		return fmt.Errorf("failed to update gallery: %w", err)
	}

	log.Debugf("Updated gallery: %v", mutation.GalleryUpdate)

	return nil
}

// AddTagToGallery adds a tag to a gallery
func AddTagToGallery(client *graphql.Client, galleryID graphql.ID, tagID graphql.ID) error {
	ctx := context.Background()

	var mutation struct {
		GalleryUpdate GalleryUpdate `graphql:"galleryUpdate(input: $input)"`
	}

	input := GalleryUpdateInput{
		ID:     string(galleryID),
		TagIds: []string{string(tagID)},
	}

	variables := map[string]interface{}{
		"input": input,
	}

	err := client.Mutate(ctx, &mutation, variables)
	if err != nil {
		return fmt.Errorf("failed to add tag to gallery: %w", err)
	}

	log.Debugf("Added tag %s to gallery %s", tagID, galleryID)
	return nil
}

// UpdateGalleryTags updates gallery tags (replaces all tags)
func UpdateGalleryTags(client *graphql.Client, galleryID graphql.ID, tagIDs []graphql.ID) error {
	galleryIDStr := string(galleryID)
	tagIDStrs := make([]string, len(tagIDs))
	for i, id := range tagIDs {
		tagIDStrs[i] = string(id)
	}

	input := GalleryUpdateInput{
		ID:     galleryIDStr,
		TagIds: tagIDStrs,
	}

	err := UpdateGallery(client, galleryID, input)
	if err != nil {
		return fmt.Errorf("failed to update gallery tags: %w", err)
	}

	log.Debugf("Updated tags for gallery %s", galleryID)
	return nil
}

// RemoveTagFromGallery removes a tag from a gallery
func RemoveTagFromGallery(client *graphql.Client, galleryID graphql.ID, tagID graphql.ID) error {
	// Get current tags
	gallery, err := GetGallery(client, galleryID)
	if err != nil {
		return fmt.Errorf("failed to get gallery: %w", err)
	}

	// Filter out the tag to remove
	tagIDs := []graphql.ID{}
	for _, tag := range gallery.Tags {
		if tag.ID != tagID {
			tagIDs = append(tagIDs, tag.ID)
		}
	}

	err = UpdateGalleryTags(client, galleryID, tagIDs)
	if err != nil {
		return fmt.Errorf("failed to remove tag from gallery: %w", err)
	}

	log.Tracef("Removed tag %s from gallery %s", tagID, galleryID)
	return nil
}
