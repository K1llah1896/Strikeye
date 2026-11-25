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

// Default Spatial Obsidian color tokens.
var (
	BgBase       = ui.Hex("#05070B") // Deep obsidian spatial canvas
	BgSurface    = ui.RGBA(15, 20, 30, 0.78) // Translucent glass card surface
	BgElevated   = ui.RGBA(24, 32, 48, 0.86) // Hover / elevated glass panel
	BgInput      = ui.RGBA(9, 13, 21, 0.85)  // Input / recessed glass well
	BorderSubtle = ui.RGBA(255, 255, 255, 0.09) // Luminous hairline glass rim
	RimBorder    = ui.RGBA(255, 255, 255, 0.09) // Specular glass edge highlight
	TextPrimary  = ui.Hex("#F8FAFC") // Crisp primary text
	TextMuted    = ui.Hex("#8B98AD") // Secondary label text
	AccentAmber  = ui.Hex("#38BDF8") // Primary highlight / action accent (ice cyan)
	AccentCyan   = ui.Hex("#22D3EE") // Secondary data / active accent
	StatusGreen  = ui.Hex("#22C55E") // Ready / verified
	StatusYellow = ui.Hex("#EAB308") // Queued / pending
	StatusRed    = ui.Hex("#EF4444") // Failed / error
	StatusGray   = ui.Hex("#64748B") // Unrendered / idle
	TeamCT       = ui.Hex("#38BDF8") // Counter-Terrorist blue
	TeamT        = ui.Hex("#F59E0B") // Terrorist amber

	themeMu        sync.RWMutex
	activeIsDark   = true
	accentHover    = ui.Hex("#7DD3FC")
	accentPressed  = ui.Hex("#0284C7")
	accentText     = ui.Hex("#05070B")
	overlayColor   = ui.RGBA(5, 7, 11, 0.72)
	bgImagePath    string
	bgBitmap       *ui.Bitmap
	hasGradient    = true
	bgGradient     = ui.LinearGradient{
		Angle: 135,
		Stops: []ui.GradientStop{
			{Color: ui.Hex("#05070B"), At: 0},
			{Color: ui.Hex("#0A1222"), At: 0.52},
			{Color: ui.Hex("#060911"), At: 1},
		},
		Oklab: true,
	}
	activePreset   = config.ThemePresetPureBlack
	activeCardOpac = 0.78
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
			Label:        "空间黑曜石 (Spatial Obsidian)",
			Dark:         true,
			BgBase:       "#05070B",
			BgSurface:    "#0F141E",
			BgElevated:   "#182030",
			BgInput:      "#090D15",
			BorderSubtle: "#1F2937",
			TextPrimary:  "#F8FAFC",
			TextMuted:    "#8B98AD",
			Accent:       "#38BDF8",
			Gradient:     []string{"#05070B", "#0A1222", "#060911"},
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
	if p.Dark {
		RimBorder = ui.RGBA(255, 255, 255, 0.09)
	} else {
		RimBorder = ui.RGBA(15, 23, 42, 0.10)
	}
	if bgBitmap != nil || hasGradient {
		BorderSubtle = RimBorder
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

// AmbientBackdrop renders the Spatial Glass background layer: either the user's
// custom background image with readability overlay, or the procedural obsidian
// gradient with soft blurred radial ambient glow orbs.
func AmbientBackdrop(c *ui.Context) {
	bmp, overlayCol := BackgroundBitmap()
	grad, hasGrad := BackgroundGradient()
	themeMu.RLock()
	dark := activeIsDark
	accent := AccentAmber
	cyan := AccentCyan
	base := BgBase
	themeMu.RUnlock()

	if bmp != nil {
		ui.Box(c).Fill().IgnoreInput(true).Children(func() {
			ui.Image(c, bmp).Fill().Fit(ui.Cover)
			ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Background(overlayCol)
		})
		return
	}

	ui.Box(c).Fill().IgnoreInput(true).Children(func() {
		if hasGrad {
			ui.Box(c).Fill().LinearGradient(grad)
		} else {
			ui.Box(c).Fill().Background(base)
		}
		if dark {
			// Top-left accent ambient glow orb
			ui.Box(c).Absolute().Left(-140).Top(-160).Width(620).Height(620).Radius(310).
				RadialGradient(ui.RadialGradient{
					Stops: []ui.GradientStop{
						{Color: withAlpha(accent, 0.16), At: 0},
						{Color: withAlpha(accent, 0.05), At: 0.55},
						{Color: ui.Transparent, At: 1},
					},
				}).Blur(60)
			// Bottom-right secondary cyan/indigo ambient glow orb
			ui.Box(c).Absolute().Right(-140).Bottom(-180).Width(680).Height(680).Radius(340).
				RadialGradient(ui.RadialGradient{
					Stops: []ui.GradientStop{
						{Color: withAlpha(cyan, 0.12), At: 0},
						{Color: withAlpha(ui.Hex("#6366F1"), 0.06), At: 0.52},
						{Color: ui.Transparent, At: 1},
					},
				}).Blur(70)
		}
	})
}

// GlassCard renders a Spatial Glass container with luminous hairline rim,
// rounded corners, soft depth shadow, and backdrop blur when active.
func GlassCard(c *ui.Context, fn func()) ui.Element {
	themeMu.RLock()
	dark := activeIsDark
	rim := RimBorder
	themeMu.RUnlock()

	el := ui.Column(c).
		Background(BgSurface).
		Border(1, rim).
		Radius(20).
		Padding(16).
		Gap(12)
	if dark {
		el = el.Shadow(0, 12, 32, -6, ui.RGBA(0, 0, 0, 0.48))
	} else {
		el = el.Shadow(0, 8, 24, -4, ui.RGBA(15, 23, 42, 0.08))
	}
	return el.Children(fn)
}

// BentoCard renders a Bento 2.0 modular glass tile with configurable padding and optional accent glow rim.
func BentoCard(c *ui.Context, accent ui.Color, padding float32, fn func()) ui.Element {
	themeMu.RLock()
	dark := activeIsDark
	rim := RimBorder
	themeMu.RUnlock()

	if accent.A > 0 {
		rim = withAlpha(accent, 0.30)
	}
	el := ui.Column(c).
		Background(BgSurface).
		Border(1, rim).
		Radius(20).
		Padding(padding).
		Gap(10)
	if dark {
		el = el.Shadow(0, 10, 26, -4, ui.RGBA(0, 0, 0, 0.42))
	}
	return el.Children(fn)
}

// Card renders a Spatial Glass surface container (alias of GlassCard for compatibility).
func Card(c *ui.Context, fn func()) ui.Element {
	return GlassCard(c, fn)
}

// StatTile renders a Bento KPI metric tile with uppercase label, large Rajdhani number, and subtle accent bar.
func StatTile(c *ui.Context, label, value, sub string, accent ui.Color) ui.Element {
	return BentoCard(c, accent, 14, func() {
		RowCenter(c).FillWidth().Gap(6).Children(func() {
			ui.Box(c).Width(6).Height(6).Radius(3).Background(accent)
			ui.Text(c, strings.ToUpper(label)).Font("Rajdhani").FontSize(11).Bold().LetterSpacing(1.1).TextColor(TextMuted)
			ui.Spacer(c)
		})
		ScoreText(c, value).FontSize(28).TextColor(TextPrimary)
		if sub != "" {
			ui.Text(c, sub).FontSize(11).TextColor(TextMuted)
		}
	})
}

// SegmentItem defines one option inside a SegmentedControl pill switcher.
type SegmentItem struct {
	Key   string
	Label string
	Icon  *ui.SVG
}

// SegmentedControl renders a modern floating pill segmented tab switcher.
func SegmentedControl(c *ui.Context, items []SegmentItem, activeKey string, onSelect func(string)) ui.Element {
	themeMu.RLock()
	rim := RimBorder
	themeMu.RUnlock()

	return RowCenter(c).
		Background(BgInput).
		Border(1, rim).
		Radius(14).
		Padding(4).
		Gap(4).
		Children(func() {
			for _, item := range items {
				active := item.Key == activeKey
				bg := ui.Transparent
				fg := TextMuted
				borderCol := ui.Transparent
				if active {
					bg = withAlpha(AccentAmber, 0.18)
					fg = TextPrimary
					borderCol = withAlpha(AccentAmber, 0.36)
				}
				btn := ui.ButtonBase(c).
					Radius(10).
					PaddingX(12).
					PaddingY(6).
					Gap(6).
					AlignItems(ui.Center).
					Children(func() {
						if item.Icon != nil {
							iconCol := TextMuted
							if active {
								iconCol = AccentAmber
							}
							RenderIcon(c, item.Icon, 14, iconCol)
						}
						ui.Text(c, item.Label).FontSize(12).Bold().TextColor(fg)
					}).
					Cursor(ui.CursorPointer).
					Transition(ui.ElementTransition{Colors: true, Animation: &ui.Snappy})
				pose := ui.Pose{}
				if btn.Pressed() {
					pose = ui.Pose{Scale: 0.97}
					btn = btn.Background(BgSurface).Border(1, withAlpha(AccentAmber, 0.45))
				} else if btn.Hovered() {
					pose = ui.Pose{Y: -1, Scale: 1.02}
					btn = btn.Background(BgElevated).Border(1, withAlpha(AccentAmber, 0.30))
				} else {
					btn = btn.Background(bg).Border(1, borderCol)
				}
				btn = btn.AnimatePose(pose, ui.Snappy)
				if btn.Clicked() && onSelect != nil {
					onSelect(item.Key)
				}
			}
		})
}

// SectionTitle renders a bold section heading in primary text color.
func SectionTitle(c *ui.Context, title string) ui.Element {
	return ui.Text(c, title).FontSize(15).Bold().TextColor(TextPrimary)
}

// MutedText renders secondary informational text.
func MutedText(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(12).TextColor(TextMuted)
}

// PillBadge renders a translucent glowing status pill without heavy solid borders.
func PillBadge(c *ui.Context, label string, accent ui.Color) ui.Element {
	return RowCenter(c).
		Shrink(0).
		Background(withAlpha(accent, 0.14)).
		Border(1, withAlpha(accent, 0.30)).
		Radius(999).
		PaddingX(10).
		PaddingY(4).
		Gap(6).
		Children(func() {
			StatusDot(c, accent, 6)
			ui.Text(c, label).FontSize(11).Bold().TextColor(accent)
		})
}

// Capsule renders a compact status pill badge (delegates to PillBadge).
func Capsule(c *ui.Context, label string, accent ui.Color) ui.Element {
	return PillBadge(c, label, accent)
}

// EmptyStateCard renders a structured Spatial Glass empty-state guide with a centered vector icon.
func EmptyStateCard(c *ui.Context, icon *ui.SVG, title, subtitle string) ui.Element {
	return GlassCard(c, func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).PaddingY(24).Gap(10).Children(func() {
			if icon != nil {
				ui.Box(c).Width(44).Height(44).Radius(22).
					Background(withAlpha(AccentAmber, 0.12)).
					Border(1, withAlpha(AccentAmber, 0.25)).
					Center().
					Children(func() {
						RenderIcon(c, icon, 22, AccentAmber)
					})
			}
			SectionTitle(c, title)
			if subtitle != "" {
				ui.Text(c, subtitle).FontSize(12).TextColor(TextMuted)
			}
		})
	})
}

