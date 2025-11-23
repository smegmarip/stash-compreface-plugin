package vision_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smegmarip/stash-compreface-plugin/internal/vision"
)

func getParams(useSprites bool, spriteVTT, spriteImage string) vision.FacesParameters {
	minConfidence := 0.7
	minQuality := 0.2
	qualityTrigger := 0.5

	enhancementParams := vision.EnhancementParameters{
		Enabled:        true,
		QualityTrigger: qualityTrigger,
		Model:          "codeformer",
		FidelityWeight: 0.25,
	}

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

func TestBuildAnalyzeRequest(t *testing.T) {
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
		{
			name:       "Scene frames mode with sprite paths",
			videoPath:  "/path/to/video.mp4",
			sceneID:    "scene789",
			parameters: getParams(false, "/path/to/sprite.vtt", "/path/to/sprite.jpg"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := vision.BuildAnalyzeRequest(
				tt.videoPath,
				tt.sceneID,
				tt.parameters,
			)

			assert.Equal(t, tt.videoPath, req.Source, "source should match")
			assert.Equal(t, tt.sceneID, req.SceneID, "scene ID should match")

			// Verify modules are configured
			assert.True(t, req.Modules.Faces.Enabled, "faces module should be enabled")
		})
	}
}

func TestBuildAnalyzeRequest_EmptyPaths(t *testing.T) {
	parameters := getParams(false, "", "")
	req := vision.BuildAnalyzeRequest("", "", parameters)

	assert.Empty(t, req.Source, "source should be empty")
	assert.Empty(t, req.SceneID, "scene ID should be empty")
	assert.True(t, req.Modules.Faces.Enabled, "faces module should be enabled")
}

func TestBuildAnalyzeRequest_LongPaths(t *testing.T) {
	longPath := "/very/long/path/to/video/file/with/many/directories/and/subdirectories/video.mp4"
	longSceneID := "scene-with-very-long-identifier-12345678901234567890"

	parameters := getParams(false, "", "")
	req := vision.BuildAnalyzeRequest(longPath, longSceneID, parameters)

	assert.Equal(t, longPath, req.Source, "should handle long paths")
	assert.Equal(t, longSceneID, req.SceneID, "should handle long scene IDs")
}

func TestBuildAnalyzeRequest_SpecialCharacters(t *testing.T) {
	videoPath := "/path/with spaces/and-dashes/file.mp4"
	sceneID := "scene_123-456"
	spriteVTT := "/path/to/sprite (1).vtt"
	spriteImage := "/path/to/sprite [thumb].jpg"
	parameters := getParams(true, spriteVTT, spriteImage)

	req := vision.BuildAnalyzeRequest(videoPath, sceneID, parameters)

	assert.Equal(t, videoPath, req.Source, "should handle spaces in path")
	assert.Equal(t, sceneID, req.SceneID, "should handle mixed characters")
	assert.True(t, req.Modules.Faces.Enabled, "faces module should be enabled")
}

func TestNewVisionServiceClient(t *testing.T) {
	baseURL := "http://localhost:5010"
	client := vision.NewVisionServiceClient(baseURL)

	assert.NotNil(t, client, "client should not be nil")
}

// Note: Most vision package functions (SubmitJob, GetJobStatus, GetResults,
// WaitForCompletion, HealthCheck, ExtractFrame) require HTTP client and
// are tested in integration tests. This unit test file focuses on functions
// that can be tested without external dependencies.
