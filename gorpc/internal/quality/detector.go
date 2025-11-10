package quality

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"

	"github.com/Kagami/go-face"
)

// Detector handles face detection and quality assessment using dlib
type Detector struct {
	rec              *face.Recognizer
	modelsDir        string
	minConfidence    float64
	use68Landmarks   bool // Use 68-point landmarks instead of 5-point
	shape68Predictor string
}

// DetectorConfig holds configuration for the face detector
type DetectorConfig struct {
	ModelsDir      string  // Directory containing model files
	MinConfidence  float64 // Minimum confidence threshold (default: 1.0)
	Use68Landmarks bool    // Use 68-point landmarks (default: false, uses 5-point)
}

// NewDetector creates a new face detector instance
func NewDetector(config DetectorConfig) (*Detector, error) {
	if config.ModelsDir == "" {
		return nil, fmt.Errorf("models directory is required")
	}

	if config.MinConfidence == 0 {
		config.MinConfidence = 1.0
	}

	// Initialize go-face recognizer
	rec, err := face.NewRecognizer(config.ModelsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize face recognizer: %w", err)
	}

	d := &Detector{
		rec:            rec,
		modelsDir:      config.ModelsDir,
		minConfidence:  config.MinConfidence,
		use68Landmarks: config.Use68Landmarks,
	}

	// If 68-point landmarks requested, check for model file
	if config.Use68Landmarks {
		shape68Path := filepath.Join(config.ModelsDir, "shape_predictor_68_face_landmarks.dat")
		if _, err := os.Stat(shape68Path); err == nil {
			d.shape68Predictor = shape68Path
		} else {
			return nil, fmt.Errorf("68-point landmark model not found: %s", shape68Path)
		}
	}

	return d, nil
}

// Close releases resources used by the detector
func (d *Detector) Close() {
	if d.rec != nil {
		d.rec.Close()
	}
}

// DetectFile detects faces in an image file
func (d *Detector) DetectFile(imagePath string) ([]FaceDetection, error) {
	// Recognize faces using go-face
	faces, err := d.rec.RecognizeFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to recognize faces: %w", err)
	}

	// Load image for cropping
	img, err := loadImage(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load image: %w", err)
	}

	return d.processFaces(faces, img)
}

// DetectBytes detects faces in image bytes
func (d *Detector) DetectBytes(imageData []byte) ([]FaceDetection, error) {
	// For go-face, we need to save to temp file or use RecognizeSingleFile
	// This is a limitation of the go-face library
	tmpFile, err := os.CreateTemp("", "face-detect-*.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(imageData); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	return d.DetectFile(tmpFile.Name())
}

// processFaces converts go-face results to our FaceDetection format
func (d *Detector) processFaces(faces []face.Face, img image.Image) ([]FaceDetection, error) {
	detections := make([]FaceDetection, 0, len(faces))

	for _, f := range faces {
		// Convert rectangle to bounding box
		bbox := BoundingBox{
			XMin: f.Rectangle.Min.X,
			YMin: f.Rectangle.Min.Y,
			XMax: f.Rectangle.Max.X,
			YMax: f.Rectangle.Max.Y,
		}

		// Calculate quality score based on face size and position
		confidence := d.calculateConfidence(bbox, img.Bounds())

		// Determine pose type (simplified - go-face doesn't provide this directly)
		poseType := d.estimatePose(f.Descriptor)

		// Crop face from image
		cropped, err := d.cropFace(img, bbox)
		if err != nil {
			// Cropping failed, but continue with empty cropped data
			cropped = []byte{}
		}

		// Convert landmarks
		landmarks := make([]image.Point, len(f.Shapes))
		for i, shape := range f.Shapes {
			landmarks[i] = image.Point{X: shape.X, Y: shape.Y}
		}

		detection := FaceDetection{
			Box: bbox,
			Confidence: ConfidenceScore{
				Score:   confidence,
				Type:    poseType.String(),
				TypeRaw: int(poseType),
			},
			Landmarks: landmarks,
			Cropped:   cropped,
			Size: image.Point{
				X: bbox.Width(),
				Y: bbox.Height(),
			},
		}

		// Filter by minimum confidence
		if confidence >= d.minConfidence {
			detections = append(detections, detection)
		}
	}

	return detections, nil
}

// calculateConfidence estimates face quality based on size and position
// This is a simplified version - the Python service uses dlib's more sophisticated scoring
func (d *Detector) calculateConfidence(bbox BoundingBox, imgBounds image.Rectangle) float64 {
	// Base score on face size relative to image
	faceArea := float64(bbox.Area())
	imgArea := float64(imgBounds.Dx() * imgBounds.Dy())
	sizeRatio := faceArea / imgArea

	// Faces that are 5-20% of image area are considered good
	var sizeScore float64
	if sizeRatio >= 0.05 && sizeRatio <= 0.20 {
		sizeScore = 2.0
	} else if sizeRatio >= 0.02 && sizeRatio < 0.05 {
		sizeScore = 1.5
	} else if sizeRatio > 0.20 && sizeRatio <= 0.40 {
		sizeScore = 1.5
	} else if sizeRatio >= 0.01 {
		sizeScore = 1.0
	} else {
		sizeScore = 0.5
	}

	// Check if face is centered (bonus points)
	center := bbox.Center()
	imgCenter := image.Point{
		X: imgBounds.Dx() / 2,
		Y: imgBounds.Dy() / 2,
	}

	dx := abs(center.X - imgCenter.X)
	dy := abs(center.Y - imgCenter.Y)
	distFromCenter := float64(dx + dy)
	maxDist := float64(imgBounds.Dx() + imgBounds.Dy())
	centerRatio := 1.0 - (distFromCenter / maxDist)

	centerScore := centerRatio * 0.5 // Max 0.5 bonus for centered faces

	return sizeScore + centerScore
}

// estimatePose estimates face pose based on descriptor
// This is a simplified heuristic - go-face doesn't provide direct pose estimation
func (d *Detector) estimatePose(descriptor face.Descriptor) PoseType {
	// Without explicit pose detection, default to front
	// In a full implementation, we could use landmark positions to estimate pose
	return PoseFront
}

// cropFace extracts and encodes the face region from the image
func (d *Detector) cropFace(img image.Image, bbox BoundingBox) ([]byte, error) {
	// Add padding (10%)
	padding := bbox.Width() / 10
	x1 := max(0, bbox.XMin-padding)
	y1 := max(0, bbox.YMin-padding)
	x2 := min(img.Bounds().Dx(), bbox.XMax+padding)
	y2 := min(img.Bounds().Dy(), bbox.YMax+padding)

	// Create sub-image
	rect := image.Rect(x1, y1, x2, y2)
	cropped := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(rect)

	// Encode to JPEG bytes
	tmpFile, err := os.CreateTemp("", "face-crop-*.jpg")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if err := jpeg.Encode(tmpFile, cropped, &jpeg.Options{Quality: 95}); err != nil {
		return nil, err
	}

	return os.ReadFile(tmpFile.Name())
}

// loadImage loads an image file
func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	return img, err
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
