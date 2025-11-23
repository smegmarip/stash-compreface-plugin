//go:build integration
// +build integration

package integration_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smegmarip/stash-compreface-plugin/internal/vision"
	"github.com/smegmarip/stash-compreface-plugin/tests/testutil"
)

func createVisionServiceClient(t *testing.T) *vision.VisionServiceClient {
	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	if env.VisionServiceURL == "" {
		t.Skip("Vision service URL not configured")
	} else {
		t.Logf("Using Vision service URL: %s", env.VisionServiceURL)
		// Check if service is available
		if !vision.IsVisionServiceAvailable(env.VisionServiceURL) {
			t.Skipf("Vision service not available at %s", env.VisionServiceURL)
		}
	}

	client := vision.NewVisionServiceClient(env.VisionServiceURL)
	require.NotNil(t, client)

	return client
}

func TestVisionIntegration_HealthCheck(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createVisionServiceClient(t)
	require.NotNil(t, client)

	err := client.HealthCheck()
	if err != nil {
		t.Skipf("Vision service not available at %s: %v", env.VisionServiceURL, err)
	}

	t.Logf("Vision service is healthy at %s", env.VisionServiceURL)
}

func TestVisionIntegration_IsVisionServiceAvailable(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	if env.VisionServiceURL == "" {
		t.Skip("Vision service URL not configured")
	}

	available := vision.IsVisionServiceAvailable(env.VisionServiceURL)
	if !available {
		t.Skipf("Vision service not available at %s", env.VisionServiceURL)
	}

	assert.True(t, available)
	t.Logf("Vision service is available at %s", env.VisionServiceURL)
}

func TestVisionIntegration_SubmitAndCheckJob(t *testing.T) {
	testutil.SkipIfNoServices(t)

	// NOTE: The Vision Service has /Users/x/dev/resources/repo/stash-auto-vision/tests/data
	// mounted to /media/videos inside the container. We need to use the container path.
	// The Charades test videos are in tests/data/charades/dataset/
	testVideoContainerPath := "/media/videos/charades/dataset/001YG.mp4"

	// Verify the video exists in the host filesystem (for reference)
	testVideoHostPath := "/Users/x/dev/resources/repo/stash-auto-vision/tests/data/charades/dataset/001YG.mp4"
	if _, err := os.Stat(testVideoHostPath); os.IsNotExist(err) {
		t.Skipf("Test video not found at host path: %s", testVideoHostPath)
	}

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	client := createVisionServiceClient(t)

	minConfidence := 0.7
	minQuality := 0.2
	qualityTrigger := 0.5

	enhancementParams := vision.EnhancementParameters{
		Enabled:        true,
		QualityTrigger: qualityTrigger,
		Model:          "codeformer",
		FidelityWeight: 0.25,
	}

	parameters := vision.FacesParameters{
		FaceMinConfidence:            minConfidence, // Mid-High confidence detections only
		FaceMinQuality:               minQuality,    // Minimum quality threshold
		MaxFaces:                     50,            // Maximum unique faces to extract
		SamplingInterval:             2.0,           // Sample every 2 seconds initially
		UseSprites:                   false,
		SpriteVTTURL:                 "",
		SpriteImageURL:               "",
		EnableDeduplication:          true,               // De-duplicate faces across video
		EmbeddingSimilarityThreshold: 0.6,                // Cosine similarity threshold for clustering
		DetectDemographics:           true,               // Detect age, gender, emotion
		CacheDuration:                3600,               // Cache for 1 hour
		Enhancement:                  &enhancementParams, // Enable face enhancement
	}

	// Build analyze request using the container path
	req := vision.BuildAnalyzeRequest(testVideoContainerPath, "test-scene-123", parameters)
	assert.Equal(t, testVideoContainerPath, req.Source)
	assert.Equal(t, "test-scene-123", req.SceneID)
	assert.True(t, req.Modules.Faces.Enabled)

	// Submit job
	jobResp, err := client.SubmitJob(req)
	require.NoError(t, err, "failed to submit job")
	require.NotNil(t, jobResp)
	require.NotEmpty(t, jobResp.JobID, "job ID should not be empty")

	t.Logf("Submitted job: %s", jobResp.JobID)

	// Check job status
	status, err := client.GetJobStatus(jobResp.JobID)
	require.NoError(t, err, "failed to get job status")
	require.NotNil(t, status)

	t.Logf("Job status: %s, progress: %.2f", status.Status, status.Progress)

	// Don't wait for completion in this test - that could take a while
	// Just verify we can submit and check status
}

func TestVisionIntegration_BuildAnalyzeRequest(t *testing.T) {
	minConfidence := 0.7
	minQuality := 0.2
	qualityTrigger := 0.5

	enhancementParams := vision.EnhancementParameters{
		Enabled:        true,
		QualityTrigger: qualityTrigger,
		Model:          "codeformer",
		FidelityWeight: 0.25,
	}

	getParams := func(useSprites bool, spriteVTT, spriteImage string) vision.FacesParameters {
		return vision.FacesParameters{
			FaceMinConfidence:            minConfidence, // Mid-High confidence detections only
			FaceMinQuality:               minQuality,    // Minimum quality threshold
			MaxFaces:                     50,            // Maximum unique faces to extract
			SamplingInterval:             2.0,           // Sample every 2 seconds initially
			UseSprites:                   useSprites,
			SpriteVTTURL:                 spriteVTT,
			SpriteImageURL:               spriteImage,
			EnableDeduplication:          true,               // De-duplicate faces across video
			EmbeddingSimilarityThreshold: 0.6,                // Cosine similarity threshold for clustering
			DetectDemographics:           true,               // Detect age, gender, emotion
			CacheDuration:                3600,               // Cache for 1 hour
			Enhancement:                  &enhancementParams, // Enable face enhancement
		}
	}

	// This is pure logic but test it here to verify the structure
	tests := []struct {
		name       string
		videoPath  string
		sceneID    string
		parameters vision.FacesParameters
	}{
		{
			name:       "Scene frames mode",
			videoPath:  "/path/to/video.mp4",
			sceneID:    "scene123",
			parameters: getParams(false, "", ""),
		},
		{
			name:       "Sprites mode",
			videoPath:  "/path/to/video.mp4",
			sceneID:    "scene456",
			parameters: getParams(true, "/path/to/sprite.vtt", "/path/to/sprite.jpg"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := vision.BuildAnalyzeRequest(
				tt.videoPath,
				tt.sceneID,
				tt.parameters,
			)

			assert.Equal(t, tt.videoPath, req.Source)
			assert.Equal(t, tt.sceneID, req.SceneID)
			assert.True(t, req.Modules.Faces.Enabled)
		})
	}
}

// Note: Full workflow tests (submit → wait → get results) are more suitable
// for E2E tests as they require actual video files and can take significant time.
