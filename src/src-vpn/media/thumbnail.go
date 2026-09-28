package media

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
)

// ThumbnailConfig defines thumbnail generation parameters.
type ThumbnailConfig struct {
	MaxWidth  int
	MaxHeight int
	Quality   int // JPEG quality 1-100
}

// DefaultThumbnailConfig returns default thumbnail settings.
func DefaultThumbnailConfig() ThumbnailConfig {
	return ThumbnailConfig{MaxWidth: 200, MaxHeight: 200, Quality: 75}
}

// GenerateThumbnail creates a resized thumbnail from image bytes.
// Supports JPEG and PNG. Returns JPEG thumbnail bytes.
func GenerateThumbnail(data []byte, cfg ThumbnailConfig) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty image data")
	}

	img, format, err := decodeImage(data)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	thumb := resizeImage(img, cfg.MaxWidth, cfg.MaxHeight)

	var buf bytes.Buffer
	switch format {
	case "png":
		err = png.Encode(&buf, thumb)
	default:
		err = jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: cfg.Quality})
	}
	if err != nil {
		return nil, fmt.Errorf("encode thumbnail: %w", err)
	}
	return buf.Bytes(), nil
}

// GetImageDimensions returns width and height of an image.
func GetImageDimensions(data []byte) (width, height int, format string, err error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, "", err
	}
	return cfg.Width, cfg.Height, format, nil
}

// GeneratePlaceholder creates a solid-color placeholder image.
func GeneratePlaceholder(width, height int, c color.Color) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Base64EncodeImage encodes image bytes to data URI.
func Base64EncodeImage(data []byte, mimeType string) string {
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
}

// DetectMimeType returns MIME type from file extension.
func DetectMimeType(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(lower, ".webm"):
		return "video/webm"
	case strings.HasSuffix(lower, ".ogg"):
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}

func decodeImage(data []byte) (image.Image, string, error) {
	return image.Decode(bytes.NewReader(data))
}

// resizeImage resizes an image to fit within maxW x maxH preserving aspect ratio.
func resizeImage(img image.Image, maxW, maxH int) image.Image {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if w <= maxW && h <= maxH {
		return img
	}

	scale := 1.0
	if float64(w)/float64(maxW) > float64(h)/float64(maxH) {
		scale = float64(maxW) / float64(w)
	} else {
		scale = float64(maxH) / float64(h)
	}

	newW := int(float64(w) * scale)
	newH := int(float64(h) * scale)

	// Simple nearest-neighbor resize
	thumb := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcX := int(float64(x) / scale)
			srcY := int(float64(y) / scale)
			if srcX >= w {
				srcX = w - 1
			}
			if srcY >= h {
				srcY = h - 1
			}
			thumb.Set(x, y, img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}
	return thumb
}

// ValidateImageSize checks that image data is within size limits.
func ValidateImageSize(data []byte, maxBytes int64) error {
	if int64(len(data)) > maxBytes {
		return fmt.Errorf("image too large: %d bytes (max %d)", len(data), maxBytes)
	}
	return nil
}

// ReadImageLimit reads image data with a size limit.
func ReadImageLimit(r io.Reader, maxBytes int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxBytes))
}
