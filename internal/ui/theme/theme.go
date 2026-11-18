// Package theme defines Strikeye's dark esports MyGo Native UI palette,
// font registration, and reusable card/capsule/button/table primitives.
package theme

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	"strikeye/assets/fonts"
	"strikeye/internal/config"
)

var frostedGlassShader = shader.MustCompile(`package main

func ColorEffect(pos vec2, color vec4) vec4 {
	b := backdropSample(pos)
	return mix(b, color, color.a)
}
`)

// Default Pure Black + Emerald Green color tokens.
var (
	BgBase       = ui.Hex("#000000") // Pure black OLED background
	BgSurface    = ui.Hex("#09090B") // Card & panel background
	BgElevated   = ui.Hex("#141418") // Hover / elevated panel
	BgInput      = ui.Hex("#050507") // Input / dark control surface
	BorderSubtle = ui.Hex("#222228") // Subtle card border
	TextPrimary  = ui.Hex("#FAFAFA") // Crisp primary text
	TextMuted    = ui.Hex("#8E8E98") // Secondary label text
	AccentAmber  = ui.Hex("#22C55E") // Primary highlight / action accent (emerald green)
	AccentCyan   = ui.Hex("#38BDF8") // Secondary data / active accent
	StatusGreen  = ui.Hex("#22C55E") // Ready / verified
	StatusYellow = ui.Hex("#EAB308") // Queued / pending
	StatusRed    = ui.Hex("#EF4444") // Failed / error
	StatusGray   = ui.Hex("#64748B") // Unrendered / idle
	TeamCT       = ui.Hex("#38BDF8") // Counter-Terrorist blue
	TeamT        = ui.Hex("#F59E0B") // Terrorist amber

	themeMu        sync.RWMutex
	activeIsDark   = true
	accentHover    = ui.Hex("#4ADE80")
	accentPressed  = ui.Hex("#16A34A")
	accentText     = ui.Hex("#000000")
	overlayColor   = ui.RGBA(0, 0, 0, 0.72)
	bgImagePath    string
	bgBitmap       *ui.Bitmap
	hasGradient    bool
	bgGradient     ui.LinearGradient
	activePreset   = config.ThemePresetPureBlack
	activeCardOpac = 0.90
)

// PresetSpec defines a built-in Strikeye color palette preset.
type PresetSpec struct {
	ID           string
	Label        string
	Dark         bool
	BgBase       string
	BgSurface    string
	BgElevated   string
	BgInput      string
	BorderSubtle string
	TextPrimary  string
	TextMuted    string
	Accent       string
	Gradient     []string
}

