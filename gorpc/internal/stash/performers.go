package stash

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// ============================================================================
// Performer Data Operations (Repository Layer)
// ============================================================================

// FindPerformers finds performers with optional filtering
func FindPerformers(client *graphql.Client, filter map[string]interface{}, page int, perPage int) ([]Performer, int, error) {
	var query struct {
		FindPerformers struct {
			Count      int
			Performers []Performer
		} `graphql:"findPerformers(filter: $page_filter)"`
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
		return nil, 0, fmt.Errorf("failed to query performers: %w", err)
	}

	log.Debugf("Found %d performers (page %d, per_page %d)", len(query.FindPerformers.Performers), page, perPage)
	return query.FindPerformers.Performers, query.FindPerformers.Count, nil
}

// CreatePerformer creates a new performer
func CreatePerformer(client *graphql.Client, name string, aliases []string) (graphql.ID, error) {
	ctx := context.Background()

	// Build alias list JSON
	aliasJSON := "[]"
	if len(aliases) > 0 {
		aliasBytes, err := json.Marshal(aliases)
		if err != nil {
			return "", fmt.Errorf("failed to marshal aliases: %w", err)
		}
		aliasJSON = string(aliasBytes)
	}

	// Use ExecRaw with literal query to avoid nullable type issues
	query := fmt.Sprintf(`mutation {
		performerCreate(input: {name: "%s", alias_list: %s}) {
			id
			name
		}
	}`, name, aliasJSON)

	data, err := client.ExecRaw(ctx, query, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create performer: %w", err)
	}

	// Unmarshal response
	var response struct {
		PerformerCreate struct {
			ID   graphql.ID `json:"id"`
			Name string     `json:"name"`
		} `json:"performerCreate"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal performer create response: %w", err)
	}

	performerID := response.PerformerCreate.ID
	log.Infof("Created performer '%s': %s", name, performerID)
	return performerID, nil
}

// UpdatePerformer updates performer details
func UpdatePerformer(client *graphql.Client, performerID graphql.ID, name *string, aliases []string, tagIDs []graphql.ID) error {
	ctx := context.Background()

	// Build the mutation query dynamically based on provided parameters
	var inputParts []string

	// ID is always required
	inputParts = append(inputParts, fmt.Sprintf("id: \"%s\"", performerID))

	// Add name if provided
	if name != nil {
		inputParts = append(inputParts, fmt.Sprintf("name: \"%s\"", *name))
	}

	// Add aliases if provided
	if aliases != nil {
		aliasJSON, err := json.Marshal(aliases)
		if err != nil {
			return fmt.Errorf("failed to marshal aliases: %w", err)
		}
		inputParts = append(inputParts, fmt.Sprintf("alias_list: %s", string(aliasJSON)))
	}

	// Add tag IDs if provided
	if tagIDs != nil {
		tagIDStrs := make([]string, len(tagIDs))
		for i, id := range tagIDs {
			tagIDStrs[i] = fmt.Sprintf("\"%s\"", id)
		}
		inputParts = append(inputParts, fmt.Sprintf("tag_ids: [%s]", strings.Join(tagIDStrs, ",")))
	}

	// Build the complete query
	query := fmt.Sprintf(`mutation {
		performerUpdate(input: {%s}) {
			id
			name
		}
	}`, strings.Join(inputParts, ", "))

	data, err := client.ExecRaw(ctx, query, nil)
	if err != nil {
		return fmt.Errorf("failed to update performer: %w", err)
	}

	// Unmarshal response
	var response struct {
		PerformerUpdate struct {
			ID   graphql.ID `json:"id"`
			Name string     `json:"name"`
		} `json:"performerUpdate"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to unmarshal performer update response: %w", err)
	}

	log.Debugf("Updated performer %s to name '%s'", performerID, response.PerformerUpdate.Name)
	return nil
}

// AddTagToPerformer adds a tag to a performer
func AddTagToPerformer(client *graphql.Client, performerID graphql.ID, tagID graphql.ID) error {
	ctx := context.Background()

	// Query the specific performer to get current tags
	query := fmt.Sprintf(`query {
		findPerformer(id: "%s") {
			id
			tags {
				id
			}
		}
	}`, performerID)

	data, err := client.ExecRaw(ctx, query, nil)
	if err != nil {
		return fmt.Errorf("failed to query performer: %w", err)
	}

	// Parse response
	var response struct {
		FindPerformer struct {
			ID   graphql.ID `json:"id"`
			Tags []struct {
				ID graphql.ID `json:"id"`
			} `json:"tags"`
		} `json:"findPerformer"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to parse query response: %w", err)
	}

	// Check if already has tag
	for _, tag := range response.FindPerformer.Tags {
		if tag.ID == tagID {
			log.Tracef("Performer %s already has tag %s", performerID, tagID)
			return nil // Already has tag
		}
	}

	// Build tag ID list with new tag
	tagIDs := make([]graphql.ID, len(response.FindPerformer.Tags)+1)
	for i, tag := range response.FindPerformer.Tags {
		tagIDs[i] = tag.ID
	}
	tagIDs[len(response.FindPerformer.Tags)] = tagID

	// Update performer with new tag
	err = UpdatePerformer(client, performerID, nil, nil, tagIDs)
	if err != nil {
		return fmt.Errorf("failed to update performer tags: %w", err)
	}

	log.Tracef("Added tag %s to performer %s", tagID, performerID)
	return nil
}

// FindPerformerBySubjectName finds a performer by Compreface subject name/alias
func FindPerformerBySubjectName(client *graphql.Client, subjectName string) (graphql.ID, error) {
	// Try to find performer by name first
	performers, _, err := FindPerformers(client, map[string]interface{}{}, 1, 1000)
	if err != nil {
		return "", fmt.Errorf("failed to query performers: %w", err)
	}

	// Check performer names and aliases for match
	for _, performer := range performers {
		if performer.Name == subjectName {
			return performer.ID, nil
		}
		for _, alias := range performer.AliasList {
			if alias == subjectName {
				return performer.ID, nil
			}
		}
	}

	return "", nil // Not found (not an error)
}
