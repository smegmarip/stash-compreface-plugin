//go:build integration
// +build integration

package integration_test

import (
	"strings"
	"testing"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
	"github.com/smegmarip/stash-compreface-plugin/tests/testutil"
)

func createTestGraphQLClient(url string) *graphql.Client {
	return stash.TestClient(url+"/graphql", nil)
}

func TestStashIntegration_FindImages(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	// Find first 10 images
	images, total, err := stash.FindImages(client, nil, 1, 10)
	require.NoError(t, err, "failed to find images")

	t.Logf("Found %d images (total: %d)", len(images), total)

	assert.LessOrEqual(t, len(images), 10, "should not return more than requested")

	for i, img := range images {
		if i >= 3 {
			break // Only log first 3
		}
		path := ""
		if len(img.Files) > 0 {
			path = img.Files[0].Path
		}
		t.Logf("Image %d: ID=%s, Path=%s, Tags=%d",
			i+1, img.ID, path, len(img.Tags))
	}
}

func TestStashIntegration_GetPerformerByID(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	performerID := graphql.ID("7") // Example performer ID; adjust as needed
	performer, err := stash.GetPerformerByID(client, performerID)
	require.NoError(t, err, "failed to get performer by ID")

	t.Logf("Found performer: ID=%s, Name=%s, Aliases=%v",
		performer.ID, performer.Name, performer.AliasList)
}

func TestStashIntegration_FindPerformers(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	// Find first 10 performers
	performers, total, err := stash.FindPerformers(client, nil, 1, 10)
	require.NoError(t, err, "failed to find performers")

	t.Logf("Found %d performers (total: %d)", len(performers), total)

	assert.LessOrEqual(t, len(performers), 10, "should not return more than requested")

	for i, perf := range performers {
		if i >= 3 {
			break // Only log first 3
		}
		t.Logf("Performer %d: ID=%s, Name=%s, Aliases=%v",
			i+1, perf.ID, perf.Name, perf.AliasList)
	}
}

func TestStashIntegration_FindFilteredIndividualPerformer(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	searchTerm := "Devyn"

	// Find performer with search term
	filter := stash.PerformerFilterType{
		Name: &stash.StringCriterionInput{
			Value:    searchTerm,
			Modifier: stash.CriterionModifierIncludes,
		},
	}

	performer, err := stash.FindPerformer(client, filter)
	require.NoError(t, err, "failed to find performer with filter")

	if performer == nil {
		t.Logf("No performer found with name: %s", searchTerm)
	} else {
		t.Logf("Found performer: ID=%s, Name=%s, Aliases=%v",
			performer.ID, performer.Name, performer.AliasList)
	}
}

func TestStashIntegration_CreatePerformerWithImage(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	performer, err := testutil.RandomUser(testutil.GenderFemale)
	require.NoError(t, err, "failed to create random performer")

	imageURL := performer.URLs.List()[0]
	performerAliases := performer.Aliases.List()
	performerAliases = append(performerAliases, "Test CreatePerformerWithImage")
	performerAge, err := stash.CaclulateAgeFromBirthday(performer.Birthdate.Format("2006-01-02"))
	require.NoError(t, err, "failed to calculate age from birthday")

	performerSubject := stash.PerformerSubject{
		Name:    performer.Name,
		Age:     performerAge,
		Aliases: performerAliases,
		Gender:  performer.Gender.String(),
		Image:   imageURL,
	}

	performerID, err := stash.CreatePerformerWithImage(client, performerSubject)
	require.NoError(t, err, "failed to create performer with image")

	t.Logf("Created performer: ID=%s, Name=%s, Aliases=%v, ImagePath=%s",
		performerID, performerSubject.Name, performerSubject.Aliases, imageURL)
}

