package rpc

import (
	"net/url"
	"regexp"
	"strings"

	"bytes"
	"image"
	_ "image/gif" // Register GIF format
	"image/jpeg"
	_ "image/png" // Register PNG format

	_ "golang.org/x/image/bmp"  // Register BMP format
	_ "golang.org/x/image/webp" // Register WEBP format

	"github.com/rwcarlsen/goexif/exif"
	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// NormalizeHost normalizes localhost IP addresses in the given URL to the configured Stash host URL.
func (s *Service) NormalizeHost(urlStr string) string {
	log.Debugf("Normalizing URL host for: %s", urlStr)
	hostName := "0.0.0.0"
	config := s.config
	u, err := url.Parse(urlStr)
	if err != nil {
		log.Warnf("Failed to parse URL %s: %v", urlStr, err)
		return urlStr
	}
	log.Debugf("Parsed URL host: %s", u.Host)
	if strings.HasPrefix(u.Host, hostName) {
		log.Debugf("Detected localhost IP, normalizing to %s", config.StashHostURL)
		re := regexp.MustCompile(`http[s]?://` + regexp.QuoteMeta(hostName) + `(:\d+)?`)
		return re.ReplaceAllString(urlStr, config.StashHostURL)
	}
	return urlStr
}

// ============================================================================
// EXIF Orientation Normalization
// ============================================================================

// NormalizeImageOrientation applies EXIF orientation transformation
// to image pixels and returns correctly-oriented JPEG bytes without EXIF.
//
// CRITICAL: This function prioritizes EXIF orientation tag 274 over any
// conflicting XMP or TIFF orientation metadata to handle stale metadata.
//
// If no EXIF orientation is found or orientation == 1, returns original bytes unchanged.
// If transformation fails, returns original bytes with warning log.
func NormalizeImageOrientation(imageBytes []byte) ([]byte, error) {
	// Parse EXIF from bytes (reads from EXIF IFD only, not XMP/TIFF)
	reader := bytes.NewReader(imageBytes)
	exifData, err := exif.Decode(reader)
	if err != nil {
		// No EXIF data or corrupt EXIF - return original bytes
		log.Debugf("No EXIF data found or failed to decode: %v", err)
		return imageBytes, nil
	}

	// Check orientation tag 274 in EXIF IFD0
	orientationTag, err := exifData.Get(exif.Orientation)
	if err != nil {
		// No orientation tag - return original bytes
		log.Debugf("No EXIF orientation tag found")
		return imageBytes, nil
	}

	orientation, err := orientationTag.Int(0)
	if err != nil {
		log.Warnf("Failed to parse EXIF orientation value: %v", err)
		return imageBytes, nil
	}

	// If orientation == 1 (normal), no transformation needed
	if orientation == 1 {
		log.Debugf("EXIF orientation is 1 (normal), no transformation needed")
		return imageBytes, nil
	}

	log.Infof("Applying EXIF orientation transformation: %d", orientation)

	// Decode image
	img, format, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		log.Warnf("Failed to decode image for EXIF normalization: %v", err)
		return imageBytes, nil
	}

	log.Debugf("Decoded image format for EXIF normalization: %s", format)

	// Apply transformation based on EXIF orientation value
	transformedImg := applyOrientation(img, orientation)

	// Re-encode as JPEG (quality 95) without any EXIF/XMP metadata
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, transformedImg, &jpeg.Options{Quality: 95}); err != nil {
		log.Warnf("Failed to re-encode image after EXIF normalization: %v", err)
		return imageBytes, nil
	}

	log.Infof("Successfully normalized EXIF orientation %d -> 1", orientation)
	return buf.Bytes(), nil
}

// applyOrientation applies EXIF orientation transformation to image
func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 1:
		return img // No transformation
	case 2:
		return flipHorizontal(img)
	case 3:
		return rotate180(img)
	case 4:
		return flipVertical(img)
	case 5:
		return rotate270CW(flipHorizontal(img))
	case 6:
		return rotate90CW(img)
	case 7:
		return rotate90CW(flipHorizontal(img))
	case 8:
		return rotate270CW(img)
	default:
		log.Warnf("Unknown EXIF orientation value: %d, returning original", orientation)
		return img
	}
}

// rotate90CW rotates image 90 degrees clockwise
func rotate90CW(img image.Image) image.Image {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// New image will be rotated: width and height are swapped
	rotated := image.NewRGBA(image.Rect(0, 0, height, width))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// (x, y) -> (height-1-y, x)
			rotated.Set(height-1-y, x, img.At(x+bounds.Min.X, y+bounds.Min.Y))
		}
	}

	return rotated
}

// rotate180 rotates image 180 degrees
func rotate180(img image.Image) image.Image {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	rotated := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// (x, y) -> (width-1-x, height-1-y)
			rotated.Set(width-1-x, height-1-y, img.At(x+bounds.Min.X, y+bounds.Min.Y))
		}
	}

	return rotated
}

// rotate270CW rotates image 270 degrees clockwise (90 degrees counter-clockwise)
func rotate270CW(img image.Image) image.Image {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// New image will be rotated: width and height are swapped
	rotated := image.NewRGBA(image.Rect(0, 0, height, width))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// (x, y) -> (y, width-1-x)
			rotated.Set(y, width-1-x, img.At(x+bounds.Min.X, y+bounds.Min.Y))
		}
	}

	return rotated
}

// flipHorizontal flips image horizontally (mirror)
func flipHorizontal(img image.Image) image.Image {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	flipped := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// (x, y) -> (width-1-x, y)
			flipped.Set(width-1-x, y, img.At(x+bounds.Min.X, y+bounds.Min.Y))
		}
	}

	return flipped
}

// flipVertical flips image vertically
func flipVertical(img image.Image) image.Image {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	flipped := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// (x, y) -> (x, height-1-y)
			flipped.Set(x, height-1-y, img.At(x+bounds.Min.X, y+bounds.Min.Y))
		}
	}

	return flipped
}
