package compreface_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
	"github.com/smegmarip/stash-compreface-plugin/tests/testutil"
)

func TestCreateSubjectName(t *testing.T) {
	tests := []struct {
		name    string
		imageID string
	}{
		{
			name:    "Standard image ID",
			imageID: "12345",
		},
		{
			name:    "Long image ID",
			imageID: "9876543210",
		},
		{
			name:    "Single digit ID",
			imageID: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject := compreface.CreateSubjectName(tt.imageID)

			// Verify format: "Person {id} {16-char-random}"
			testutil.AssertSubjectNameFormat(t, subject, tt.imageID)

			// Verify uniqueness - calling twice should produce different results
			subject2 := compreface.CreateSubjectName(tt.imageID)
			assert.NotEqual(t, subject, subject2, "subject names should be unique due to random component")
		})
	}
}

func TestCreateSubjectName_Format(t *testing.T) {
	imageID := "test123"
	subject := compreface.CreateSubjectName(imageID)

	// Check prefix
	expectedPrefix := "Person test123 "
	require.True(t, strings.HasPrefix(subject, expectedPrefix),
		"subject should start with 'Person test123 ', got: %s", subject)

	// Check total length
	expectedLen := len(expectedPrefix) + 16
	assert.Equal(t, expectedLen, len(subject),
		"subject should be %d characters (prefix + 16 random)", expectedLen)

	// Verify random part contains only uppercase letters and digits
	randomPart := subject[len(expectedPrefix):]
	assert.Len(t, randomPart, 16, "random part should be exactly 16 characters")

	for _, ch := range randomPart {
		valid := (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
		assert.True(t, valid, "random part should only contain A-Z and 0-9, found: %c", ch)
	}
}

func TestFindPersonAlias_WithAliases(t *testing.T) {
	tests := []struct {
		name     string
		performer *stash.Performer
		expected string
	}{
		{
			name: "Alias matches Person pattern",
			performer: &stash.Performer{
				Name:      "Jane Doe",
				AliasList: []string{"Person 12345 ABC123XYZ456GHIJ", "Jane"},
			},
			expected: "Person 12345 ABC123XYZ456GHIJ",
		},
		{
			name: "Multiple aliases, first Person match",
			performer: &stash.Performer{
				Name:      "Jane Doe",
				AliasList: []string{"Jane", "Person 111 AAAA", "Person 222 BBBB"},
			},
			expected: "Person 111 AAAA",
		},
		{
			name: "Name is Person pattern",
			performer: &stash.Performer{
				Name:      "Person 999 XYZABC",
				AliasList: []string{},
			},
			expected: "Person 999 XYZABC",
		},
		{
			name: "No Person pattern found",
			performer: &stash.Performer{
				Name:      "Jane Doe",
				AliasList: []string{"Jane", "JD"},
			},
			expected: "",
		},
		{
			name: "Empty aliases",
			performer: &stash.Performer{
				Name:      "Jane Doe",
				AliasList: []string{},
			},
			expected: "",
		},
		{
			name: "Alias takes priority over name",
			performer: &stash.Performer{
				Name:      "Person 123 OLD",
				AliasList: []string{"Person 456 NEW"},
			},
			expected: "Person 456 NEW",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compreface.FindPersonAlias(tt.performer)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindPersonAlias_EdgeCases(t *testing.T) {
	t.Run("Nil performer", func(t *testing.T) {
		// This should probably panic or be handled, but test current behavior
		defer func() {
			if r := recover(); r != nil {
				// Expected - nil dereference
				t.Log("Function panics on nil performer (expected)")
			}
		}()
		compreface.FindPersonAlias(nil)
	})

	t.Run("Partial Person match", func(t *testing.T) {
		performer := &stash.Performer{
			Name:      "Jane Doe",
			AliasList: []string{"Person", "PersonA", "A Person"},
		}
		// "Person" alone matches the pattern ^Person .*$ (Person followed by space and anything)
		result := compreface.FindPersonAlias(performer)
		// The pattern is "^Person .*$" so "Person" without space doesn't match
		assert.Equal(t, "", result, "partial matches should not be found")
	})

	t.Run("Case sensitivity", func(t *testing.T) {
		performer := &stash.Performer{
			Name:      "Jane Doe",
			AliasList: []string{"person 123 ABC", "PERSON 456 DEF"},
		}
		// Pattern should be case-sensitive (^Person .*$)
		result := compreface.FindPersonAlias(performer)
		assert.Equal(t, "", result, "pattern should be case-sensitive")
	})
}

func TestCreateSubjectName_Uniqueness(t *testing.T) {
	// Generate multiple subject names and verify they're all unique
	imageID := "test"
	names := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		subject := compreface.CreateSubjectName(imageID)

		// Check it's not a duplicate
		assert.False(t, names[subject], "found duplicate subject name: %s", subject)
		names[subject] = true

		// Verify format
		testutil.AssertSubjectNameFormat(t, subject, imageID)
	}

	// Verify we generated exactly `iterations` unique names
	assert.Len(t, names, iterations, "should have generated %d unique names", iterations)
}
