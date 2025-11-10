// +build integration

package integration_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/tests/testutil"
)

func TestComprefaceIntegration_ListSubjects(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := compreface.NewClient(
		env.ComprefaceURL,
		env.RecognitionKey,
		env.DetectionKey,
		env.VerificationKey,
		0.89,
	)

	subjects, err := client.ListSubjects()
	require.NoError(t, err, "failed to list subjects")

	// Should return a list (may be empty)
	assert.NotNil(t, subjects)
	t.Logf("Found %d subjects in Compreface", len(subjects))

	for _, subject := range subjects {
		t.Logf("  - %s", subject)
	}
}

func TestComprefaceIntegration_DetectFaces(t *testing.T) {
	testutil.SkipIfNoServices(t)

	// Check if test image exists (PNG or JPG)
	// Note: path is relative to the test package directory (tests/integration/)
	testImagePath := "../fixtures/images/test_face.png"
	if _, err := os.Stat(testImagePath); os.IsNotExist(err) {
		// Try JPG if PNG doesn't exist
		testImagePath = "../fixtures/images/test_face.jpg"
		if _, err := os.Stat(testImagePath); os.IsNotExist(err) {
			t.Skip("Test image not found, skipping face detection test")
		}
	}

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := compreface.NewClient(
		env.ComprefaceURL,
		env.RecognitionKey,
		env.DetectionKey,
		env.VerificationKey,
		0.89,
	)

	result, err := client.DetectFaces(testImagePath)
	require.NoError(t, err, "face detection failed")
	require.NotNil(t, result)

	t.Logf("Detected %d faces", len(result.Result))

	for i, face := range result.Result {
		t.Logf("Face %d:", i)
		t.Logf("  Age: %d-%d", face.Age.Low, face.Age.High)
		t.Logf("  Gender: %s (%.2f)", face.Gender.Value, face.Gender.Probability)
		t.Logf("  Mask: %s (%.2f)", face.Mask.Value, face.Mask.Probability)
		t.Logf("  Box: (%d,%d) to (%d,%d)",
			face.Box.XMin, face.Box.YMin, face.Box.XMax, face.Box.YMax)
		t.Logf("  Embedding: %d dimensions", len(face.Embedding))
	}
}

func TestComprefaceIntegration_AddAndDeleteSubject(t *testing.T) {
	testutil.SkipIfNoServices(t)

	// Check if test image exists (PNG or JPG)
	testImagePath := "../fixtures/images/test_face.png"
	if _, err := os.Stat(testImagePath); os.IsNotExist(err) {
		testImagePath = "../fixtures/images/test_face.jpg"
		if _, err := os.Stat(testImagePath); os.IsNotExist(err) {
			t.Skip("Test image not found, skipping subject creation test")
		}
	}

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := compreface.NewClient(
		env.ComprefaceURL,
		env.RecognitionKey,
		env.DetectionKey,
		env.VerificationKey,
		0.89,
	)

	// Create a test subject
	subjectName := compreface.CreateSubjectName("integration-test")
	t.Logf("Creating subject: %s", subjectName)

	addResp, err := client.AddSubject(subjectName, testImagePath)
	require.NoError(t, err, "failed to add subject")
	require.NotNil(t, addResp)

	assert.Equal(t, subjectName, addResp.Subject)
	assert.NotEmpty(t, addResp.ImageID)
	t.Logf("Subject created with image ID: %s", addResp.ImageID)

	// Register cleanup to delete the test subject
	env.AddCleanup(func() {
		err := client.DeleteSubject(subjectName)
		if err != nil {
			t.Logf("Warning: failed to cleanup test subject %s: %v", subjectName, err)
		} else {
			t.Logf("Cleaned up test subject: %s", subjectName)
		}
	})

	// List subjects and verify our subject exists
	subjects, err := client.ListSubjects()
	require.NoError(t, err)
	assert.Contains(t, subjects, subjectName, "created subject should be in list")

	// List faces for the subject
	faces, err := client.ListFaces(subjectName)
	require.NoError(t, err)
	assert.Len(t, faces, 1, "should have one face")
	assert.Equal(t, addResp.ImageID, faces[0].ImageID)
}

func TestComprefaceIntegration_RecognizeFaces(t *testing.T) {
	testutil.SkipIfNoServices(t)

	// Check if test image exists (PNG or JPG)
	testImagePath := "../fixtures/images/test_face.png"
	if _, err := os.Stat(testImagePath); os.IsNotExist(err) {
		testImagePath = "../fixtures/images/test_face.jpg"
		if _, err := os.Stat(testImagePath); os.IsNotExist(err) {
			t.Skip("Test image not found, skipping recognition test")
		}
	}

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := compreface.NewClient(
		env.ComprefaceURL,
		env.RecognitionKey,
		env.DetectionKey,
		env.VerificationKey,
		0.89,
	)

	// First, add a subject
	subjectName := compreface.CreateSubjectName("recognition-test")
	t.Logf("Creating subject: %s", subjectName)

	_, err := client.AddSubject(subjectName, testImagePath)
	require.NoError(t, err)

	// Register cleanup
	env.AddCleanup(func() {
		client.DeleteSubject(subjectName)
	})

	// Try to recognize the same face (should match)
	recogResp, err := client.RecognizeFaces(testImagePath)
	require.NoError(t, err)
	require.NotNil(t, recogResp)

	t.Logf("Recognition results: %d faces detected", len(recogResp.Result))

	if len(recogResp.Result) > 0 {
		face := recogResp.Result[0]
		t.Logf("Face detected:")
		t.Logf("  Age: %d-%d", face.Age.Low, face.Age.High)
		t.Logf("  Gender: %s", face.Gender.Value)

		if len(face.Subjects) > 0 {
			for _, subject := range face.Subjects {
				t.Logf("  Match: %s (similarity: %.2f)", subject.Subject, subject.Similarity)
			}

			// Should find our subject with high similarity
			found := false
			for _, subject := range face.Subjects {
				if subject.Subject == subjectName {
					found = true
					assert.GreaterOrEqual(t, subject.Similarity, 0.89,
						"similarity should meet minimum threshold")
					break
				}
			}
			assert.True(t, found, "should recognize the same face")
		} else {
			t.Log("  No subjects matched (database might be empty)")
		}
	}
}