func withButtonMotion(el ui.Element, baseBg, hoverBg, pressBg ui.Color, hasBorder bool, baseBorder, hoverBorder ui.Color) ui.Element {
	// Colors cross-fade in paint; the press/hover scale runs as a pose, which
	// also animates without rebuilding the view, so busy screens stay at 60fps.
	el = el.Cursor(ui.CursorPointer).
		Transition(ui.ElementTransition{Colors: true, Animation: &ui.Snappy})
	pose := ui.Pose{}
	switch {
	case el.Pressed():
		pose = ui.Pose{Scale: 0.96}
		el = el.Background(pressBg)
		if hasBorder {
			el = el.Border(1, hoverBorder)
		}
	case el.Hovered():
		pose = ui.Pose{Y: -1, Scale: 1.02}
		el = el.Background(hoverBg)
		if hasBorder {
			el = el.Border(1, hoverBorder)
		}
	default:
		el = el.Background(baseBg)
		if hasBorder {
			el = el.Border(1, baseBorder)
		}
	}
	return el.AnimatePose(pose, ui.Snappy)
}

// HoverLift raises an interactive surface a few DIPs while the pointer is over
// it. The lift is a pose, so it is painted without rebuilding the view.
func HoverLift(el ui.Element, lift float32) ui.Element {
	if el.Hovered() {
		return el.AnimatePose(ui.Pose{Y: -lift, Scale: 1.004}, ui.Snappy)
	}
	return el.AnimatePose(ui.Pose{}, ui.Snappy)
}