func TestStashIntegration_FindUnsychronizedPerformers(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	syncTagName := "Compreface Synced"
	syncTagID, err := stash.GetOrCreateTag(client, stash.NewTagCache(), syncTagName, syncTagName)
	require.NoError(t, err, "failed to get or create sync tag")

	subjectCriterion := stash.StringCriterionInput{
		Value:    "Person ",
		Modifier: stash.CriterionModifierIncludes,
	}
	tagsFilter := stash.HierarchicalMultiCriterionInput{
		Value:    []string{string(syncTagID)},
		Modifier: stash.CriterionModifierExcludes,
	}

	// Find performer without sync tag
	filter := stash.PerformerFilterType{
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

	unfiltered, total, err := stash.FindPerformers(client, &filter, 1, 10)
	require.NoError(t, err, "failed to find unsynchronized performer with filter")

	t.Logf("Found %d performers (total: %d)", len(unfiltered), total)

	assert.LessOrEqual(t, len(unfiltered), 10, "should not return more than requested")

	// Filter performers without an Image URL
	var performers []stash.Performer
	for _, p := range unfiltered {
		if p.ImagePath != "" && !strings.Contains(p.ImagePath, "default=true") {
			performers = append(performers, p)
		}
	}

	t.Logf("Filtered to %d performers with images", len(performers))

	for i, perf := range performers {
		if i >= 3 {
			break // Only log first 3
		}
		t.Logf("Performer %d: ID=%s, Name=%s, Aliases=%v ImagePath=%s",
			i+1, perf.ID, perf.Name, perf.AliasList, perf.ImagePath)
	}
}

func TestStashIntegration_FindFilteredPerformers(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	var results []stash.Performer
	var total int
	searchTerm := "Person"

	// Find performers with name (or alias) containing "Person"
	nameFilter := stash.PerformerFilterType{
		Name: &stash.StringCriterionInput{
			Value:    searchTerm,
			Modifier: stash.CriterionModifierIncludes,
		},
	}

	named, _total, err := stash.FindPerformers(client, &nameFilter, 1, 10)
	require.NoError(t, err, "failed to find performers with filter")

	total += _total
	if len(named) > 0 {
		results = append(results, named...)
	}

	// Find performers with name (or alias) containing "Person"
	aliasFilter := stash.PerformerFilterType{
		Aliases: &stash.StringCriterionInput{
			Value:    searchTerm,
			Modifier: stash.CriterionModifierIncludes,
		},
	}

	aliased, _total, err := stash.FindPerformers(client, &aliasFilter, 1, 10)
	require.NoError(t, err, "failed to find performers with filter")

	total += _total
	if len(aliased) > 0 {
		results = append(results, aliased...)
	}

	// Remove duplicates
	performerMap := make(map[graphql.ID]stash.Performer)
	for _, p := range results {
		performerMap[p.ID] = p
	}
	performers := make([]stash.Performer, 0, len(performerMap))
	for _, p := range performerMap {
		performers = append(performers, p)
	}

	t.Logf("Found %d performers with cumulative filter (total: %d)", len(performers), total)

	for i, perf := range performers {
		if i >= 3 {
			break // Only log first 3
		}
		t.Logf("Filtered Performer %d: ID=%s, Name=%s, Aliases=%v",
			i+1, perf.ID, perf.Name, perf.AliasList)
	}
}

func TestStashIntegration_TagOperations(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)
	cache := stash.NewTagCache()

	// Create a test tag
	tagName := "Test Compreface Integration"

	tagID, err := stash.GetOrCreateTag(client, cache, tagName, tagName)
	require.NoError(t, err, "failed to create tag")
	require.NotEmpty(t, tagID, "tag ID should not be empty")

	t.Logf("Created/found tag '%s' with ID: %s", tagName, tagID)

	// Verify we can find it again (should be cached or found)
	tagID2, err := stash.GetOrCreateTag(client, cache, tagName, tagName)
	require.NoError(t, err)
	assert.Equal(t, tagID, tagID2, "should return same tag ID")

	// Note: We don't delete the tag as Stash might have other references to it
	// and tag deletion might require more complex cleanup
}

func TestStashIntegration_TagCache(t *testing.T) {
	cache := stash.NewTagCache()

	// Test cache miss
	_, found := cache.Get("test-tag")
	assert.False(t, found, "cache should be empty initially")

	// Test cache set and get
	testID := graphql.ID("123")
	cache.Set("test-tag", testID)

	retrievedID, found := cache.Get("test-tag")
	assert.True(t, found, "tag should be in cache")
	assert.Equal(t, testID, retrievedID, "retrieved ID should match")

	// Test multiple tags
	cache.Set("tag1", graphql.ID("1"))
	cache.Set("tag2", graphql.ID("2"))
	cache.Set("tag3", graphql.ID("3"))

	id1, _ := cache.Get("tag1")
	id2, _ := cache.Get("tag2")
	id3, _ := cache.Get("tag3")

	assert.Equal(t, graphql.ID("1"), id1)
	assert.Equal(t, graphql.ID("2"), id2)
	assert.Equal(t, graphql.ID("3"), id3)
}

func TestStashIntegration_ImageTagOperations(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)
	cache := stash.NewTagCache()

	// Find an image to test with
	images, _, err := stash.FindImages(client, nil, 1, 1)
	require.NoError(t, err)

	if len(images) == 0 {
		t.Skip("No images in Stash, skipping image tag test")
	}

	testImage := images[0]
	t.Logf("Testing with image ID: %s", testImage.ID)

	// Create a test tag
	tagName := "Compreface Integration Test"
	tagID, err := stash.GetOrCreateTag(client, cache, tagName, tagName)
	require.NoError(t, err)
	t.Logf("Created test tag: %s", tagID)

	// Add tag to image
	err = stash.AddTagToImage(client, testImage.ID, tagID)
	require.NoError(t, err, "failed to add tag to image")
	t.Logf("Added tag to image")

	// Verify tag was added by fetching image
	updatedImage, err := stash.GetImage(client, testImage.ID)
	require.NoError(t, err)

	hasTag := false
	for _, tag := range updatedImage.Tags {
		if tag.ID == tagID {
			hasTag = true
			break
		}
	}
	assert.True(t, hasTag, "image should have the tag")

	// Remove tag from image
	err = stash.RemoveTagFromImage(client, testImage.ID, tagID)
	require.NoError(t, err, "failed to remove tag from image")
	t.Logf("Removed tag from image")

	// Verify tag was removed
	finalImage, err := stash.GetImage(client, testImage.ID)
	require.NoError(t, err)

	stillHasTag := false
	for _, tag := range finalImage.Tags {
		if tag.ID == tagID {
			stillHasTag = true
			break
		}
	}
	assert.False(t, stillHasTag, "image should not have the tag anymore")
}

