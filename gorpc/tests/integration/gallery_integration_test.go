package integration

import (
	"testing"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
	"github.com/smegmarip/stash-compreface-plugin/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestGraphQLClient(url string) *graphql.Client {
	return stash.TestClient(url+"/graphql", nil)
}

func TestStashIntegration_FindGalleries(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	// Test: Find galleries with no filter
	galleries, count, err := stash.FindGalleries(client, nil, 1, 10)
	require.NoError(t, err, "failed to find galleries")

	t.Logf("Found %d galleries (total count: %d)", len(galleries), count)

	// Validate results
	assert.GreaterOrEqual(t, count, 0, "count should be non-negative")
	assert.LessOrEqual(t, len(galleries), 10, "should not exceed per_page limit")
	assert.LessOrEqual(t, len(galleries), count, "returned galleries should not exceed total count")

	// If galleries exist, validate structure
	if len(galleries) > 0 {
		gallery := galleries[0]
		assert.NotEmpty(t, gallery.ID, "gallery should have an ID")
		t.Logf("First gallery: ID=%s, Title=%s, ImageCount=%d", gallery.ID, gallery.Title, gallery.ImageCount)
	}
}

func TestStashIntegration_GetGallery(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)

	// Find a gallery to test with
	galleries, _, err := stash.FindGalleries(client, nil, 1, 1)
	require.NoError(t, err)

	if len(galleries) == 0 {
		t.Skip("No galleries in Stash, skipping get gallery test")
	}

	testGallery := galleries[0]
	testGalleryID := testGallery.ID
	t.Logf("Testing with gallery ID: %s", testGalleryID)

	// Test: Get gallery by ID
	gallery, err := stash.GetGallery(client, testGalleryID)
	require.NoError(t, err, "failed to get gallery")
	require.NotNil(t, gallery, "gallery should not be nil")

	// Validate gallery data
	assert.Equal(t, testGallery.ID, gallery.ID, "gallery ID should match")
	assert.Equal(t, testGallery.Title, gallery.Title, "gallery title should match")
	t.Logf("Retrieved gallery: ID=%s, Title=%s, ImageCount=%d", gallery.ID, gallery.Title, gallery.ImageCount)
}

func TestStashIntegration_GalleryTagOperations(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)
	cache := stash.NewTagCache()

	// Find a gallery to test with
	galleries, _, err := stash.FindGalleries(client, nil, 1, 1)
	require.NoError(t, err)

	if len(galleries) == 0 {
		t.Skip("No galleries in Stash, skipping gallery tag test")
	}

	testGallery := galleries[0]
	testGalleryID := testGallery.ID
	t.Logf("Testing with gallery ID: %s", testGalleryID)

	// Create a test tag
	tagName := "Compreface Gallery Test"
	tagID, err := stash.GetOrCreateTag(client, cache, tagName, tagName)
	require.NoError(t, err)
	t.Logf("Created test tag: %s", tagID)

	// Add tag to gallery
	err = stash.AddTagToGallery(client, testGalleryID, tagID)
	require.NoError(t, err, "failed to add tag to gallery")
	t.Logf("Added tag to gallery")

	// Verify tag was added by fetching gallery
	updatedGallery, err := stash.GetGallery(client, testGalleryID)
	require.NoError(t, err)

	hasTag := false
	for _, tag := range updatedGallery.Tags {
		if tag.ID == tagID {
			hasTag = true
			break
		}
	}
	assert.True(t, hasTag, "gallery should have the tag")

	// Remove tag from gallery
	err = stash.RemoveTagFromGallery(client, testGalleryID, tagID)
	require.NoError(t, err, "failed to remove tag from gallery")
	t.Logf("Removed tag from gallery")

	// Verify tag was removed
	finalGallery, err := stash.GetGallery(client, testGalleryID)
	require.NoError(t, err)

	stillHasTag := false
	for _, tag := range finalGallery.Tags {
		if tag.ID == tagID {
			stillHasTag = true
			break
		}
	}
	assert.False(t, stillHasTag, "gallery should not have the tag anymore")
}

func TestStashIntegration_UpdateGalleryTags(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createTestGraphQLClient(env.StashURL)
	cache := stash.NewTagCache()

	// Find a gallery to test with
	galleries, _, err := stash.FindGalleries(client, nil, 1, 1)
	require.NoError(t, err)

	if len(galleries) == 0 {
		t.Skip("No galleries in Stash, skipping update gallery tags test")
	}

	testGallery := galleries[0]
	testGalleryID := testGallery.ID
	t.Logf("Testing with gallery ID: %s", testGalleryID)

	// Save original tags
	originalTagIDs := make([]graphql.ID, len(testGallery.Tags))
	for i, tag := range testGallery.Tags {
		originalTagIDs[i] = tag.ID
	}

	// Create multiple test tags
	tag1Name := "Compreface Gallery Tag 1"
	tag1ID, err := stash.GetOrCreateTag(client, cache, tag1Name, tag1Name)
	require.NoError(t, err)

	tag2Name := "Compreface Gallery Tag 2"
	tag2ID, err := stash.GetOrCreateTag(client, cache, tag2Name, tag2Name)
	require.NoError(t, err)

	t.Logf("Created test tags: %s, %s", tag1ID, tag2ID)

	// Update gallery with new tags (complete replacement)
	newTagIDs := []graphql.ID{tag1ID, tag2ID}
	err = stash.UpdateGalleryTags(client, testGalleryID, newTagIDs)
	require.NoError(t, err, "failed to update gallery tags")
	t.Logf("Updated gallery tags")

	// Verify tags were replaced
	updatedGallery, err := stash.GetGallery(client, testGalleryID)
	require.NoError(t, err)

	assert.Len(t, updatedGallery.Tags, 2, "gallery should have exactly 2 tags")

	hasTag1 := false
	hasTag2 := false
	for _, tag := range updatedGallery.Tags {
		if tag.ID == tag1ID {
			hasTag1 = true
		}
		if tag.ID == tag2ID {
			hasTag2 = true
		}
	}
	assert.True(t, hasTag1, "gallery should have tag1")
	assert.True(t, hasTag2, "gallery should have tag2")

	// Restore original tags
	err = stash.UpdateGalleryTags(client, testGalleryID, originalTagIDs)
	require.NoError(t, err, "failed to restore original tags")
	t.Logf("Restored original tags")
}
