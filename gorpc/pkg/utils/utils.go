package utils

import (
	graphql "github.com/hasura/go-graphql-client"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
)

// ============================================================================
// Pure Utility Functions
// ============================================================================
//
// This file contains only domain-agnostic utility functions that can be
// used across any part of the application.
// ============================================================================

// GetFaceDimensions returns the width and height of a face bounding box
func GetFaceDimensions(box compreface.BoundingBox) (int, int) {
	width := box.XMax - box.XMin
	height := box.YMax - box.YMin
	return width, height
}

// IsFaceSizeValid checks if a face meets the minimum size requirement
func IsFaceSizeValid(box compreface.BoundingBox, minSize int) bool {
	width, height := GetFaceDimensions(box)
	return width >= minSize && height >= minSize
}

// DeduplicateIDs removes duplicate IDs from a slice
func DeduplicateIDs(ids []graphql.ID) []graphql.ID {
	seen := make(map[graphql.ID]bool)
	result := []graphql.ID{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}
