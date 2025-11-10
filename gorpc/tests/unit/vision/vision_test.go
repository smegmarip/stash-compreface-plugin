package vision_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smegmarip/stash-compreface-plugin/internal/vision"
)

func TestBuildAnalyzeRequest(t *testing.T) {
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
		{
			name:        "Scene frames mode with sprite paths",
			videoPath:   "/path/to/video.mp4",
			sceneID:     "scene789",
			useSprites:  false,
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

			assert.Equal(t, tt.videoPath, req.VideoPath, "video path should match")
			assert.Equal(t, tt.sceneID, req.SceneID, "scene ID should match")

			// Verify modules are configured
			assert.True(t, req.Modules.Faces.Enabled, "faces module should be enabled")
		})
	}
}

func TestBuildAnalyzeRequest_EmptyPaths(t *testing.T) {
	req := vision.BuildAnalyzeRequest("", "", false, "", "")

	assert.Empty(t, req.VideoPath, "video path should be empty")
	assert.Empty(t, req.SceneID, "scene ID should be empty")
	assert.True(t, req.Modules.Faces.Enabled, "faces module should be enabled")
}

func TestBuildAnalyzeRequest_LongPaths(t *testing.T) {
	longPath := "/very/long/path/to/video/file/with/many/directories/and/subdirectories/video.mp4"
	longSceneID := "scene-with-very-long-identifier-12345678901234567890"

	req := vision.BuildAnalyzeRequest(longPath, longSceneID, false, "", "")

	assert.Equal(t, longPath, req.VideoPath, "should handle long paths")
	assert.Equal(t, longSceneID, req.SceneID, "should handle long scene IDs")
}

func TestBuildAnalyzeRequest_SpecialCharacters(t *testing.T) {
	videoPath := "/path/with spaces/and-dashes/file.mp4"
	sceneID := "scene_123-456"
	spriteVTT := "/path/to/sprite (1).vtt"
	spriteImage := "/path/to/sprite [thumb].jpg"

	req := vision.BuildAnalyzeRequest(videoPath, sceneID, true, spriteVTT, spriteImage)

	assert.Equal(t, videoPath, req.VideoPath, "should handle spaces in path")
	assert.Equal(t, sceneID, req.SceneID, "should handle mixed characters")
	assert.True(t, req.Modules.Faces.Enabled, "faces module should be enabled")
}

func TestNewVisionServiceClient(t *testing.T) {
	baseURL := "http://localhost:5000"
	client := vision.NewVisionServiceClient(baseURL)

	assert.NotNil(t, client, "client should not be nil")
}

// Note: Most vision package functions (SubmitJob, GetJobStatus, GetResults,
// WaitForCompletion, HealthCheck, ExtractFrame) require HTTP client and
// are tested in integration tests. This unit test file focuses on functions
// that can be tested without external dependencies.
