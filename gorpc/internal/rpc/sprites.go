package rpc

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // Register PNG decoder
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
)

// VTTCue represents a single cue in a WebVTT file
type VTTCue struct {
	StartTime float64
	EndTime   float64
	X         int
	Y         int
	Width     int
	Height    int
}

// ParseVTT parses a WebVTT file and returns sprite cues
func ParseVTT(vttContent string) ([]VTTCue, error) {
	var cues []VTTCue
	scanner := bufio.NewScanner(strings.NewReader(vttContent))

	// Regex to parse timestamp line and xywh coordinates
	// Format: 00:00:05.000 --> 00:00:10.000
	timeRegex := regexp.MustCompile(`(\d+):(\d+):(\d+\.\d+)\s*-->\s*(\d+):(\d+):(\d+\.\d+)`)
	// Format: xywh=160,90,160,90
	xywhRegex := regexp.MustCompile(`xywh=(\d+),(\d+),(\d+),(\d+)`)

	var currentStartTime, currentEndTime float64
	lineHasTime := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and WEBVTT header
		if line == "" || strings.HasPrefix(line, "WEBVTT") {
			continue
		}

		// Check if line contains timestamp
		if timeMatch := timeRegex.FindStringSubmatch(line); timeMatch != nil {
			// Parse start time (HH:MM:SS.mmm)
			startHour, _ := strconv.Atoi(timeMatch[1])
			startMin, _ := strconv.Atoi(timeMatch[2])
			startSec, _ := strconv.ParseFloat(timeMatch[3], 64)
			currentStartTime = float64(startHour*3600+startMin*60) + startSec

			// Parse end time (HH:MM:SS.mmm)
			endHour, _ := strconv.Atoi(timeMatch[4])
			endMin, _ := strconv.Atoi(timeMatch[5])
			endSec, _ := strconv.ParseFloat(timeMatch[6], 64)
			currentEndTime = float64(endHour*3600+endMin*60) + endSec

			lineHasTime = true
		}

		// Check if line contains xywh coordinates
		if lineHasTime {
			if xywhMatch := xywhRegex.FindStringSubmatch(line); xywhMatch != nil {
				x, _ := strconv.Atoi(xywhMatch[1])
				y, _ := strconv.Atoi(xywhMatch[2])
				w, _ := strconv.Atoi(xywhMatch[3])
				h, _ := strconv.Atoi(xywhMatch[4])

				cues = append(cues, VTTCue{
					StartTime: currentStartTime,
					EndTime:   currentEndTime,
					X:         x,
					Y:         y,
					Width:     w,
					Height:    h,
				})

				lineHasTime = false
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning VTT: %w", err)
	}

	return cues, nil
}

// FindCueForTimestamp finds the VTT cue that contains the given timestamp
func FindCueForTimestamp(cues []VTTCue, timestamp float64) (*VTTCue, error) {
	for i := range cues {
		if timestamp >= cues[i].StartTime && timestamp < cues[i].EndTime {
			return &cues[i], nil
		}
	}
	return nil, fmt.Errorf("no cue found for timestamp %.2f", timestamp)
}

// FetchSpriteImage downloads a sprite image from URL
func FetchSpriteImage(url string) (image.Image, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sprite image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch sprite image: status %d", resp.StatusCode)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode sprite image: %w", err)
	}

	return img, nil
}

// FetchVTT downloads a VTT file from URL
func FetchVTT(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch VTT: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch VTT: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read VTT: %w", err)
	}

	return string(body), nil
}

// ExtractThumbnailFromSprite extracts a thumbnail region from a sprite image
func ExtractThumbnailFromSprite(spriteImg image.Image, cue VTTCue) ([]byte, error) {
	// Extract the thumbnail region using imaging library
	thumbnail := imaging.Crop(spriteImg, image.Rect(cue.X, cue.Y, cue.X+cue.Width, cue.Y+cue.Height))

	// Encode as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	return buf.Bytes(), nil
}

// ExtractFromSprite fetches sprite VTT and image, finds the thumbnail for timestamp, and returns it as bytes
func ExtractFromSprite(spriteURL, vttURL string, timestamp float64) ([]byte, error) {
	// Fetch and parse VTT
	vttContent, err := FetchVTT(vttURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VTT: %w", err)
	}

	cues, err := ParseVTT(vttContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse VTT: %w", err)
	}

	// Find cue for timestamp
	cue, err := FindCueForTimestamp(cues, timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to find cue: %w", err)
	}

	// Fetch sprite image
	spriteImg, err := FetchSpriteImage(spriteURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sprite image: %w", err)
	}

	// Extract thumbnail
	thumbnailBytes, err := ExtractThumbnailFromSprite(spriteImg, *cue)
	if err != nil {
		return nil, fmt.Errorf("failed to extract thumbnail: %w", err)
	}

	return thumbnailBytes, nil
}