// ActionButton renders a themed secondary glass action button with hover/press motion.
func ActionButton(c *ui.Context, label string) ui.Element {
	themeMu.RLock()
	rim := RimBorder
	themeMu.RUnlock()
	el := ui.Button(c, label).
		Radius(12).
		PaddingX(13).
		PaddingY(7).
		TextColor(TextPrimary)
	return withButtonMotion(el, BgElevated, adjustLuminance(BgElevated, 1.25), BgSurface, true, rim, withAlpha(AccentAmber, 0.55))
}

// IconActionButton renders a secondary glass action button with a leading vector SVG icon.
func IconActionButton(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	themeMu.RLock()
	rim := RimBorder
	themeMu.RUnlock()
	el := ui.ButtonBase(c).
		Radius(12).
		PaddingX(12).
		PaddingY(7).
		Gap(7).
		AlignItems(ui.Center).
		Children(func() {
			if icon != nil {
				RenderIcon(c, icon, 15, TextMuted)
			}
			ui.Text(c, label).FontSize(12).Bold().TextColor(TextPrimary)
		})
	return withButtonMotion(el, BgElevated, adjustLuminance(BgElevated, 1.25), BgSurface, true, rim, withAlpha(AccentAmber, 0.55))
}

// IconButton renders a compact square glass icon button with an optional unread indicator dot.
func IconButton(c *ui.Context, icon *ui.SVG, badgeDot bool) ui.Element {
	themeMu.RLock()
	rim := RimBorder
	themeMu.RUnlock()
	el := ui.ButtonBase(c).
		Width(34).
		Height(34).
		Radius(11).
		Center().
		Children(func() {
			ui.Box(c).Width(20).Height(20).Center().Children(func() {
				if icon != nil {
					RenderIcon(c, icon, 16, TextPrimary)
				}
				if badgeDot {
					ui.Box(c).Absolute().Right(0).Top(0).Width(7).Height(7).Radius(3.5).Background(AccentAmber)
				}
			})
		})
	return withButtonMotion(el, BgElevated, adjustLuminance(BgElevated, 1.25), BgSurface, true, rim, withAlpha(AccentAmber, 0.55))
}

