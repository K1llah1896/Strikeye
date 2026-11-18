package theme

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"strikeye/internal/config"
)

func TestThemeColorsAndStatusMapping(t *testing.T) {
	if BgBase == BgSurface {
		t.Fatalf("expected distinct base and surface colors")
	}
	if ParseStatusColor("ready") != StatusGreen {
		t.Fatalf("expected ready -> StatusGreen")
	}
	if ParseStatusColor("failed") != StatusRed {
		t.Fatalf("expected failed -> StatusRed")
	}
	if MediaStatusColor("queued") != StatusYellow {
		t.Fatalf("expected queued -> StatusYellow")
	}
	if MediaStatusColor("unrendered") != StatusGray {
		t.Fatalf("expected unrendered -> StatusGray")
	}
	et := EsportsTheme()
	if !et.Dark || et.Background != BgBase || et.Accent != AccentAmber || et.Font != "Noto Sans SC" {
		t.Fatalf("unexpected EsportsTheme values: %+v", et)
	}
}

func TestDisplayFormatters(t *testing.T) {
	if ParseStatusLabel("ready") != "已解析" || ParseStatusLabel("parsing") != "解析中" {
		t.Fatalf("unexpected ParseStatusLabel results")
	}
	if BuyTypeLabel("full_buy") != "长枪局" || BuyTypeLabel("half_buy") != "半起局" {
		t.Fatalf("unexpected BuyTypeLabel results")
	}
	if WinReasonLabel("elimination") != "全歼" || WinReasonLabel("bomb_exploded") != "雷包引爆" {
		t.Fatalf("unexpected WinReasonLabel results")
	}
	if ClipTypeLabel("multi_kill") != "多杀" || ClipTypeLabel("clutch") != "残局" {
		t.Fatalf("unexpected ClipTypeLabel results")
	}
	tags := ParseTags(`["4K","Headshot"]`)
	if len(tags) != 2 || tags[0] != "4K" || tags[1] != "Headshot" {
		t.Fatalf("unexpected ParseTags JSON result: %#v", tags)
	}
	if len(ParseTags("[]")) != 0 {
		t.Fatalf("expected empty tags for []")
	}
	plain := ParseTags("3K, Clutch")
	if len(plain) != 2 || plain[0] != "3K" || plain[1] != "Clutch" {
		t.Fatalf("unexpected ParseTags plain fallback: %#v", plain)
	}
}

func TestDynamicThemeApplyAndBackgroundImage(t *testing.T) {
	defer Apply(config.DefaultConfig(t.TempDir()).Theme)

	tmpDir := t.TempDir()
	bgPath := filepath.Join(tmpDir, "bg.png")
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	img.Set(0, 0, color.RGBA{R: 120, G: 80, B: 200, A: 255})
	f, err := os.Create(bgPath)
	if err != nil {
		t.Fatalf("create bg png: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode bg png: %v", err)
	}
	_ = f.Close()

	Apply(config.ThemeConfig{
		Preset:          config.ThemePresetLightClean,
		AccentColor:     "#38BDF8",
		BackgroundImage: bgPath,
		OverlayOpacity:  0.65,
		CardOpacity:     0.85,
	})

	th := ActiveTheme()
	if th.Dark {
		t.Fatalf("expected light_clean theme to have Dark=false")
	}
	if th.Accent != ui.Hex("#38BDF8") {
		t.Fatalf("expected custom accent #38BDF8, got %+v", th.Accent)
	}
	bmp, ov := BackgroundBitmap()
	if bmp == nil {
		t.Fatalf("expected decoded background bitmap")
	}
	if ov.A == 0 {
		t.Fatalf("expected non-zero overlay alpha")
	}

	// Missing image path should fall back cleanly without panicking.
	Apply(config.ThemeConfig{
		Preset:          config.ThemePresetMidnightBlue,
		AccentColor:     "#EF4444",
		BackgroundImage: filepath.Join(tmpDir, "missing.png"),
		OverlayOpacity:  0.5,
		CardOpacity:     0.9,
	})
	if bmp2, _ := BackgroundBitmap(); bmp2 != nil {
		t.Fatalf("expected nil bitmap on missing background image")
	}
}

func TestProgressBarRendersDeterminateAndIndeterminate(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(10).Gap(8).Children(func() {
			ProgressBar(c, 0.45, "正在解析 Demo")
			ProgressBar(c, -1, "加载比赛分析...")
		})
	}, 400, 200)
	tt.Frame()
}

func TestPureBlackAndGradientPresets(t *testing.T) {
	defer Apply(config.DefaultConfig(t.TempDir()).Theme)

	Apply(config.ThemeConfig{
		Preset:      config.ThemePresetPureBlack,
		AccentColor: "#F59E0B",
	})
	if BgBase != ui.Hex("#000000") {
		t.Fatalf("expected pure black #000000 base, got %+v", BgBase)
	}
	if _, hasGrad := BackgroundGradient(); hasGrad {
		t.Fatalf("expected no gradient for pure_black")
	}

	Apply(config.ThemeConfig{
		Preset:      config.ThemePresetGradientAurora,
		AccentColor: "#38BDF8",
		CardOpacity: 0.80,
	})
	grad, hasGrad := BackgroundGradient()
	if !hasGrad || len(grad.Stops) != 3 {
		t.Fatalf("expected 3-stop gradient for gradient_aurora, got has=%v stops=%d", hasGrad, len(grad.Stops))
	}
}
