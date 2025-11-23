package stash

import (
	"context"
	"fmt"
	"strings"
	"time"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// ============================================================================
// Performer Data Operations (Repository Layer)
// ============================================================================

// GetPerformerByID retrieves a performer by their ID
func GetPerformerByID(client *graphql.Client, performerID graphql.ID) (*Performer, error) {
	var query struct {
		Performer `graphql:"findPerformer(id: $id)"`
	}

	variables := map[string]interface{}{
		"id": performerID,
	}

	err := client.Query(context.Background(), &query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to query performer: %w", err)
	}

	log.Debugf("Found performer: %v for performer id: %s", query.Performer, performerID)
	return &query.Performer, nil
}

// FindPerformer finds a single performer with optional filtering
func FindPerformer(client *graphql.Client, filter PerformerFilterType) (*Performer, error) {
	res, _, err := FindPerformers(client, &filter, 1, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to find performer: %w", err)
	}
	if len(res) == 0 {
		return nil, nil // Not found
	}
	return &res[0], nil
}

// FindPerformers finds performers with optional filtering
func FindPerformers(client *graphql.Client, filter *PerformerFilterType, page int, perPage int) ([]Performer, int, error) {
	var query struct {
		FindPerformers struct {
			Count      int
			Performers []Performer
		} `graphql:"findPerformers(performer_filter: $filter, filter: $page_filter)"`
	}

	pageInt := int(page)
	perPageInt := int(perPage)
	pageFilter := &FindFilterType{
		Page:    &pageInt,
		PerPage: &perPageInt,
	}

	variables := map[string]interface{}{
		"page_filter": pageFilter,
		"filter":      filter,
	}

	err := client.Query(context.Background(), &query, variables)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query performers: %w", err)
	}

	log.Debugf("Found %d performers (page %d, per_page %d)", len(query.FindPerformers.Performers), page, perPage)
	return query.FindPerformers.Performers, query.FindPerformers.Count, nil
}

// CreatePerformer creates a new performer
func CreatePerformer(client *graphql.Client, performerSubject PerformerSubject) (graphql.ID, error) {
	return CreatePerformerWithImage(client, performerSubject)
}

// CreatePerformerWithImage creates a new performer with optional image URL
func CreatePerformerWithImage(client *graphql.Client, performerSubject PerformerSubject) (graphql.ID, error) {
	ctx := context.Background()

	if performerSubject.ID != "" {
		return graphql.ID(performerSubject.ID), nil
	}

	if performerSubject.Name == "" {
		return "", fmt.Errorf("performer name is required")
	}

	input := PerformerCreateInput{
		Name: performerSubject.Name,
	}

	aliases := performerSubject.Aliases
	age := performerSubject.Age
	gender := performerSubject.Gender
	imageURL := performerSubject.Image

	if len(aliases) > 0 {
		input.AliasList = aliases
	}

	if age > 0 {
		birthDay := CalculateBirthdayFromAge(age)
		input.Birthdate = &birthDay
	}

	if gender != "" {
		stashGender, err := ParseGenderEnum(gender)
		if err == nil {
			modelGender := models.GenderEnum(stashGender)
			input.Gender = &modelGender
		}
	}

	if imageURL != "" {
		input.Image = &imageURL
	}

	var mutation struct {
		PerformerCreate PerformerCreate `graphql:"performerCreate(input: $input)"`
	}

	variables := map[string]interface{}{
		"input": input,
	}

	err := client.Mutate(ctx, &mutation, variables)
	if err != nil {
		return "", fmt.Errorf("failed to create performer: %w", err)
	}

	performerID := mutation.PerformerCreate.ID
	log.Infof("Created performer '%s': %s", performerSubject.Name, performerID)
	return performerID, nil
}

// UpdatePerformer updates performer details
func UpdatePerformer(client *graphql.Client, performerID graphql.ID, input PerformerUpdateInput) error {
	ctx := context.Background()

	var mutation struct {
		PerformerUpdate PerformerUpdateInput `graphql:"performerUpdate(input: $input)"`
	}

	variables := map[string]interface{}{
		"input": input,
	}

	err := client.Mutate(ctx, &mutation, variables)
	if err != nil {
		return fmt.Errorf("failed to update performer: %w", err)
	}

	log.Debugf("Updated performer %s", performerID)
	return nil
}