// AccentButton renders a high-visibility primary action button with hover/press motion.
func AccentButton(c *ui.Context, label string) ui.Element {
	themeMu.RLock()
	fg := accentText
	hv := accentHover
	pr := accentPressed
	themeMu.RUnlock()
	el := ui.PrimaryButton(c, label).
		Radius(12).
		PaddingX(15).
		PaddingY(7).
		TextColor(fg)
	return withButtonMotion(el, AccentAmber, hv, pr, false, ui.Transparent, ui.Transparent)
}

// IconAccentButton renders a primary action button with a leading vector SVG icon.
func IconAccentButton(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	themeMu.RLock()
	fg := accentText
	hv := accentHover
	pr := accentPressed
	themeMu.RUnlock()
	el := ui.ButtonBase(c).
		Radius(12).
		PaddingX(14).
		PaddingY(7).
		Gap(7).
		AlignItems(ui.Center).
		Children(func() {
			if icon != nil {
				RenderIcon(c, icon, 15, fg)
			}
			ui.Text(c, label).FontSize(12).Bold().TextColor(fg)
		})
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
		track := ui.Row(c).FillWidth().Height(9).Background(BgInput).Border(1, BorderSubtle).Radius(5).Clip()
		if indeterminate {
			// Painted per frame without rebuilding the view: the sweep keeps
			// running at 60fps even while the rest of the app is busy.
			track.Draw(func(p *ui.Painter, r ui.Rect) {
				p.AnimationFrameRate(60)
				const cycle = 1200 * time.Millisecond
				t := float64(p.Now().UnixMilli()%cycle.Milliseconds()) / float64(cycle.Milliseconds())
				eased := t * t * (3 - 2*t) // smoothstep
				w := (r.W - 2) * 0.30
				x := r.X + 1 + (r.W-2-w)*float32(eased)
				p.Fill(ui.Rect{X: x, Y: r.Y + 1, W: w, H: r.H - 2}, AccentAmber, 4)
			})
		} else if progress > 0 {
			track.Children(func() {
				ui.Box(c).FillHeight().
					WidthPercent(float32(progress) * 100).
					Background(AccentAmber).
					Radius(4).
					Transition(ui.ElementTransition{Width: true, Duration: 320 * time.Millisecond, Ease: ui.EaseOut})
			})
		}
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
				ui.Text(c, label).FontSize(12).Bold().TextColor(fg).
					Transition(ui.ElementTransition{Colors: true, Duration: 180 * time.Millisecond, Ease: ui.EaseOut})
				ui.Box(c).FillWidth().Height(2).Radius(1).Background(bar).
					Transition(ui.ElementTransition{Colors: true, Animation: &ui.Snappy})
			})
		})
	return withButtonMotion(el, bg, BgElevated, BgSurface, false, ui.Transparent, ui.Transparent)
}

