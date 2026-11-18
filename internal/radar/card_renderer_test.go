package radar

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writeSolidPNG(t *testing.T, path string, w, h int, c color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatalf("encode %s: %v", path, err)
	}
	_ = f.Close()
}

func TestRenderCardPNGDimensionsAndVariants(t *testing.T) {
	tmp := t.TempDir()
	raw := RawValues{
		KPR:       0.92,
		Surviving: 0.46,
		ADR:       94.5,
		KAST:      0.81,
		MultiKill: 7,
		Rating:    1.36,
	}
	baseIn := CardInput{
		PlayerName: "选手测试Player",
		TeamKey:    "CT",
		TeamLabel:  "FURIA",
		Kills:      24,
		Deaths:     15,
		Assists:    6,
		Raw:        raw,
		Normalized: Normalize(raw),
		MatchAvg:   NormalizedValues{KPR: 0.8, Surviving: 0.8, ADR: 0.8, KAST: 0.8, MultiKill: 0.7, Rating: 0.8},
	}

	out1 := filepath.Join(tmp, "card_placeholder.png")
	if err := RenderCardPNG(baseIn, out1); err != nil {
		t.Fatalf("RenderCardPNG placeholder failed: %v", err)
	}
	f1, err := os.Open(out1)
	if err != nil {
		t.Fatalf("open %s: %v", out1, err)
	}
	cfg1, err := png.DecodeConfig(f1)
	_ = f1.Close()
	if err != nil {
		t.Fatalf("decode config %s: %v", out1, err)
	}
	if cfg1.Width != CanvasWidth || cfg1.Height != CanvasHeight {
		t.Fatalf("expected %dx%d, got %dx%d", CanvasWidth, CanvasHeight, cfg1.Width, cfg1.Height)
	}

	// Render with portrait + team logo + T side.
	portraitPath := filepath.Join(tmp, "portrait.png")
	logoPath := filepath.Join(tmp, "logo.png")
	writeSolidPNG(t, portraitPath, 128, 128, color.RGBA{40, 120, 200, 255})
	writeSolidPNG(t, logoPath, 128, 128, color.RGBA{240, 180, 40, 255})

	inWithAssets := baseIn
	inWithAssets.TeamKey = "T"
	inWithAssets.PortraitPath = portraitPath
	inWithAssets.TeamLogoPath = logoPath

	out2 := filepath.Join(tmp, "card_assets.png")
	if err := RenderCardPNG(inWithAssets, out2); err != nil {
		t.Fatalf("RenderCardPNG with assets failed: %v", err)
	}
	f2, err := os.Open(out2)
	if err != nil {
		t.Fatalf("open %s: %v", out2, err)
	}
	cfg2, err := png.DecodeConfig(f2)
	_ = f2.Close()
	if err != nil {
		t.Fatalf("decode config %s: %v", out2, err)
	}
	if cfg2.Width != CanvasWidth || cfg2.Height != CanvasHeight {
		t.Fatalf("expected %dx%d, got %dx%d", CanvasWidth, CanvasHeight, cfg2.Width, cfg2.Height)
	}
}
