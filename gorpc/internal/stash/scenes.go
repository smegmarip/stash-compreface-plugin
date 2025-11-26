package stash

import (
	"context"
	"fmt"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// FindScenes queries scenes with pagination
func FindScenes(client *graphql.Client, filter *SceneFilterType, page, perPage int) ([]Scene, int, error) {
	ctx := context.Background()

	var query struct {
		FindScenes struct {
			Count  int     `graphql:"count"`
			Scenes []Scene `graphql:"scenes"`
		} `graphql:"findScenes(filter: $filter, scene_filter: $scene_filter)"`
	}

	pageInt := int(page)
	perPageInt := int(perPage)
	filterInput := &FindFilterType{
		Page:    &pageInt,
		PerPage: &perPageInt,
	}

	variables := map[string]interface{}{
		"filter":       filterInput,
		"scene_filter": filter,
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

// UpdateScene updates a scene with the provided input
func UpdateScene(client *graphql.Client, sceneID graphql.ID, input SceneUpdateInput) error {
	ctx := context.Background()

	var mutation struct {
		SceneUpdate SceneUpdate `graphql:"sceneUpdate(input: $input)"`
	}

	variables := map[string]interface{}{
		"input": input,
	}

	err := client.Mutate(ctx, &mutation, variables)
	if err != nil {
		return fmt.Errorf("scene update mutation failed: %w", err)
	}

	log.Infof("Successfully updated scene %s", sceneID)
	return nil
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
	sceneIDStr := string(sceneID)
	tagIDStrs := make([]string, len(tagIDs))
	for i, id := range tagIDs {
		tagIDStrs[i] = string(id)
	}

	input := SceneUpdateInput{
		ID:     sceneIDStr,
		TagIds: tagIDStrs,
	}

	err := UpdateScene(client, sceneID, input)
	if err != nil {
		return fmt.Errorf("failed to update scene tags: %w", err)
	}

	log.Debugf("Updated tags for scene %s", sceneID)
	return nil
}

// RemoveTagFromScene removes a tag from a scene
func RemoveTagFromScene(client *graphql.Client, sceneID graphql.ID, tagID graphql.ID) error {
	// Get current tags
	scene, err := GetScene(client, sceneID)
	if err != nil {
		return fmt.Errorf("failed to get scene: %w", err)
	}

	// Filter out the tag to remove
	tagIDs := []graphql.ID{}
	for _, tag := range scene.Tags {
		if tag.ID != tagID {
			tagIDs = append(tagIDs, tag.ID)
		}
	}

	err = UpdateSceneTags(client, sceneID, tagIDs)
	if err != nil {
		return fmt.Errorf("failed to remove tag from scene: %w", err)
	}

	log.Tracef("Removed tag %s from scene %s", tagID, sceneID)
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

	sceneIDStr := string(sceneID)
	performerIDStrs := make([]string, len(performerIDs))
	for i, id := range performerIDs {
		performerIDStrs[i] = string(id)
	}

	updateInput := SceneUpdateInput{
		ID:           sceneIDStr,
		PerformerIds: performerIDStrs,
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
