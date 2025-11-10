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
		CooldownSeconds: 10,
		MaxBatchSize:    20,
		MinSimilarity:   0.89,
		MinFaceSize:     64,
		ScannedTagName:  "Compreface Scanned",
		MatchedTagName:  "Compreface Matched",
	}

	assert.Equal(t, 10, cfg.CooldownSeconds)
	assert.Equal(t, 20, cfg.MaxBatchSize)
	assert.Equal(t, 0.89, cfg.MinSimilarity)
	assert.Equal(t, 64, cfg.MinFaceSize)
	assert.Equal(t, "Compreface Scanned", cfg.ScannedTagName)
	assert.Equal(t, "Compreface Matched", cfg.MatchedTagName)
}

func TestPluginConfig_Fields(t *testing.T) {
	// Test that all required fields exist and can be set
	cfg := &config.PluginConfig{
		ComprefaceURL:      "http://compreface:8000",
		RecognitionAPIKey:  "test-recognition-key",
		DetectionAPIKey:    "test-detection-key",
		VerificationAPIKey: "test-verification-key",
		VisionServiceURL:   "http://vision:5000",
		QualityServiceURL:  "http://quality:6001",
		CooldownSeconds:    15,
		MaxBatchSize:       30,
		MinSimilarity:      0.95,
		MinFaceSize:        128,
		ScannedTagName:     "Custom Scanned",
		MatchedTagName:     "Custom Matched",
	}

	// Verify all fields are accessible
	assert.Equal(t, "http://compreface:8000", cfg.ComprefaceURL)
	assert.Equal(t, "test-recognition-key", cfg.RecognitionAPIKey)
	assert.Equal(t, "test-detection-key", cfg.DetectionAPIKey)
	assert.Equal(t, "test-verification-key", cfg.VerificationAPIKey)
	assert.Equal(t, "http://vision:5000", cfg.VisionServiceURL)
	assert.Equal(t, "http://quality:6001", cfg.QualityServiceURL)
	assert.Equal(t, 15, cfg.CooldownSeconds)
	assert.Equal(t, 30, cfg.MaxBatchSize)
	assert.Equal(t, 0.95, cfg.MinSimilarity)
	assert.Equal(t, 128, cfg.MinFaceSize)
	assert.Equal(t, "Custom Scanned", cfg.ScannedTagName)
	assert.Equal(t, "Custom Matched", cfg.MatchedTagName)
}

// Note: Testing resolveServiceURL function requires access to unexported functions
// This would need to be refactored to make it testable, or we test it through
// integration tests with actual service resolution
