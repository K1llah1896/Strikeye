package components

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestRadarHexPreviewLoadAndReload(t *testing.T) {
	tmp := t.TempDir()
	cardPath := filepath.Join(tmp, "card.png")

	img := image.NewRGBA(image.Rect(0, 0, 16, 9))
	img.Set(0, 0, color.RGBA{R: 245, G: 158, B: 11, A: 255})
	f, err := os.Create(cardPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	prev := NewRadarHexPreview()
	prev.SetCard(cardPath, false)
	if prev.pendingImg == nil || prev.loadErr != "" {
		t.Fatalf("expected staged preview image, got err=%q", prev.loadErr)
	}
	prev.mu.Lock()
	prev.flushPendingLocked()
	bmp := prev.bitmap
	prev.mu.Unlock()
	if bmp == nil {
		t.Fatalf("expected bitmap after main-thread flush")
	}
	prev.SetCard(filepath.Join(tmp, "missing.png"), false)
	if prev.loadErr == "" {
		t.Fatalf("expected load error for missing file")
	}
}
