package composer

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderNameCardPNGDimensionsAndAlpha(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "namecard.png")
	err := RenderNameCardPNG(NameCardInput{
		PlayerName:   "测试选手ZywOo",
		TeamKey:      "CT",
		RoundNumber:  12,
		HighlightTag: "四杀 · 颗秒",
	}, outPath)
	if err != nil {
		t.Fatalf("RenderNameCardPNG failed: %v", err)
	}
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open %s: %v", outPath, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", outPath, err)
	}
	b := img.Bounds()
	if b.Dx() != NameCardWidth || b.Dy() != NameCardHeight {
		t.Fatalf("expected %dx%d, got %dx%d", NameCardWidth, NameCardHeight, b.Dx(), b.Dy())
	}
	// Corner pixel (0,0) should be transparent (alpha == 0) outside the rounded card.
	_, _, _, a := img.At(0, 0).RGBA()
	if a != 0 {
		t.Fatalf("expected transparent corner pixel, got alpha=%d", a)
	}
}

func TestFrameMeldAdapterNoOpWhenUnconfigured(t *testing.T) {
	ctx := context.Background()
	got, err := RunFrameMeld(ctx, FrameMeldConfig{Enabled: false, ExePath: ""}, "/tmp/in.mp4", "/tmp/out.mp4")
	if err != nil {
		t.Fatalf("unexpected error when disabled: %v", err)
	}
	if got != "/tmp/in.mp4" {
		t.Fatalf("expected /tmp/in.mp4 passthrough, got %q", got)
	}

	got, err = RunFrameMeld(ctx, FrameMeldConfig{Enabled: true, ExePath: ""}, "/tmp/in.mp4", "/tmp/out.mp4")
	if err != nil || got != "/tmp/in.mp4" {
		t.Fatalf("expected passthrough when ExePath empty, got %q err=%v", got, err)
	}
}
