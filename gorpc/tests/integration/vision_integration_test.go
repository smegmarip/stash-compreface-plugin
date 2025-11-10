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

func TestVisionIntegration_HealthCheck(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	if env.VisionServiceURL == "" {
		t.Skip("Vision service URL not configured")
	}

	client := vision.NewVisionServiceClient(env.VisionServiceURL)
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

	// Check if test video exists
	testVideoPath := "tests/fixtures/videos/test_video.mp4"
	if _, err := os.Stat(testVideoPath); os.IsNotExist(err) {
		t.Skip("Test video not found, skipping job submission test")
	}

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	if env.VisionServiceURL == "" {
		t.Skip("Vision service URL not configured")
	}

	client := vision.NewVisionServiceClient(env.VisionServiceURL)

	// Check if service is available
	if !vision.IsVisionServiceAvailable(env.VisionServiceURL) {
		t.Skipf("Vision service not available at %s", env.VisionServiceURL)
	}

	// Build analyze request
	req := vision.BuildAnalyzeRequest(testVideoPath, "test-scene-123", false, "", "")
	assert.Equal(t, testVideoPath, req.VideoPath)
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
	// This is pure logic but test it here to verify the structure
	tests := []struct {
		name        string
		videoPath   string
		sceneID     string
		useSprites  bool
		spriteVTT   string
		spriteImage string
	}{
		{
			name:        "Scene frames mode",
			videoPath:   "/path/to/video.mp4",
			sceneID:     "scene123",
			useSprites:  false,
			spriteVTT:   "",
			spriteImage: "",
		},
		{
			name:        "Sprites mode",
			videoPath:   "/path/to/video.mp4",
			sceneID:     "scene456",
			useSprites:  true,
			spriteVTT:   "/path/to/sprite.vtt",
			spriteImage: "/path/to/sprite.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := vision.BuildAnalyzeRequest(
				tt.videoPath,
				tt.sceneID,
				tt.useSprites,
				tt.spriteVTT,
				tt.spriteImage,
			)

			assert.Equal(t, tt.videoPath, req.VideoPath)
			assert.Equal(t, tt.sceneID, req.SceneID)
			assert.True(t, req.Modules.Faces.Enabled)
		})
	}
}

// Note: Full workflow tests (submit → wait → get results) are more suitable
// for E2E tests as they require actual video files and can take significant time.
