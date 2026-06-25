package media

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func makeTestPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestGenerateThumbnail(t *testing.T) {
	data := makeTestPNG(800, 600)
	cfg := DefaultThumbnailConfig()
	thumb, err := GenerateThumbnail(data, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(thumb) == 0 {
		t.Error("thumbnail should not be empty")
	}
	if len(thumb) >= len(data) {
		t.Error("thumbnail should be smaller than original")
	}
}

func TestGenerateThumbnail_SmallImage(t *testing.T) {
	data := makeTestPNG(50, 50)
	cfg := DefaultThumbnailConfig()
	thumb, err := GenerateThumbnail(data, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Small image may not be resized
	if len(thumb) == 0 {
		t.Error("thumbnail should not be empty")
	}
}

func TestGenerateThumbnail_EmptyData(t *testing.T) {
	cfg := DefaultThumbnailConfig()
	_, err := GenerateThumbnail([]byte{}, cfg)
	if err == nil {
		t.Error("empty data should fail")
	}
}

func TestGetImageDimensions(t *testing.T) {
	data := makeTestPNG(640, 480)
	w, h, format, err := GetImageDimensions(data)
	if err != nil {
		t.Fatal(err)
	}
	if w != 640 || h != 480 {
		t.Errorf("expected 640x480, got %dx%d", w, h)
	}
	if format != "png" {
		t.Errorf("expected png, got %s", format)
	}
}

func TestDetectMimeType(t *testing.T) {
	tests := []struct{ name, expected string }{
		{"photo.jpg", "image/jpeg"},
		{"photo.png", "image/png"},
		{"video.mp4", "video/mp4"},
		{"file.xyz", "application/octet-stream"},
	}
	for _, tt := range tests {
		got := DetectMimeType(tt.name)
		if got != tt.expected {
			t.Errorf("DetectMimeType(%s) = %s, want %s", tt.name, got, tt.expected)
		}
	}
}

func TestGeneratePlaceholder(t *testing.T) {
	data, err := GeneratePlaceholder(100, 100, color.RGBA{0, 0, 255, 255})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("placeholder should not be empty")
	}
}

func TestValidateImageSize(t *testing.T) {
	err := ValidateImageSize(make([]byte, 1000), 500)
	if err == nil {
		t.Error("should fail for oversized image")
	}
	err = ValidateImageSize(make([]byte, 100), 500)
	if err != nil {
		t.Error("should pass for valid size")
	}
}
