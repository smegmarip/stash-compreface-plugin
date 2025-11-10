// +build integration

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smegmarip/stash-compreface-plugin/internal/quality"
	"github.com/smegmarip/stash-compreface-plugin/tests/testutil"
)

func TestQuality_FaceFilter(t *testing.T) {
	// Test the FaceFilter with different policies
	policies := []struct {
		name   string
		policy quality.AcceptancePolicy
	}{
		{"Strict", quality.PolicyStrict},
		{"Balanced", quality.PolicyBalanced},
		{"Permissive", quality.PolicyPermissive},
	}

	for _, p := range policies {
		t.Run(p.name, func(t *testing.T) {
			filter := quality.NewFaceFilter(p.policy)
			require.NotNil(t, filter)
			assert.Equal(t, p.policy, filter.GetPolicy())

			// Test policy can be changed
			filter.SetPolicy(quality.PolicyBalanced)
			assert.Equal(t, quality.PolicyBalanced, filter.GetPolicy())
		})
	}
}

func TestQuality_FaceFilterByName(t *testing.T) {
	tests := []struct {
		name     string
		policy   string
		expected quality.AcceptancePolicy
	}{
		{"strict", "strict", quality.PolicyStrict},
		{"balanced", "balanced", quality.PolicyBalanced},
		{"permissive", "permissive", quality.PolicyPermissive},
		{"STRICT uppercase", "STRICT", quality.PolicyStrict},
		{"unknown defaults to balanced", "unknown", quality.PolicyBalanced},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := quality.NewFaceFilterByName(tt.policy)
			require.NotNil(t, filter)
			assert.Equal(t, tt.expected, filter.GetPolicy())
		})
	}
}

func TestQuality_NewPythonClient(t *testing.T) {
	testutil.SkipIfNoServices(t)

	env := testutil.SetupTestEnv(t)
	defer env.Cleanup()

	if env.QualityServiceURL == "" {
		t.Skip("Quality service URL not configured")
	}

	client := quality.NewPythonClient(env.QualityServiceURL)
	require.NotNil(t, client)

	t.Logf("Created Python quality client for %s", env.QualityServiceURL)

	// Note: Actual API calls require the service to be running
	// and are more suitable for E2E tests
}

// Note: Quality router and detector initialization require specific
// configuration (models directory, service URLs) and are better tested
// in E2E tests with full plugin configuration. These integration tests
// focus on the components that can be tested in isolation.
