// +build integration

package integration_test

import (
	"fmt"
	"testing"
	"time"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
	"github.com/smegmarip/stash-compreface-plugin/tests/testutil"
)

func createTestGraphQLClient(url string) *graphql.Client {
	return graphql.NewClient(url+"/graphql", nil)
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

func TestStashIntegration_CreatePerformer(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	// Create a test performer with unique name (timestamp to avoid conflicts)
	performerName := fmt.Sprintf("Test Compreface Performer %d", time.Now().Unix())
	aliases := []string{"Alias1", "Alias2"}

	performerID, err := stash.CreatePerformer(client, performerName, aliases)
	require.NoError(t, err, "failed to create performer")
	require.NotEmpty(t, performerID, "performer ID should not be empty")

	t.Logf("Created performer ID: %s", performerID)

	// Verify performer was created by searching for it
	filter := map[string]interface{}{
		"name": map[string]interface{}{
			"value":    performerName,
			"modifier": "EQUALS",
		},
	}

	performers, _, err := stash.FindPerformers(client, filter, 1, 10)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(performers), 1, "should find at least one performer")

	found := false
	for _, perf := range performers {
		if perf.ID == performerID {
			found = true
			assert.Equal(t, performerName, perf.Name)
			t.Logf("Verified performer: %s with aliases %v", perf.Name, perf.AliasList)
			break
		}
	}
	assert.True(t, found, "created performer should be findable")

	// Note: We don't delete the performer as it might be referenced elsewhere
	// and Stash might have dependencies on it
}