// AddTagToPerformer adds a tag to a performer
func AddTagToPerformer(client *graphql.Client, performerID graphql.ID, tagID graphql.ID) error {
	performer, err := GetPerformerByID(client, performerID)
	if err != nil {
		return fmt.Errorf("failed to get performer: %w", err)
	}

	// Check if already has tag
	for _, tag := range performer.Tags {
		if tag.ID == tagID {
			log.Tracef("Performer %s already has tag %s", performerID, tagID)
			return nil // Already has tag
		}
	}

	// Build tag ID list with new tag
	tagIDs := make([]string, len(performer.Tags)+1)
	for i, tag := range performer.Tags {
		tagIDs[i] = string(tag.ID)
	}
	tagIDs[len(performer.Tags)] = string(tagID)
	input := PerformerUpdateInput{
		ID:     string(performerID),
		TagIds: tagIDs,
	}

	// Update performer with new tag
	err = UpdatePerformer(client, performerID, input)
	if err != nil {
		return fmt.Errorf("failed to update performer tags: %w", err)
	}

	log.Tracef("Added tag %s to performer %s", tagID, performerID)
	return nil
}

// FindPerformerBySubjectName finds a performer by Compreface subject name/alias
func FindPerformerBySubjectName(client *graphql.Client, subjectName string) (graphql.ID, error) {
	// Try to find performer by name or alias
	nameFilter := PerformerFilterType{
		Name: &StringCriterionInput{
			Value:    subjectName,
			Modifier: CriterionModifierEquals,
		},
	}

	named, err := FindPerformer(client, nameFilter)
	if err != nil {
		return "", fmt.Errorf("failed to query performer: %w", err)
	}

	if named != nil {
		return named.ID, nil
	}

	aliasFilter := PerformerFilterType{
		Aliases: &StringCriterionInput{
			Value:    subjectName,
			Modifier: CriterionModifierEquals,
		},
	}

	aliased, err := FindPerformer(client, aliasFilter)

	if err != nil {
		return "", fmt.Errorf("failed to query performer: %w", err)
	}

	if aliased != nil {
		return aliased.ID, nil
	}

	return "", nil // Not found (not an error)
}

// Converts a string to GenderEnum
func ParseGenderEnum(s string) (GenderEnum, error) {
	normalized := strings.ToUpper(strings.TrimSpace(s))

	switch normalized {
	case string(GenderEnumMale):
		return GenderEnumMale, nil
	case string(GenderEnumFemale):
		return GenderEnumFemale, nil
	case string(GenderEnumTransgenderMale):
		return GenderEnumTransgenderMale, nil
	case string(GenderEnumTransgenderFemale):
		return GenderEnumTransgenderFemale, nil
	case string(GenderEnumIntersex):
		return GenderEnumIntersex, nil
	case string(GenderEnumNonBinary):
		return GenderEnumNonBinary, nil
	default:
		return "", fmt.Errorf("invalid gender enum: %s", s)
	}
}

// CalculateBirthdayFromAge calculates a birthdate string (YYYY-MM-DD) from age in years
func CalculateBirthdayFromAge(age int) string {
	now := time.Now()
	birthDate := now.AddDate(-age, 0, 0)
	return birthDate.Format("2006-01-02")
}

// CaclulateAgeFromBirthday calculates age in years from a birthdate string (YYYY-MM-DD)
func CaclulateAgeFromBirthday(birthdate string) (int, error) {
	if birthdate == "" {
		return 0, fmt.Errorf("birthdate is empty")
	}
	layout := "2006-01-02"
	birthTime, err := time.Parse(layout, birthdate)
	if err != nil {
		return 0, fmt.Errorf("failed to parse birthdate: %w", err)
	}
	now := time.Now()
	age := now.Year() - birthTime.Year()
	if now.YearDay() < birthTime.YearDay() {
		age--
	}
	return age, nil
}
