package quality

import (
	"image"
)

// FaceDetection represents a detected face with quality metrics
type FaceDetection struct {
	Box        BoundingBox
	Confidence ConfidenceScore
	Landmarks  []image.Point // Facial landmarks
	Cropped    []byte        // Cropped and aligned face image
	Size       image.Point   // Width, Height of cropped face
	Masked     bool          // Whether the face is masked
}

// BoundingBox represents face coordinates in the image
type BoundingBox struct {
	XMin int `json:"x_min"`
	YMin int `json:"y_min"`
	XMax int `json:"x_max"`
	YMax int `json:"y_max"`
}

// ConfidenceScore represents face quality and pose information
type ConfidenceScore struct {
	Score   float64 `json:"score"`    // Quality score (higher is better)
	Type    string  `json:"type"`     // Pose type: front, left, right, front-rotate-left, front-rotate-right
	TypeRaw int     `json:"type_raw"` // Raw pose type value (0-4)
}

// PoseType represents the detected face orientation
type PoseType int

const (
	PoseFront PoseType = iota
	PoseLeft
	PoseRight
	PoseFrontRotateLeft
	PoseFrontRotateRight
)

// String converts PoseType to string representation
func (p PoseType) String() string {
	switch p {
	case PoseFront:
		return "front"
	case PoseLeft:
		return "left"
	case PoseRight:
		return "right"
	case PoseFrontRotateLeft:
		return "front-rotate-left"
	case PoseFrontRotateRight:
		return "front-rotate-right"
	default:
		return "n/a"
	}
}

// Width returns the bounding box width
func (b BoundingBox) Width() int {
	return b.XMax - b.XMin
}

// Height returns the bounding box height
func (b BoundingBox) Height() int {
	return b.YMax - b.YMin
}

// Center returns the center point of the bounding box
func (b BoundingBox) Center() image.Point {
	return image.Point{
		X: (b.XMin + b.XMax) / 2,
		Y: (b.YMin + b.YMax) / 2,
	}
}

// Area returns the area of the bounding box
func (b BoundingBox) Area() int {
	return b.Width() * b.Height()
}

// IoU calculates Intersection over Union with another bounding box
func (b BoundingBox) IoU(other BoundingBox) float64 {
	// Calculate intersection
	xMin := max(b.XMin, other.XMin)
	yMin := max(b.YMin, other.YMin)
	xMax := min(b.XMax, other.XMax)
	yMax := min(b.YMax, other.YMax)

	// No intersection
	if xMin >= xMax || yMin >= yMax {
		return 0.0
	}

	intersection := (xMax - xMin) * (yMax - yMin)
	union := b.Area() + other.Area() - intersection

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