// Presets returns all built-in Strikeye theme presets.
func Presets() []PresetSpec {
	return []PresetSpec{
		{
			ID:           config.ThemePresetEsportsDark,
			Label:        "暗夜电竞 (Esports Dark)",
			Dark:         true,
			BgBase:       "#0F141C",
			BgSurface:    "#171E29",
			BgElevated:   "#1F2836",
			BgInput:      "#121822",
			BorderSubtle: "#2A3546",
			TextPrimary:  "#F1F5F9",
			TextMuted:    "#94A3B8",
			Accent:       "#F59E0B",
		},
		{
			ID:           config.ThemePresetPureBlack,
			Label:        "极致全黑 (Pure Black)",
			Dark:         true,
			BgBase:       "#000000",
			BgSurface:    "#09090B",
			BgElevated:   "#141418",
			BgInput:      "#050507",
			BorderSubtle: "#222228",
			TextPrimary:  "#FAFAFA",
			TextMuted:    "#8E8E98",
			Accent:       "#22C55E",
		},
		{
			ID:           config.ThemePresetGradientAurora,
			Label:        "极光渐变 (Aurora Gradient)",
			Dark:         true,
			BgBase:       "#0B1026",
			BgSurface:    "#11182E",
			BgElevated:   "#192342",
			BgInput:      "#0D1326",
			BorderSubtle: "#2C3B66",
			TextPrimary:  "#F1F5F9",
			TextMuted:    "#94A3B8",
			Accent:       "#38BDF8",
			Gradient:     []string{"#0B1026", "#1B1445", "#0F2942"},
		},
		{
			ID:           config.ThemePresetGradientSunset,
			Label:        "暮光渐变 (Dusk Gradient)",
			Dark:         true,
			BgBase:       "#1A0E24",
			BgSurface:    "#1E132B",
			BgElevated:   "#2A1B3D",
			BgInput:      "#150C20",
			BorderSubtle: "#412A5C",
			TextPrimary:  "#F8FAFC",
			TextMuted:    "#A89BB9",
			Accent:       "#EC4899",
			Gradient:     []string{"#1A0E24", "#2D1436", "#1B1E38"},
		},
		{
			ID:           config.ThemePresetMidnightBlue,
			Label:        "深海战术 (Midnight Blue)",
			Dark:         true,
			BgBase:       "#0A101D",
			BgSurface:    "#111B2E",
			BgElevated:   "#1A2740",
			BgInput:      "#0D1526",
			BorderSubtle: "#243656",
			TextPrimary:  "#E2E8F0",
			TextMuted:    "#94A3B8",
			Accent:       "#38BDF8",
		},
		{
			ID:           config.ThemePresetTacticalRed,
			Label:        "猩红指挥 (Tactical Crimson)",
			Dark:         true,
			BgBase:       "#120D10",
			BgSurface:    "#1C1418",
			BgElevated:   "#271C22",
			BgInput:      "#161013",
			BorderSubtle: "#3D2933",
			TextPrimary:  "#F8FAFC",
			TextMuted:    "#A1A1AA",
			Accent:       "#EF4444",
		},
		{
			ID:           config.ThemePresetLightClean,
			Label:        "极简亮白 (Light Clean)",
			Dark:         false,
			BgBase:       "#F1F5F9",
			BgSurface:    "#FFFFFF",
			BgElevated:   "#E2E8F0",
			BgInput:      "#F8FAFC",
			BorderSubtle: "#CBD5E1",
			TextPrimary:  "#0F172A",
			TextMuted:    "#475569",
			Accent:       "#D97706",
		},
	}
}

// AccentSwatches returns curated accent color hex presets for the UI picker.
func AccentSwatches() []struct {
	Label string
	Hex   string
} {
	return []struct {
		Label string
		Hex   string
	}{
		{"琥珀金", "#F59E0B"},
		{"战术青", "#38BDF8"},
		{"烈焰红", "#EF4444"},
		{"翡翠绿", "#22C55E"},
		{"霓虹紫", "#A855F7"},
		{"樱花粉", "#EC4899"},
	}
}

func findPreset(id string) PresetSpec {
	for _, p := range Presets() {
		if p.ID == id {
			return p
		}
	}
	return Presets()[0]
}

func withAlpha(c ui.Color, alpha float64) ui.Color {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	return ui.RGBA(c.R, c.G, c.B, float32(alpha))
}

func adjustLuminance(c ui.Color, factor float64) ui.Color {
	clamp := func(v float64) uint8 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(v)
	}
	return ui.Color{
		R: clamp(float64(c.R) * factor),
		G: clamp(float64(c.G) * factor),
		B: clamp(float64(c.B) * factor),
		A: c.A,
	}
}

