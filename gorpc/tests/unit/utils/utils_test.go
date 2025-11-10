package utils_test

import (
	"testing"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stretchr/testify/assert"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/pkg/utils"
)

func TestGetFaceDimensions(t *testing.T) {
	tests := []struct {
		name           string
		box            compreface.BoundingBox
		expectedWidth  int
		expectedHeight int
	}{
		{
			name: "Standard face box",
			box: compreface.BoundingBox{
				XMin: 100,
				YMin: 150,
				XMax: 300,
				YMax: 400,
			},
			expectedWidth:  200,
			expectedHeight: 250,
		},
		{
			name: "Small face box",
			box: compreface.BoundingBox{
				XMin: 50,
				YMin: 50,
				XMax: 114,
				YMax: 114,
			},
			expectedWidth:  64,
			expectedHeight: 64,
		},
		{
			name: "Large face box",
			box: compreface.BoundingBox{
				XMin: 0,
				YMin: 0,
				XMax: 800,
				YMax: 1000,
			},
			expectedWidth:  800,
			expectedHeight: 1000,
		},
		{
			name: "Zero-sized box (edge case)",
			box: compreface.BoundingBox{
				XMin: 100,
				YMin: 100,
				XMax: 100,
				YMax: 100,
			},
			expectedWidth:  0,
			expectedHeight: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, height := utils.GetFaceDimensions(tt.box)
			assert.Equal(t, tt.expectedWidth, width, "width mismatch")
			assert.Equal(t, tt.expectedHeight, height, "height mismatch")
		})
	}
}

func TestIsFaceSizeValid(t *testing.T) {
	tests := []struct {
		name     string
		box      compreface.BoundingBox
		minSize  int
		expected bool
	}{
		{
			name: "Valid face - meets minimum exactly",
			box: compreface.BoundingBox{
				XMin: 0,
				YMin: 0,
				XMax: 64,
				YMax: 64,
			},
			minSize:  64,
			expected: true,
		},
		{
			name: "Valid face - exceeds minimum",
			box: compreface.BoundingBox{
				XMin: 100,
				YMin: 100,
				XMax: 300,
				YMax: 300,
			},
			minSize:  64,
			expected: true,
		},
		{
			name: "Invalid face - width too small",
			box: compreface.BoundingBox{
				XMin: 0,
				YMin: 0,
				XMax: 50,
				YMax: 100,
			},
			minSize:  64,
			expected: false,
		},
		{
			name: "Invalid face - height too small",
			box: compreface.BoundingBox{
				XMin: 0,
				YMin: 0,
				XMax: 100,
				YMax: 50,
			},
			minSize:  64,
			expected: false,
		},
		{
			name: "Invalid face - both dimensions too small",
			box: compreface.BoundingBox{
				XMin: 0,
				YMin: 0,
				XMax: 30,
				YMax: 30,
			},
			minSize:  64,
			expected: false,
		},
		{
			name: "Edge case - zero minSize",
			box: compreface.BoundingBox{
				XMin: 0,
				YMin: 0,
				XMax: 10,
				YMax: 10,
			},
			minSize:  0,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.IsFaceSizeValid(tt.box, tt.minSize)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeduplicateIDs(t *testing.T) {
	tests := []struct {
		name     string
		input    []graphql.ID
		expected []graphql.ID
	}{
		{
			name:     "No duplicates",
			input:    []graphql.ID{"1", "2", "3"},
			expected: []graphql.ID{"1", "2", "3"},
		},
		{
			name:     "Some duplicates",
			input:    []graphql.ID{"1", "2", "2", "3", "1"},
			expected: []graphql.ID{"1", "2", "3"},
		},
		{
			name:     "All duplicates",
			input:    []graphql.ID{"1", "1", "1", "1"},
			expected: []graphql.ID{"1"},
		},
		{
			name:     "Empty slice",
			input:    []graphql.ID{},
			expected: []graphql.ID{},
		},
		{
			name:     "Single element",
			input:    []graphql.ID{"1"},
			expected: []graphql.ID{"1"},
		},
		{
			name:     "Preserves order of first occurrence",
			input:    []graphql.ID{"3", "1", "2", "1", "3"},
			expected: []graphql.ID{"3", "1", "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.DeduplicateIDs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
