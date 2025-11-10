package stash

import (
	"context"
	"fmt"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// findOrCreateTag finds a tag by name or creates it if it doesn't exist
func findOrCreateTag(client *graphql.Client, cache *TagCache, tagName string) (graphql.ID, error) {
	// Check cache first
	if id, ok := cache.Get(tagName); ok {
		log.Tracef("Tag '%s' found in cache: %s", tagName, id)
		return id, nil
	}

	// Query for existing tag
	var query struct {
		FindTags struct {
			Count int
			Tags  []struct {
				ID   graphql.ID
				Name string
			}
		} `graphql:"findTags(tag_filter: $filter)"`
	}

	filterInput := &TagFilterType{
		Name: &StringCriterionInput{
			Value:    graphql.String(tagName),
			Modifier: "EQUALS",
		},
	}

	variables := map[string]interface{}{
		"filter": filterInput,
	}

	err := client.Query(context.Background(), &query, variables)
	if err != nil {
		return "", fmt.Errorf("failed to query tags: %w", err)
	}

	// Return existing tag if found
	if len(query.FindTags.Tags) > 0 {
		tagID := query.FindTags.Tags[0].ID
		cache.Set(tagName, tagID)
		log.Debugf("Found existing tag '%s': %s", tagName, tagID)
		return tagID, nil
	}

	// Create new tag
	var mutation struct {
		TagCreate struct {
			ID   graphql.ID
			Name string
		} `graphql:"tagCreate(input: $input)"`
	}

	createInput := TagCreateInput{
		Name: graphql.String(tagName),
	}

	createVars := map[string]interface{}{
		"input": createInput,
	}

	err = client.Mutate(context.Background(), &mutation, createVars)
	if err != nil {
		return "", fmt.Errorf("failed to create tag: %w", err)
	}

	tagID := mutation.TagCreate.ID
	cache.Set(tagName, tagID)
	log.Infof("Created tag '%s': %s", tagName, tagID)
	return tagID, nil
}

// GetOrCreateTag gets or creates a tag by name (convenience wrapper)
func GetOrCreateTag(client *graphql.Client, cache *TagCache, tagName string, defaultName string) (graphql.ID, error) {
	if tagName == "" {
		tagName = defaultName
	}
	return findOrCreateTag(client, cache, tagName)
}

// TriggerMetadataScan triggers a metadata scan
func TriggerMetadataScan(client *graphql.Client) error {
	var mutation struct {
		MetadataScan graphql.String `graphql:"metadataScan(input: {})"`
	}

	err := client.Mutate(context.Background(), &mutation, nil)
	if err != nil {
		return fmt.Errorf("failed to trigger metadata scan: %w", err)
	}

	log.Info("Triggered metadata scan")
	return nil
}