// Apply updates Strikeye's active palette tokens and cached background bitmap from cfg.
func Apply(cfg config.ThemeConfig) {
	p := findPreset(cfg.Preset)
	accentHex := strings.TrimSpace(cfg.AccentColor)
	if accentHex == "" {
		accentHex = p.Accent
	}

	themeMu.Lock()
	defer themeMu.Unlock()

	activePreset = p.ID
	activeIsDark = p.Dark

	base := ui.Hex(p.BgBase)
	surface := ui.Hex(p.BgSurface)
	elevated := ui.Hex(p.BgElevated)
	input := ui.Hex(p.BgInput)

	path := strings.TrimSpace(cfg.BackgroundImage)
	if path != bgImagePath {
		bgImagePath = path
		bgBitmap = nil
		if path != "" {
			if f, err := os.Open(path); err == nil {
				if img, _, decErr := image.Decode(f); decErr == nil {
					bgBitmap = ui.NewBitmap(img)
				}
				_ = f.Close()
			}
		}
	}

	cardAlpha := cfg.CardOpacity
	if cardAlpha <= 0 {
		cardAlpha = 0.90
	}
	activeCardOpac = cardAlpha

	hasGradient = len(p.Gradient) >= 2
	if hasGradient {
		stops := make([]ui.GradientStop, 0, len(p.Gradient))
		for i, hex := range p.Gradient {
			pos := float32(i) / float32(len(p.Gradient)-1)
			stops = append(stops, ui.GradientStop{Color: ui.Hex(hex), At: pos})
		}
		bgGradient = ui.LinearGradient{
			Angle: 135,
			Stops: stops,
			Oklab: true,
		}
	} else {
		bgGradient = ui.LinearGradient{}
	}

	if bgBitmap != nil || hasGradient {
		surface = withAlpha(surface, cardAlpha)
		elevated = withAlpha(elevated, cardAlpha)
		input = withAlpha(input, cardAlpha)
	}

	overlayAlpha := cfg.OverlayOpacity
	if overlayAlpha < 0 {
		overlayAlpha = 0
	}
	if overlayAlpha > 0.95 {
		overlayAlpha = 0.95
	}
	overlayColor = withAlpha(base, overlayAlpha)

	BgBase = base
	BgSurface = surface
	BgElevated = elevated
	BgInput = input
	BorderSubtle = ui.Hex(p.BorderSubtle)
	if bgBitmap != nil || hasGradient {
		if p.Dark {
			BorderSubtle = ui.RGBA(255, 255, 255, 0.14)
		} else {
			BorderSubtle = ui.RGBA(15, 23, 42, 0.12)
		}
	}
	TextPrimary = ui.Hex(p.TextPrimary)
	TextMuted = ui.Hex(p.TextMuted)

	AccentAmber = ui.Hex(accentHex)
	accentHover = adjustLuminance(AccentAmber, 1.12)
	accentPressed = adjustLuminance(AccentAmber, 0.85)
	if p.Dark {
		accentText = base
	} else {
		accentText = ui.Hex("#FFFFFF")
	}
}

// BackgroundBitmap returns the cached background image bitmap and overlay color, if configured.
func BackgroundBitmap() (*ui.Bitmap, ui.Color) {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return bgBitmap, overlayColor
}

// BackgroundGradient returns the active preset's linear gradient backdrop, if any.
func BackgroundGradient() (ui.LinearGradient, bool) {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return bgGradient, hasGradient
}

// GlassEffect returns an Apple-style frosted glass backdrop blur effect when a
// background image or translucent card surface is active.
func GlassEffect() ui.Effect {
	return frostedGlassShader.Effect(shader.Options{BackdropBlur: 18})
}

// GlassSurface returns a translucent surface color suitable for Apple-style frosted glass.
func GlassSurface(alpha float64) ui.Color {
	return withAlpha(BgSurface, alpha)
}

// CardOpacity returns the active configured card opacity.
func CardOpacity() float64 {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return activeCardOpac
}

var registerOnce sync.Once

// RegisterFonts registers Strikeye's embedded fonts with MyGo's UI renderer once.
func RegisterFonts() {
	registerOnce.Do(func() {
		entries := []struct {
			id     fonts.FontID
			family string
		}{
			{fonts.NotoSansSCMedium, "Noto Sans SC"},
			{fonts.NotoSansSCBold, "Noto Sans SC"},
			{fonts.RajdhaniSemiBold, "Rajdhani"},
			{fonts.RajdhaniBold, "Rajdhani"},
		}
		for _, e := range entries {
			if data, ok := fonts.RawBytes(e.id); ok {
				_ = ui.RegisterFont(data, e.family)
			}
		}
	})
}

// ActiveTheme returns Strikeye's active ui.Theme for wrapping root views
// via ui.Themed(c, theme.ActiveTheme()).
func ActiveTheme() *ui.Theme {
	themeMu.RLock()
	dark := activeIsDark
	hover := accentHover
	pressed := accentPressed
	aText := accentText
	themeMu.RUnlock()

	var t *ui.Theme
	if dark {
		t = ui.DarkTheme()
	} else {
		t = ui.LightTheme()
	}
	t.Dark = dark
	t.Background = BgBase
	t.Surface = BgInput
	t.SurfaceHover = BgElevated
	t.SurfacePressed = BgSurface
	t.Border = BorderSubtle
	t.Text = TextPrimary
	t.TextMuted = TextMuted
	t.Accent = AccentAmber
	t.AccentHover = hover
	t.AccentPressed = pressed
	t.AccentText = aText
	t.InactiveAccent = AccentAmber
	t.Danger = StatusRed
	t.Warning = StatusYellow
	t.Success = StatusGreen
	t.Focus = AccentCyan
	t.Scrollbar = BorderSubtle
	t.ScrollbarWidth = 6
	t.Radius = 10
	t.Spacing = 4
	t.Font = "Noto Sans SC"
	t.FontSize = 13
	return t
}