func TestStashIntegration_SceneTagOperations(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)
	cache := stash.NewTagCache()

	// Find a scene to test with
	scenes, _, err := stash.FindScenes(client, nil, 1, 1)
	require.NoError(t, err)

	if len(scenes) == 0 {
		t.Skip("No scenes in Stash, skipping scene tag test")
	}

	testScene := scenes[0]
	t.Logf("Testing with scene ID: %s", testScene.ID)

	// Create a test tag
	tagName := "Compreface Scene Test"
	tagID, err := stash.GetOrCreateTag(client, cache, tagName, tagName)
	require.NoError(t, err)
	t.Logf("Created test tag: %s", tagID)

	// Add tag to scene
	err = stash.AddTagToScene(client, testScene.ID, tagID)
	require.NoError(t, err, "failed to add tag to scene")
	t.Logf("Added tag to scene")

	// Verify tag was added by fetching scene
	updatedScene, err := stash.GetScene(client, testScene.ID)
	require.NoError(t, err)

	hasTag := false
	for _, tag := range updatedScene.Tags {
		if tag.ID == tagID {
			hasTag = true
			break
		}
	}
	assert.True(t, hasTag, "scene should have the tag")

	// Remove tag from scene
	err = stash.RemoveTagFromScene(client, testScene.ID, tagID)
	require.NoError(t, err, "failed to remove tag from scene")
	t.Logf("Removed tag from scene")

	// Verify tag was removed
	finalScene, err := stash.GetScene(client, testScene.ID)
	require.NoError(t, err)

	stillHasTag := false
	for _, tag := range finalScene.Tags {
		if tag.ID == tagID {
			stillHasTag = true
			break
		}
	}
	assert.False(t, stillHasTag, "scene should not have the tag anymore")
}

func TestStashIntegration_CreatePerformerStash(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	// Create a random test performer
	randomUser, err := testutil.RandomUser(testutil.GenderFemale)
	require.NoError(t, err, "failed to create random user")
	performerAge, err := stash.CaclulateAgeFromBirthday(
		randomUser.Birthdate.Format("2006-01-02"),
	)
	require.NoError(t, err, "failed to calculate age from birthdate")
	performerSubject := stash.PerformerSubject{
		Name:    randomUser.Name,
		Age:     performerAge,
		Gender:  string(*randomUser.Gender),
		Aliases: randomUser.Aliases.List(),
		Image:   randomUser.URLs.List()[0],
	}

	performerID, err := stash.CreatePerformer(client, performerSubject)
	require.NoError(t, err, "failed to create performer")
	require.NotEmpty(t, performerID, "performer ID should not be empty")

	t.Logf("Created performer ID: %s", performerID)

	// Verify performer was created by searching for it
	filter := stash.PerformerFilterType{
		Name: &stash.StringCriterionInput{
			Value:    randomUser.Name,
			Modifier: stash.CriterionModifierEquals,
		},
	}

	performers, _, err := stash.FindPerformers(client, &filter, 1, 10)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(performers), 1, "should find at least one performer")

	found := false
	for _, perf := range performers {
		if perf.ID == performerID {
			found = true
			assert.Equal(t, randomUser.Name, perf.Name)
			t.Logf("Verified performer: %s with aliases %v", perf.Name, perf.AliasList)
			break
		}
	}
	assert.True(t, found, "created performer should be findable")

	// Note: We don't delete the performer as it might be referenced elsewhere
	// and Stash might have dependencies on it
}

func TestStashIntegration_FindScenesUnfiltered(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	// Find first 10 scenes
	scenes, total, err := stash.FindScenes(client, nil, 1, 10)
	require.NoError(t, err, "failed to find scenes")

	t.Logf("Found %d scenes (total: %d)", len(scenes), total)

	assert.LessOrEqual(t, len(scenes), 10, "should not return more than requested")

	for i, scene := range scenes {
		if i >= 3 {
			break // Only log first 3
		}
		t.Logf("Scene %d: ID=%s, File Path=%s",
			i+1, scene.ID, scene.Files[0].Path)
	}
}

func TestStashIntegration_FindFilteredScenes(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	filter := &stash.SceneFilterType{
		Duration: &stash.IntCriterionInput{
			Value:    300,
			Modifier: stash.CriterionModifierGreaterThan,
		},
	}

	// Find first 10 scenes
	scenes, total, err := stash.FindScenes(client, filter, 1, 10)
	require.NoError(t, err, "failed to find scenes")

	t.Logf("Found %d scenes (total: %d)", len(scenes), total)

	assert.LessOrEqual(t, len(scenes), 10, "should not return more than requested")

	for i, scene := range scenes {
		if i >= 3 {
			break // Only log first 3
		}
		t.Logf("Scene %d: ID=%s, File Path=%s",
			i+1, scene.ID, scene.Files[0].Path)
	}
}
