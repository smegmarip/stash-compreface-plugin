//go:build integration
// +build integration

package integration_test

import (
	"testing"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
	"github.com/smegmarip/stash-compreface-plugin/tests/testutil"
	"github.com/stretchr/testify/require"
)

func createStashClient(url string) *graphql.Client {
	return stash.TestClient(url+"/graphql", nil)
}

// TestUpdateImage_WithPerformers tests the UpdateImage function with performer associations
// This test addresses the critical 422 bug discovered in production
func TestUpdateImage_WithPerformers(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createStashClient(env.StashURL)
	tagCache := stash.NewTagCache()

	// Step 1: Get a test image
	images, _, err := stash.FindImages(client, nil, 1, 1)
	if err != nil {
		t.Fatalf("Failed to find images: %v", err)
	}
	if len(images) == 0 {
		t.Skip("No images available for testing")
	}
	imageID := images[0].ID
	t.Logf("Using image ID: %s", imageID)

	// Step 2: Get or create test performers
	performers, _, err := stash.FindPerformers(client, nil, 1, 2)
	if err != nil {
		t.Fatalf("Failed to find performers: %v", err)
	}

	var performer1ID, performer2ID graphql.ID
	if len(performers) >= 2 {
		performer1ID = performers[0].ID
		performer2ID = performers[1].ID
		t.Logf("Using existing performers: %s, %s", performer1ID, performer2ID)
	} else {
		// Create test performers if not enough exist
		randomUser1, err := testutil.RandomUser(testutil.GenderFemale)
		require.NoError(t, err, "failed to create performer")
		subject1Age, err := stash.CaclulateAgeFromBirthday(randomUser1.Birthdate.Format("2006-01-02"))
		testSubject1 := stash.PerformerSubject{
			Name:    randomUser1.Name,
			Aliases: []string{"Test UpdateImage Performer 1"},
			Age:     subject1Age,
			Gender:  string(*randomUser1.Gender),
			Image:   randomUser1.URLs.List()[0],
		}
		performer1ID, err = stash.CreatePerformer(client, testSubject1)
		if err != nil {
			t.Fatalf("Failed to create performer 1: %v", err)
		}
		t.Logf("Created performer 1: %s", performer1ID)

		randomUser2, err := testutil.RandomUser(testutil.GenderFemale)
		require.NoError(t, err, "failed to create performer")
		subject2Age, err := stash.CaclulateAgeFromBirthday(randomUser2.Birthdate.Format("2006-01-02"))
		testSubject2 := stash.PerformerSubject{
			Name:    randomUser2.Name,
			Aliases: []string{"Test UpdateImage Performer 2"},
			Age:     subject2Age,
			Gender:  string(*randomUser2.Gender),
			Image:   randomUser2.URLs.List()[0],
		}
		performer2ID, err = stash.CreatePerformer(client, testSubject2)
		if err != nil {
			t.Fatalf("Failed to create performer 2: %v", err)
		}
		t.Logf("Created performer 2: %s", performer2ID)
	}

	// Step 3: Test UpdateImage with single performer
	t.Run("UpdateWithSinglePerformer", func(t *testing.T) {
		performerIDs := []string{string(performer1ID)}
		input := stash.ImageUpdateInput{
			ID:           string(imageID),
			PerformerIds: performerIDs,
		}
		err := stash.UpdateImage(client, imageID, input)
		if err != nil {
			t.Errorf("UpdateImage failed with single performer: %v", err)
		}

		// Verify update
		image, err := stash.GetImage(client, imageID)
		if err != nil {
			t.Fatalf("Failed to get image after update: %v", err)
		}

		if len(image.Performers) == 0 {
			t.Errorf("Image has no performers after update")
		} else {
			found := false
			for _, p := range image.Performers {
				if p.ID == performer1ID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Performer %s not found on image", performer1ID)
			} else {
				t.Logf("✓ Performer successfully associated with image")
			}
		}
	})

	// Step 4: Test UpdateImage with multiple performers
	t.Run("UpdateWithMultiplePerformers", func(t *testing.T) {
		performerIDs := []string{string(performer1ID), string(performer2ID)}
		input := stash.ImageUpdateInput{
			ID:           string(imageID),
			PerformerIds: performerIDs,
		}
		err := stash.UpdateImage(client, imageID, input)
		if err != nil {
			t.Errorf("UpdateImage failed with multiple performers: %v", err)
		}

		// Verify update
		image, err := stash.GetImage(client, imageID)
		if err != nil {
			t.Fatalf("Failed to get image after update: %v", err)
		}

		if len(image.Performers) < 2 {
			t.Errorf("Image has %d performers, expected at least 2", len(image.Performers))
		} else {
			t.Logf("✓ Multiple performers successfully associated")
		}
	})

	// Step 5: Test UpdateImage with tags AND performers
	t.Run("UpdateWithTagsAndPerformers", func(t *testing.T) {
		// Get or create a test tag
		tagID, err := stash.GetOrCreateTag(client, tagCache, "Test UpdateImage Tag", "Test UpdateImage Tag")
		if err != nil {
			t.Fatalf("Failed to create test tag: %v", err)
		}

		performerIDs := []string{string(performer1ID)}
		tagIDs := []string{string(tagID)}
		input := stash.ImageUpdateInput{
			ID:           string(imageID),
			PerformerIds: performerIDs,
			TagIds:       tagIDs,
		}

		err = stash.UpdateImage(client, imageID, input)
		if err != nil {
			t.Errorf("UpdateImage failed with tags and performers: %v", err)
		}

		// Verify update
		image, err := stash.GetImage(client, imageID)
		if err != nil {
			t.Fatalf("Failed to get image after update: %v", err)
		}

		// Check tags
		hasTag := false
		for _, tag := range image.Tags {
			if tag.ID == tagID {
				hasTag = true
				break
			}
		}
		if !hasTag {
			t.Errorf("Tag not found on image")
		} else {
			t.Logf("✓ Tag successfully added")
		}

		// Check performers
		hasPerformer := false
		for _, p := range image.Performers {
			if p.ID == performer1ID {
				hasPerformer = true
				break
			}
		}
		if !hasPerformer {
			t.Errorf("Performer not found on image")
		} else {
			t.Logf("✓ Performer successfully associated")
		}
	})

	// Step 6: Test UpdateImage with empty performers (clear performers)
	t.Run("UpdateWithEmptyPerformers", func(t *testing.T) {
		performerIDs := []string{}
		input := stash.ImageUpdateInput{
			ID:           string(imageID),
			PerformerIds: performerIDs,
		}
		err := stash.UpdateImage(client, imageID, input)
		if err != nil {
			t.Errorf("UpdateImage failed with empty performers: %v", err)
		}

		// Verify performers cleared
		image, err := stash.GetImage(client, imageID)
		if err != nil {
			t.Fatalf("Failed to get image after update: %v", err)
		}

		if len(image.Performers) > 0 {
			t.Errorf("Image still has %d performers after clearing", len(image.Performers))
		} else {
			t.Logf("✓ Performers successfully cleared")
		}
	})
}