// EsportsTheme returns the active Strikeye ui.Theme (kept for compatibility).
func EsportsTheme() *ui.Theme {
	return ActiveTheme()
}

// RowCenter starts a horizontal flex row with vertical center alignment.
func RowCenter(c *ui.Context) ui.Element {
	return ui.Row(c).AlignItems(ui.Center)
}

// StatText renders tabular numeric text using the embedded Rajdhani font.
func StatText(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).Font("Rajdhani").FontFeatures("tnum")
}

// ScoreText renders bold tabular numeric text in Rajdhani for scorelines and KPIs.
func ScoreText(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).Font("Rajdhani").FontFeatures("tnum").Bold()
}

// StatusDot renders a crisp circular vector status indicator.
func StatusDot(c *ui.Context, col ui.Color, size float32) ui.Element {
	return ui.Box(c).Width(size).Height(size).Radius(size / 2).Background(col)
}

// Card renders a frosted-glass / esports surface container with subtle border and padding.
func Card(c *ui.Context, fn func()) ui.Element {
	el := ui.Column(c).
		Background(BgSurface).
		Border(1, BorderSubtle).
		Radius(16).
		Padding(14).
		Gap(10)
	if bmp, _ := BackgroundBitmap(); bmp != nil || hasGradient || c.Vibrancy() {
		el = el.Effect(GlassEffect())
	}
	return el.Children(fn)
}

// SectionTitle renders a bold section heading in primary text color.
func SectionTitle(c *ui.Context, title string) ui.Element {
	return ui.Text(c, title).FontSize(15).Bold().TextColor(TextPrimary)
}

// MutedText renders secondary informational text.
func MutedText(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(12).TextColor(TextMuted)
}

// Capsule renders a compact status badge with a tinted border and label.
func Capsule(c *ui.Context, label string, accent ui.Color) ui.Element {
	return RowCenter(c).
		Shrink(0).
		Background(BgInput).
		Border(1, accent).
		Radius(12).
		PaddingX(10).
		PaddingY(4).
		Gap(6).
		Children(func() {
			StatusDot(c, accent, 6)
			ui.Text(c, label).FontSize(11).Bold().TextColor(accent)
		})
}

func withButtonMotion(el ui.Element, baseBg, hoverBg, pressBg ui.Color, hasBorder bool, baseBorder, hoverBorder ui.Color) ui.Element {
	el = el.Cursor(ui.CursorPointer).
		Transition(ui.ElementTransition{Duration: 130 * time.Millisecond, Ease: ui.EaseOut})
	if el.Pressed() {
		el = el.Background(pressBg).Scale(0.96, 0.96)
		if hasBorder {
			el = el.Border(1, hoverBorder)
		}
		return el
	}
	if el.Hovered() {
		el = el.Background(hoverBg).Scale(1.02, 1.02)
		if hasBorder {
			el = el.Border(1, hoverBorder)
		}
		return el
	}
	el = el.Background(baseBg).Scale(1.0, 1.0)
	if hasBorder {
		el = el.Border(1, baseBorder)
	}
	return el
}

// ActionButton renders a themed secondary action button with hover/press motion.
func ActionButton(c *ui.Context, label string) ui.Element {
	el := ui.Button(c, label).
		Radius(10).
		PaddingX(12).
		PaddingY(6).
		TextColor(TextPrimary)
	return withButtonMotion(el, BgElevated, adjustLuminance(BgElevated, 1.25), BgSurface, true, BorderSubtle, AccentAmber)
}

// AccentButton renders a high-visibility primary action button with hover/press motion.
func AccentButton(c *ui.Context, label string) ui.Element {
	themeMu.RLock()
	fg := accentText
	hv := accentHover
	pr := accentPressed
	themeMu.RUnlock()
	el := ui.PrimaryButton(c, label).
		Radius(10).
		PaddingX(14).
		PaddingY(7).
		TextColor(fg)
	return withButtonMotion(el, AccentAmber, hv, pr, false, ui.Transparent, ui.Transparent)
}

