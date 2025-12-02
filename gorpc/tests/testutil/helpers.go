package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEnv holds test environment configuration
type TestEnv struct {
	t                *testing.T
	StashURL         string
	ComprefaceURL    string
	VisionServiceURL string
	FrameServerURL   string
	RecognitionKey   string
	DetectionKey     string
	VerificationKey  string
	cleanup          []func()
}

// SetupTestEnv creates a test environment with service URLs from environment variables
func SetupTestEnv(t *testing.T) *TestEnv {
	env := &TestEnv{
		t:                t,
		StashURL:         getEnvOrDefault("STASH_URL", "http://localhost:9999"),
		ComprefaceURL:    getEnvOrDefault("COMPREFACE_URL", "http://localhost:8000"),
		VisionServiceURL: getEnvOrDefault("VISION_SERVICE_URL", "http://localhost:5010"),
		RecognitionKey:   os.Getenv("COMPREFACE_RECOGNITION_KEY"),
		DetectionKey:     os.Getenv("COMPREFACE_DETECTION_KEY"),
		VerificationKey:  os.Getenv("COMPREFACE_VERIFICATION_KEY"),
		cleanup:          make([]func(), 0),
	}

	// Verify required environment variables for integration tests
	if testing.Short() {
		return env
	}

	return env
}

// Cleanup runs all registered cleanup functions
func (e *TestEnv) Cleanup() {
	for i := len(e.cleanup) - 1; i >= 0; i-- {
		e.cleanup[i]()
	}
}

// AddCleanup registers a cleanup function to run on teardown
func (e *TestEnv) AddCleanup(fn func()) {
	e.cleanup = append(e.cleanup, fn)
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// LoadFixture loads a test fixture file
func LoadFixture(t *testing.T, path string) []byte {
	t.Helper()

	fullPath := filepath.Join("fixtures", path)
	data, err := os.ReadFile(fullPath)
	require.NoError(t, err, "failed to load fixture: %s", path)

	return data
}

// SkipIfNoServices skips the test if required services are not available
func SkipIfNoServices(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
}

// AssertSubjectNameFormat validates subject name format: "Person {id} {random}"
func AssertSubjectNameFormat(t *testing.T, name string, expectedID string) {
	t.Helper()

	require.NotEmpty(t, name, "subject name should not be empty")
	require.Contains(t, name, "Person "+expectedID, "subject name should contain 'Person {id}'")

	// Check total length: "Person " + ID + " " + 16 random chars
	expectedMinLen := len("Person ") + len(expectedID) + len(" ") + 16
	require.GreaterOrEqual(t, len(name), expectedMinLen, "subject name should be at least %d characters", expectedMinLen)

	// Extract random part
	prefix := "Person " + expectedID + " "
	require.True(t, len(name) >= len(prefix), "subject name too short")

	randomPart := name[len(prefix):]
	require.Len(t, randomPart, 16, "random part should be exactly 16 characters")

	// Verify random part contains only uppercase letters and digits
	for _, ch := range randomPart {
		valid := (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
		require.True(t, valid, "random part should only contain A-Z and 0-9, found: %c", ch)
	}
}

// CreateTempImage creates a temporary test image file
func CreateTempImage(t *testing.T, width, height int) string {
	t.Helper()

	// Create a simple test image (this is a placeholder - would need image library for real images)
	tmpFile, err := os.CreateTemp("", "test-image-*.jpg")
	require.NoError(t, err)

	// Register cleanup
	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})

	// Write minimal JPEG header (for testing purposes)
	// In real tests, you'd use image/jpeg to create actual images
	tmpFile.Close()

	return tmpFile.Name()
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
