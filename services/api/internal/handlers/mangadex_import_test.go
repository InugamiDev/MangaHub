package handlers

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestMangaDexNoticeImageDetection(t *testing.T) {
	notice := encodePNG(t, 600, 650)
	if !isMangaDexNoticeImage("uploads.mangadex.org", "image/png", notice) {
		t.Fatal("expected MangaDex 600x650 notice image to be rejected")
	}
	if isMangaDexNoticeImage("cdn.example.com", "image/png", notice) {
		t.Fatal("expected same image dimensions from non-MangaDex host to be allowed")
	}

	page := encodePNG(t, 900, 1400)
	if isMangaDexNoticeImage("uploads.mangadex.org", "image/png", page) {
		t.Fatal("expected normal MangaDex page dimensions to be allowed")
	}
}

func encodePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.White)
	var body bytes.Buffer
	if err := png.Encode(&body, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return body.Bytes()
}