// ProgressBar renders a themed progress indicator with optional label.
// Pass progress in [0.0, 1.0] for determinate progress, or < 0 for an indeterminate loading bar.
func ProgressBar(c *ui.Context, progress float64, label string) ui.Element {
	indeterminate := progress < 0
	if progress > 1 {
		progress = 1
	}
	return ui.Column(c).FillWidth().Gap(4).Children(func() {
		if label != "" || !indeterminate {
			RowCenter(c).FillWidth().Gap(8).Children(func() {
				if label != "" {
					MutedText(c, label)
				}
				ui.Spacer(c)
				if !indeterminate {
					StatText(c, fmt.Sprintf("%.0f%%", progress*100)).FontSize(11).Bold().TextColor(AccentAmber)
				}
			})
		}
		ui.Row(c).FillWidth().Height(9).Background(BgInput).Border(1, BorderSubtle).Radius(5).Clip().Children(func() {
			if indeterminate {
				phase := float32((time.Now().UnixMilli()%1200))/1200.0 * 0.72
				ui.Box(c).Grow(phase)
				ui.Box(c).FillHeight().Grow(0.28).Background(AccentAmber).Radius(4)
				ui.Box(c).Grow(0.72 - phase)
			} else if progress > 0 {
				p := float32(progress)
				ui.Box(c).FillHeight().Grow(p).Background(AccentAmber).Radius(4)
				if p < 1 {
					ui.Box(c).Grow(1 - p)
				}
			}
		})
	})
}

// DangerButton renders a destructive/abort action button with hover/press motion.
func DangerButton(c *ui.Context, label string) ui.Element {
	el := ui.Button(c, label).
		Radius(10).
		PaddingX(12).
		PaddingY(6).
		TextColor(StatusRed)
	return withButtonMotion(el, BgElevated, adjustLuminance(BgElevated, 1.2), BgSurface, true, StatusRed, StatusRed)
}

// TabButton renders a borderless secondary workspace tab with a bottom indicator bar,
// visually distinct from bordered ActionButton controls.
func TabButton(c *ui.Context, label string, active bool) ui.Element {
	fg := TextMuted
	bar := ui.Transparent
	bg := ui.Transparent
	if active {
		fg = TextPrimary
		bar = AccentAmber
		bg = BgSurface
	}
	el := ui.ButtonBase(c).
		Radius(8).
		PaddingX(12).
		PaddingY(6).
		Children(func() {
			ui.Column(c).AlignItems(ui.Center).Gap(4).Children(func() {
				ui.Text(c, label).FontSize(12).Bold().TextColor(fg)
				ui.Box(c).FillWidth().Height(2).Radius(1).Background(bar)
			})
		})
	return withButtonMotion(el, bg, BgElevated, BgSurface, false, ui.Transparent, ui.Transparent)
}

// NavButton renders a two-tier sidebar navigation item with an active accent bar.
func NavButton(c *ui.Context, title, subtitle string, active bool) ui.Element {
	bg := BgSurface
	border := BgSurface
	bar := ui.Transparent
	titleCol := TextMuted
	subCol := StatusGray
	if active {
		bg = BgElevated
		border = BorderSubtle
		bar = AccentAmber
		titleCol = TextPrimary
		subCol = AccentAmber
	}
	el := ui.ButtonBase(c).
		FillWidth().
		Radius(10).
		PaddingX(10).
		PaddingY(9).
		Gap(10).
		AlignItems(ui.Center).
		Children(func() {
			ui.Box(c).Width(3).Height(26).Radius(2).Background(bar)
			ui.Column(c).FillWidth().Gap(2).Children(func() {
				ui.Text(c, title).FontSize(13).Bold().TextColor(titleCol)
				ui.Text(c, subtitle).Font("Rajdhani").FontSize(10).LetterSpacing(0.8).TextColor(subCol)
			})
		})
	return withButtonMotion(el, bg, BgElevated, BgSurface, true, border, BorderSubtle)
}

// ParseStatusColor maps a demo parse_status value to its semantic badge color.
func ParseStatusColor(status string) ui.Color {
	switch status {
	case "ready":
		return StatusGreen
	case "parsing":
		return AccentCyan
	case "failed":
		return StatusRed
	default:
		return StatusYellow
	}
}

// MediaStatusColor maps a spectator/render readiness state to its badge color.
func MediaStatusColor(state string) ui.Color {
	switch state {
	case "ready":
		return StatusGreen
	case "queued", "running":
		return StatusYellow
	case "failed":
		return StatusRed
	default:
		return StatusGray
	}
}
