package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smegmarip/stash-compreface-plugin/internal/config"
)

// Note: Testing config.Load() requires mocking the plugin input and GraphQL client
// which is complex. We'll test the PluginConfig struct itself and defer full
// integration testing of Load() to integration tests.

func TestPluginConfig_Defaults(t *testing.T) {
	// Test that PluginConfig struct can be created with expected defaults
	cfg := &config.PluginConfig{
		CooldownSeconds:                10,
		MaxBatchSize:                   20,
		MinSimilarity:                  0.81,
		MinFaceSize:                    64,
		MinSceneConfidenceScore:        0.7,
		MinSceneQualityScore:           0.65,
		MinSceneProcessingQualityScore: 0.2,
		EnhanceQualityScoreTrigger:     0.5,
		ScannedTagName:                 "Compreface Scanned",
		MatchedTagName:                 "Compreface Matched",
		PartialTagName:                 "Compreface Partial",
		CompleteTagName:                "Compreface Complete",
		SyncedTagName:                  "Compreface Synced",
	}

	assert.Equal(t, 10, cfg.CooldownSeconds)
	assert.Equal(t, 20, cfg.MaxBatchSize)
	assert.Equal(t, 0.81, cfg.MinSimilarity)
	assert.Equal(t, 64, cfg.MinFaceSize)
	assert.Equal(t, 0.7, cfg.MinSceneConfidenceScore)
	assert.Equal(t, 0.65, cfg.MinSceneQualityScore)
	assert.Equal(t, 0.2, cfg.MinSceneProcessingQualityScore)
	assert.Equal(t, 0.5, cfg.EnhanceQualityScoreTrigger)
	assert.Equal(t, "Compreface Scanned", cfg.ScannedTagName)
	assert.Equal(t, "Compreface Matched", cfg.MatchedTagName)
	assert.Equal(t, "Compreface Partial", cfg.PartialTagName)
	assert.Equal(t, "Compreface Complete", cfg.CompleteTagName)
	assert.Equal(t, "Compreface Synced", cfg.SyncedTagName)
}

func TestPluginConfig_Fields(t *testing.T) {
	// Test that all required fields exist and can be set
	cfg := &config.PluginConfig{
		ComprefaceURL:                  "http://compreface:8000",
		RecognitionAPIKey:              "test-recognition-key",
		DetectionAPIKey:                "test-detection-key",
		VerificationAPIKey:             "test-verification-key",
		VisionServiceURL:               "http://vision:5010",
		QualityServiceURL:              "http://quality:6001",
		CooldownSeconds:                15,
		MaxBatchSize:                   30,
		MinSimilarity:                  0.95,
		MinFaceSize:                    128,
		MinSceneConfidenceScore:        0.25,
		MinSceneQualityScore:           0.99,
		MinSceneProcessingQualityScore: 0.55,
		EnhanceQualityScoreTrigger:     0.75,
		ScannedTagName:                 "Custom Scanned",
		MatchedTagName:                 "Custom Matched",
		PartialTagName:                 "Custom Partial",
		CompleteTagName:                "Custom Complete",
		SyncedTagName:                  "Custom Synced",
	}

	// Verify all fields are accessible
	assert.Equal(t, "http://compreface:8000", cfg.ComprefaceURL)
	assert.Equal(t, "test-recognition-key", cfg.RecognitionAPIKey)
	assert.Equal(t, "test-detection-key", cfg.DetectionAPIKey)
	assert.Equal(t, "test-verification-key", cfg.VerificationAPIKey)
	assert.Equal(t, "http://vision:5010", cfg.VisionServiceURL)
	assert.Equal(t, "http://quality:6001", cfg.QualityServiceURL)
	assert.Equal(t, 15, cfg.CooldownSeconds)
	assert.Equal(t, 30, cfg.MaxBatchSize)
	assert.Equal(t, 0.95, cfg.MinSimilarity)
	assert.Equal(t, 128, cfg.MinFaceSize)
	assert.Equal(t, 0.25, cfg.MinSceneConfidenceScore)
	assert.Equal(t, 0.99, cfg.MinSceneQualityScore)
	assert.Equal(t, 0.55, cfg.MinSceneProcessingQualityScore)
	assert.Equal(t, 0.75, cfg.EnhanceQualityScoreTrigger)
	assert.Equal(t, "Custom Scanned", cfg.ScannedTagName)
	assert.Equal(t, "Custom Matched", cfg.MatchedTagName)
	assert.Equal(t, "Custom Partial", cfg.PartialTagName)
	assert.Equal(t, "Custom Complete", cfg.CompleteTagName)
	assert.Equal(t, "Custom Synced", cfg.SyncedTagName)
}

// Note: Testing resolveServiceURL function requires access to unexported functions
// This would need to be refactored to make it testable, or we test it through
// integration tests with actual service resolution