// NavButton renders a two-tier sidebar navigation item (kept for compatibility).
func NavButton(c *ui.Context, title, subtitle string, active bool) ui.Element {
	return NavButtonWithIcon(c, nil, title, subtitle, active)
}

// NavButtonWithIcon renders a Spatial Glass pill navigation item with an inline SVG icon and active glow.
func NavButtonWithIcon(c *ui.Context, icon *ui.SVG, title, subtitle string, active bool) ui.Element {
	bg := ui.Transparent
	border := ui.Transparent
	titleCol := TextMuted
	subCol := StatusGray
	iconCol := TextMuted
	iconBg := withAlpha(TextMuted, 0.08)
	if active {
		bg = withAlpha(AccentAmber, 0.14)
		border = withAlpha(AccentAmber, 0.34)
		titleCol = TextPrimary
		subCol = AccentAmber
		iconCol = AccentAmber
		iconBg = withAlpha(AccentAmber, 0.20)
	}
	el := ui.ButtonBase(c).
		FillWidth().
		Radius(14).
		PaddingX(10).
		PaddingY(9).
		Gap(10).
		AlignItems(ui.Center).
		Children(func() {
			if icon != nil {
				ui.Box(c).Width(32).Height(32).Radius(10).Background(iconBg).Center().
					Transition(ui.ElementTransition{Colors: true, Duration: 180 * time.Millisecond, Ease: ui.EaseOut}).
					Children(func() {
						RenderIcon(c, icon, 16, iconCol)
					})
			} else {
				bar := ui.Transparent
				if active {
					bar = AccentAmber
				}
				ui.Box(c).Width(3).Height(26).Radius(2).Background(bar).
					Transition(ui.ElementTransition{Colors: true, Duration: 180 * time.Millisecond, Ease: ui.EaseOut})
			}
			ui.Column(c).FillWidth().Gap(1).Children(func() {
				ui.Text(c, title).FontSize(13).Bold().TextColor(titleCol).
					Transition(ui.ElementTransition{Colors: true, Duration: 180 * time.Millisecond, Ease: ui.EaseOut})
				ui.Text(c, subtitle).Font("Rajdhani").FontSize(10).LetterSpacing(0.9).TextColor(subCol).
					Transition(ui.ElementTransition{Colors: true, Duration: 180 * time.Millisecond, Ease: ui.EaseOut})
			})
		})
	return withButtonMotion(el, bg, BgElevated, BgSurface, true, border, withAlpha(AccentAmber, 0.30))
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
