package stash

import (
	"context"
	"fmt"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// FindScenesResult represents the result of FindScenes query
type FindScenesResult struct {
	Count  int     `graphql:"count"`
	Scenes []Scene `graphql:"scenes"`
}

// SceneUpdateInput represents input for updating a scene
type SceneUpdateInput struct {
	ID           graphql.ID   `json:"id"`
	TagIds       []graphql.ID `json:"tag_ids,omitempty"`
	PerformerIds []graphql.ID `json:"performer_ids,omitempty"`
}

// FindScenes queries scenes with pagination
func FindScenes(client *graphql.Client, page, perPage int) ([]Scene, int, error) {
	ctx := context.Background()

	var query struct {
		FindScenes FindScenesResult `graphql:"findScenes(filter: $f)"`
	}

	pageInt := graphql.Int(page)
	perPageInt := graphql.Int(perPage)
	filterInput := &FindFilterType{
		Page:    &pageInt,
		PerPage: &perPageInt,
	}

	variables := map[string]interface{}{
		"f": filterInput,
	}

	err := client.Query(ctx, &query, variables)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query scenes: %w", err)
	}

	log.Debugf("FindScenes returned %d scenes (total count: %d)", len(query.FindScenes.Scenes), query.FindScenes.Count)

	return query.FindScenes.Scenes, query.FindScenes.Count, nil
}

// GetScene retrieves a single scene by ID
func GetScene(client *graphql.Client, sceneID graphql.ID) (*Scene, error) {
	ctx := context.Background()

	var query struct {
		FindScene *Scene `graphql:"findScene(id: $id)"`
	}

	variables := map[string]interface{}{
		"id": sceneID,
	}

	err := client.Query(ctx, &query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to query scene: %w", err)
	}

	if query.FindScene == nil {
		return nil, fmt.Errorf("scene not found")
	}

	return query.FindScene, nil
}

// AddTagToScene adds a tag to a scene (preserving existing tags)
func AddTagToScene(client *graphql.Client, sceneID graphql.ID, tagID graphql.ID) error {
	// First, get the current scene to retrieve existing tags
	scene, err := GetScene(client, sceneID)
	if err != nil {
		return fmt.Errorf("failed to get scene: %w", err)
	}

	// Build list of existing tag IDs
	tagIDs := []graphql.ID{}
	for _, tag := range scene.Tags {
		tagIDs = append(tagIDs, tag.ID)
	}

	// Check if tag already exists
	for _, existingTagID := range tagIDs {
		if existingTagID == tagID {
			// Tag already present, no update needed
			return nil
		}
	}

	// Add the new tag
	tagIDs = append(tagIDs, tagID)

	return UpdateSceneTags(client, sceneID, tagIDs)
}

// UpdateSceneTags updates a scene's tags (replaces all tags)
func UpdateSceneTags(client *graphql.Client, sceneID graphql.ID, tagIDs []graphql.ID) error {
	ctx := context.Background()

	var mutation struct {
		SceneUpdate struct {
			ID graphql.ID
		} `graphql:"sceneUpdate(input: $input)"`
	}

	updateInput := SceneUpdateInput{
		ID:     sceneID,
		TagIds: tagIDs,
	}

	variables := map[string]interface{}{
		"input": updateInput,
	}

	err := client.Mutate(ctx, &mutation, variables)
	if err != nil {
		return fmt.Errorf("scene update mutation failed: %w", err)
	}

	log.Debugf("Successfully updated tags for scene %s", sceneID)
	return nil
}

// UpdateScenePerformers updates a scene's performers (replaces all performers)
func UpdateScenePerformers(client *graphql.Client, sceneID graphql.ID, performerIDs []graphql.ID) error {
	ctx := context.Background()

	var mutation struct {
		SceneUpdate struct {
			ID graphql.ID
		} `graphql:"sceneUpdate(input: $input)"`
	}

	updateInput := SceneUpdateInput{
		ID:           sceneID,
		PerformerIds: performerIDs,
	}

	variables := map[string]interface{}{
		"input": updateInput,
	}

	err := client.Mutate(ctx, &mutation, variables)
	if err != nil {
		return fmt.Errorf("scene update mutation failed: %w", err)
	}

	log.Infof("Successfully updated performers for scene %s (%d performers)", sceneID, len(performerIDs))
	return nil
}

// AddPerformerToScene adds a performer to a scene (preserving existing performers)
func AddPerformerToScene(client *graphql.Client, sceneID graphql.ID, performerID graphql.ID) error {
	// First, get the current scene to retrieve existing performers
	scene, err := GetScene(client, sceneID)
	if err != nil {
		return fmt.Errorf("failed to get scene: %w", err)
	}

	// Build list of existing performer IDs
	performerIDs := []graphql.ID{}
	for _, performer := range scene.Performers {
		performerIDs = append(performerIDs, performer.ID)
	}

	// Check if performer already exists
	for _, existingPerformerID := range performerIDs {
		if existingPerformerID == performerID {
			// Performer already present, no update needed
			return nil
		}
	}

	// Add the new performer
	performerIDs = append(performerIDs, performerID)

	return UpdateScenePerformers(client, sceneID, performerIDs)
}