// TestUpdateImage_EdgeCases tests edge cases and error scenarios
func TestUpdateImage_EdgeCases(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createStashClient(env.StashURL)

	// Test 1: Non-existent image ID (should fail gracefully)
	t.Run("NonExistentImage", func(t *testing.T) {
		fakeImageID := graphql.ID("999999")
		performerIDs := []string{}
		input := stash.ImageUpdateInput{
			ID:           string(fakeImageID),
			PerformerIds: performerIDs,
		}
		err := stash.UpdateImage(client, fakeImageID, input)
		if err == nil {
			t.Logf("Warning: UpdateImage succeeded with non-existent ID (may create placeholder)")
		}
	})

	// Test 2: Invalid performer ID (should handle gracefully)
	t.Run("InvalidPerformerID", func(t *testing.T) {
		images, _, err := stash.FindImages(client, nil, 1, 1)
		if err != nil || len(images) == 0 {
			t.Skip("No images available")
		}

		imageID := images[0].ID
		fakePerformerID := graphql.ID("999999")
		performerIDs := []string{string(fakePerformerID)}
		input := stash.ImageUpdateInput{
			ID:           string(imageID),
			PerformerIds: performerIDs,
		}
		err = stash.UpdateImage(client, imageID, input)
		if err == nil {
			t.Logf("Warning: UpdateImage accepted invalid performer ID")
		}
	})

	// Test 3: Nil parameters (should handle gracefully)
	t.Run("NilParameters", func(t *testing.T) {
		images, _, err := stash.FindImages(client, nil, 1, 1)
		if err != nil || len(images) == 0 {
			t.Skip("No images available")
		}

		imageID := images[0].ID
		input := stash.ImageUpdateInput{
			ID: string(imageID),
		}

		err = stash.UpdateImage(client, imageID, input)
		if err != nil {
			t.Errorf("UpdateImage failed with nil parameters: %v", err)
		}
	})
}

// TestUpdateImage_422Regression specifically tests the 422 bug scenario
func TestUpdateImage_422Regression(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createStashClient(env.StashURL)

	// Get test data
	images, _, err := stash.FindImages(client, nil, 1, 1)
	if err != nil || len(images) == 0 {
		t.Skip("No images available")
	}
	imageID := images[0].ID

	performers, _, err := stash.FindPerformers(client, nil, 1, 1)
	if err != nil || len(performers) == 0 {
		t.Skip("No performers available")
	}
	performerID := performers[0].ID

	// This exact scenario caused 422 errors before the fix
	t.Run("ExactBugScenario", func(t *testing.T) {
		t.Logf("Testing exact scenario that caused 422 bug...")
		t.Logf("Image ID: %s, Performer ID: %s", imageID, performerID)

		// Get existing state
		image, err := stash.GetImage(client, imageID)
		if err != nil {
			t.Fatalf("Failed to get image: %v", err)
		}

		existingPerformerIDs := make([]string, len(image.Performers))
		for i, p := range image.Performers {
			existingPerformerIDs[i] = string(p.ID)
		}

		// Merge with new performer (this was failing with 422)
		allPerformerIDs := append(existingPerformerIDs, string(performerID))
		input := stash.ImageUpdateInput{
			ID:           string(imageID),
			PerformerIds: allPerformerIDs,
		}

		err = stash.UpdateImage(client, imageID, input)
		if err != nil {
			t.Fatalf("UpdateImage failed with 422-like scenario: %v", err)
		}

		// Verify the update succeeded
		updatedImage, err := stash.GetImage(client, imageID)
		if err != nil {
			t.Fatalf("Failed to get updated image: %v", err)
		}

		found := false
		for _, p := range updatedImage.Performers {
			if p.ID == performerID {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("422 regression: Performer was not associated despite no error")
		} else {
			t.Logf("✓ 422 bug scenario resolved - performer successfully associated")
		}
	})
}
