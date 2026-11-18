package radar

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"

	"strikeye/assets/fonts"
)

const (
	CanvasWidth    = 2560
	CanvasHeight   = 1440
	RadarCenterX   = 640.0
	RadarCenterY   = 730.0
	RadarMaxR      = 400.0
	PortraitCX     = 1890.0
	PortraitCY     = 600.0
	PortraitSize   = 640.0
	TeamLogoFactor = 1.8
)

// CardInput supplies all player, match-average, and visual asset inputs for a radar card.
type CardInput struct {
	PlayerName   string
	TeamKey      string // "CT" or "T" (or "team_a"/"team_b")
	TeamLabel    string
	Kills        int
	Deaths       int
	Assists      int
	Raw          RawValues
	Normalized   NormalizedValues
	MatchAvg     NormalizedValues
	PortraitPath string
	TeamLogoPath string
}

// RenderParams controls animation-time visual modulation for a single frame.
// For static cards, Scale=1.0, MatchAvgAlpha=1.0, PulsePhase=0.
type RenderParams struct {
	Scale         float64
	MatchAvgAlpha float64
	PulsePhase    float64
	FlashAlpha    float64
}

// StaticParams returns the RenderParams used for static 2560x1440 PNG cards.
func StaticParams() RenderParams {
	return RenderParams{
		Scale:         1.0,
		MatchAvgAlpha: 1.0,
		PulsePhase:    0,
		FlashAlpha:    0,
	}
}

type cardAssets struct {
	portrait *image.RGBA
	teamLogo *image.RGBA
}

type cardFonts struct {
	watermark font.Face
	title     font.Face
	sub       font.Face
	dimLabel  font.Face
	dimVal    font.Face
	dimFull   font.Face
	initial   font.Face
}

func loadCardFonts() (*cardFonts, error) {
	wm, err := fonts.NewFace(fonts.RajdhaniBold, 260)
	if err != nil {
		return nil, err
	}
	title, err := fonts.NewFace(fonts.NotoSansSCBold, 68)
	if err != nil {
		return nil, err
	}
	sub, err := fonts.NewFace(fonts.RajdhaniSemiBold, 42)
	if err != nil {
		return nil, err
	}
	dimLabel, err := fonts.NewFace(fonts.RajdhaniBold, 34)
	if err != nil {
		return nil, err
	}
	dimVal, err := fonts.NewFace(fonts.RajdhaniBold, 42)
	if err != nil {
		return nil, err
	}
	dimFull, err := fonts.NewFace(fonts.NotoSansSCMedium, 24)
	if err != nil {
		return nil, err
	}
	initial, err := fonts.NewFace(fonts.NotoSansSCBold, 220)
	if err != nil {
		return nil, err
	}
	return &cardFonts{
		watermark: wm,
		title:     title,
		sub:       sub,
		dimLabel:  dimLabel,
		dimVal:    dimVal,
		dimFull:   dimFull,
		initial:   initial,
	}, nil
}

func (f *cardFonts) Close() {
	for _, face := range []font.Face{f.watermark, f.title, f.sub, f.dimLabel, f.dimVal, f.dimFull, f.initial} {
		if face != nil {
			_ = face.Close()
		}
	}
}

func loadCardAssets(in CardInput) cardAssets {
	var out cardAssets
	if in.PortraitPath != "" {
		if img, err := loadImageFile(in.PortraitPath); err == nil {
			out.portrait = circularCropRGBA(img, int(PortraitSize))
		}
	}
	if in.TeamLogoPath != "" {
		if img, err := loadImageFile(in.TeamLogoPath); err == nil {
			logoSize := int(math.Round(PortraitSize * TeamLogoFactor))
			out.teamLogo = circularFeatheredRGBA(img, logoSize, 0.40)
		}
	}
	return out
}

// RenderCardPNG renders a 2560x1440 static radar card PNG to outputPath.
func RenderCardPNG(in CardInput, outputPath string) error {
	rgba, err := RenderCardRGBA(in)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create radar card dir: %w", err)
	}
	tmp := outputPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create radar card file: %w", err)
	}
	if err := png.Encode(f, rgba); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("encode radar card png: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, outputPath)
}

// RenderCardRGBA renders a 2560x1440 static radar card into an *image.RGBA.
func RenderCardRGBA(in CardInput) (*image.RGBA, error) {
	cf, err := loadCardFonts()
	if err != nil {
		return nil, err
	}
	defer cf.Close()
	assets := loadCardAssets(in)
	return renderCardFrame(in, StaticParams(), assets, cf), nil
}

func renderCardFrame(in CardInput, params RenderParams, assets cardAssets, cf *cardFonts) *image.RGBA {
	dc := gg.NewContext(CanvasWidth, CanvasHeight)

	drawBackdrop(dc, params)
	drawWatermark(dc, cf, params)
	drawRadarGrid(dc)
	drawMatchAvgPolygon(dc, in.MatchAvg, params.MatchAvgAlpha)
	drawPlayerPolygon(dc, in.TeamKey, in.Normalized, params)
	drawDimensionLabels(dc, in, cf)
	drawConnectionLine(dc)
	drawPortraitSection(dc, in, assets, cf, params)

	if rgba, ok := dc.Image().(*image.RGBA); ok {
		return rgba
	}
	bounds := dc.Image().Bounds()
	out := image.NewRGBA(bounds)
	draw.Draw(out, bounds, dc.Image(), bounds.Min, draw.Src)
	return out
}

func drawBackdrop(dc *gg.Context, params RenderParams) {
	grad := gg.NewLinearGradient(0, 0, CanvasWidth, CanvasHeight)
	grad.AddColorStop(0, color.RGBA{11, 14, 20, 255})
	grad.AddColorStop(0.55, color.RGBA{16, 22, 32, 255})
	grad.AddColorStop(1, color.RGBA{10, 13, 19, 255})
	dc.SetFillStyle(grad)
	dc.DrawRectangle(0, 0, CanvasWidth, CanvasHeight)
	dc.Fill()

	// Subtle geometric starfield rays with gentle breathing modulation.
	rayAlpha := 0.08 + 0.04*math.Sin(params.PulsePhase*2*math.Pi)
	dc.SetRGBA(1.0, 0.79, 0.36, rayAlpha)
	dc.SetLineWidth(1.5)
	nodes := [][2]float64{
		{180, 140}, {520, 190}, {1080, 120}, {1380, 320},
		{2350, 180}, {2420, 1180}, {1260, 1260}, {210, 1290},
	}
	for i := 0; i < len(nodes); i++ {
		j := (i + 1) % len(nodes)
		dc.DrawLine(nodes[i][0], nodes[i][1], nodes[j][0], nodes[j][1])
		dc.Stroke()
		dc.DrawCircle(nodes[i][0], nodes[i][1], 4)
		dc.Fill()
	}

	// Outer glowing cage border.
	dc.SetRGBA(1.0, 0.79, 0.36, 0.28)
	dc.SetLineWidth(3)
	dc.DrawRoundedRectangle(36, 36, CanvasWidth-72, CanvasHeight-72, 18)
	dc.Stroke()
}

func drawWatermark(dc *gg.Context, cf *cardFonts, params RenderParams) {
	alpha := 0.05 + 0.025*math.Sin(params.PulsePhase*2*math.Pi)
	dc.SetFontFace(cf.watermark)
	dc.SetRGBA(1.0, 0.82, 0.42, alpha)
	dc.DrawStringAnchored("STRIKEYE", RadarCenterX, RadarCenterY-40, 0.5, 0.5)
}

func axisAngle(idx int) float64 {
	return -math.Pi/2.0 + float64(idx)*math.Pi/3.0
}

func hexPoint(cx, cy, r float64, idx int) (float64, float64) {
	a := axisAngle(idx)
	return cx + r*math.Cos(a), cy + r*math.Sin(a)
}

func drawRadarGrid(dc *gg.Context) {
	// Inner gray level rings.
	for _, frac := range []float64{0.25, 0.50, 0.75} {
		r := RadarMaxR * frac
		dc.SetRGBA(0.55, 0.62, 0.72, 0.28)
		dc.SetLineWidth(2)
		for i := 0; i < 6; i++ {
			x, y := hexPoint(RadarCenterX, RadarCenterY, r, i)
			if i == 0 {
				dc.MoveTo(x, y)
			} else {
				dc.LineTo(x, y)
			}
		}
		dc.ClosePath()
		dc.Stroke()
	}

	// Blue full-scale outer ring (1.0).
	for _, pass := range []struct {
		w float64
		a float64
	}{{10, 0.16}, {5, 0.35}, {2.5, 0.95}} {
		dc.SetRGBA(0.22, 0.74, 0.97, pass.a)
		dc.SetLineWidth(pass.w)
		for i := 0; i < 6; i++ {
			x, y := hexPoint(RadarCenterX, RadarCenterY, RadarMaxR, i)
			if i == 0 {
				dc.MoveTo(x, y)
			} else {
				dc.LineTo(x, y)
			}
		}
		dc.ClosePath()
		dc.Stroke()
	}

	// Six radial axis spokes extending slightly past 1.0 ring.
	dc.SetRGBA(0.55, 0.62, 0.72, 0.32)
	dc.SetLineWidth(1.5)
	for i := 0; i < 6; i++ {
		x, y := hexPoint(RadarCenterX, RadarCenterY, RadarMaxR*1.08, i)
		dc.DrawLine(RadarCenterX, RadarCenterY, x, y)
		dc.Stroke()
	}
}

func drawMatchAvgPolygon(dc *gg.Context, avg NormalizedValues, alpha float64) {
	if alpha <= 0 {
		return
	}
	vals := avg.Slice()
	for _, pass := range []struct {
		w float64
		a float64
	}{{9, 0.18 * alpha}, {3.5, 0.92 * alpha}} {
		dc.SetRGBA(0.94, 0.27, 0.27, pass.a)
		dc.SetLineWidth(pass.w)
		for i := 0; i < 6; i++ {
			r := RadarMaxR * vals[i]
			x, y := hexPoint(RadarCenterX, RadarCenterY, r, i)
			if i == 0 {
				dc.MoveTo(x, y)
			} else {
				dc.LineTo(x, y)
			}
		}
		dc.ClosePath()
		dc.Stroke()
	}
}

func teamColors(teamKey string) (r, g, b float64) {
	upper := strings.ToUpper(strings.TrimSpace(teamKey))
	if upper == "T" || upper == "TERRORIST" || upper == "TEAM_B" {
		return 0.96, 0.62, 0.04 // T orange-amber #F59E0B
	}
	return 0.22, 0.74, 0.97 // CT cyan-blue #38BDF8
}

func drawPlayerPolygon(dc *gg.Context, teamKey string, norm NormalizedValues, params RenderParams) {
	vals := norm.Slice()
	scale := params.Scale
	if scale < 0 {
		scale = 0
	}
	cr, cg, cb := teamColors(teamKey)

	tracePoly := func() {
		for i := 0; i < 6; i++ {
			r := RadarMaxR * vals[i] * scale
			x, y := hexPoint(RadarCenterX, RadarCenterY, r, i)
			if i == 0 {
				dc.MoveTo(x, y)
			} else {
				dc.LineTo(x, y)
			}
		}
		dc.ClosePath()
	}

	// Translucent fill.
	tracePoly()
	dc.SetRGBA(cr, cg, cb, 0.26)
	dc.Fill()

	// Outer glow + crisp white core stroke.
	for _, pass := range []struct {
		r, g, b, w, a float64
	}{
		{cr, cg, cb, 14, 0.22},
		{cr, cg, cb, 6, 0.60},
		{1.0, 1.0, 1.0, 2.5, 0.92},
	} {
		tracePoly()
		dc.SetRGBA(pass.r, pass.g, pass.b, pass.a)
		dc.SetLineWidth(pass.w)
		dc.Stroke()
	}

	// Vertex data dots (with subtle flash during fast-out phase).
	for i := 0; i < 6; i++ {
		r := RadarMaxR * vals[i] * scale
		x, y := hexPoint(RadarCenterX, RadarCenterY, r, i)
		if params.FlashAlpha > 0 {
			dc.SetRGBA(1, 1, 1, params.FlashAlpha*0.55)
			dc.DrawCircle(x, y, 13)
			dc.Fill()
		}
		dc.SetRGBA(1, 1, 1, 0.95)
		dc.DrawCircle(x, y, 6)
		dc.Fill()
	}
}

func drawDimensionLabels(dc *gg.Context, in CardInput, cf *cardFonts) {
	raw := in.Raw.Slice()
	labelR := RadarMaxR + 135.0
	for i, spec := range Dimensions {
		lx, ly := hexPoint(RadarCenterX, RadarCenterY, labelR, i)
		dc.SetFontFace(cf.dimLabel)
		dc.SetRGBA(0.82, 0.87, 0.94, 0.95)
		dc.DrawStringAnchored(spec.Label, lx, ly-28, 0.5, 0.5)

		dc.SetFontFace(cf.dimVal)
		dc.SetRGBA(1.0, 0.84, 0.40, 1.0)
		dc.DrawStringAnchored(FormatValue(spec, raw[i]), lx, ly+10, 0.5, 0.5)

		dc.SetFontFace(cf.dimFull)
		dc.SetRGBA(0.55, 0.65, 0.78, 0.88)
		dc.DrawStringAnchored(spec.FullText, lx, ly+44, 0.5, 0.5)
	}
}

func drawConnectionLine(dc *gg.Context) {
	dc.SetRGBA(0.85, 0.66, 0.28, 0.42)
	dc.SetLineWidth(2)
	dc.DrawLine(1280, 140, 1280, CanvasHeight-140)
	dc.Stroke()
}

func drawPortraitSection(dc *gg.Context, in CardInput, assets cardAssets, cf *cardFonts, params RenderParams) {
	if assets.teamLogo != nil {
		w := assets.teamLogo.Bounds().Dx()
		h := assets.teamLogo.Bounds().Dy()
		dc.DrawImage(assets.teamLogo, int(PortraitCX)-w/2, int(PortraitCY)-h/2)
	}

	r := PortraitSize / 2.0
	if assets.portrait != nil {
		w := assets.portrait.Bounds().Dx()
		h := assets.portrait.Bounds().Dy()
		dc.DrawImage(assets.portrait, int(PortraitCX)-w/2, int(PortraitCY)-h/2)
	} else {
		// Dark circular plate with player's initial letter.
		dc.SetRGBA(0.11, 0.14, 0.20, 0.95)
		dc.DrawCircle(PortraitCX, PortraitCY, r)
		dc.Fill()

		initial := "?"
		trimmed := strings.TrimSpace(in.PlayerName)
		if trimmed != "" {
			ch, _ := utf8.DecodeRuneInString(trimmed)
			initial = strings.ToUpper(string(ch))
		}
		dc.SetFontFace(cf.initial)
		dc.SetRGBA(1.0, 0.79, 0.36, 0.90)
		dc.DrawStringAnchored(initial, PortraitCX, PortraitCY+10, 0.5, 0.5)
	}

	// Gold portrait ring with subtle pulse.
	ringAlpha := 0.88 + 0.12*math.Sin(params.PulsePhase*2*math.Pi)
	dc.SetRGBA(1.0, 0.79, 0.36, ringAlpha*0.30)
	dc.SetLineWidth(18)
	dc.DrawCircle(PortraitCX, PortraitCY, r+4)
	dc.Stroke()

	dc.SetRGBA(1.0, 0.79, 0.36, ringAlpha)
	dc.SetLineWidth(6)
	dc.DrawCircle(PortraitCX, PortraitCY, r)
	dc.Stroke()

	// Player name, team label, and K/D/A summary.
	name := strings.TrimSpace(in.PlayerName)
	if name == "" {
		name = "Unknown Player"
	}
	dc.SetFontFace(cf.title)
	dc.SetRGBA(0.97, 0.98, 1.0, 1.0)
	dc.DrawStringAnchored(name, PortraitCX, PortraitCY+r+105, 0.5, 0.5)

	team := strings.TrimSpace(in.TeamLabel)
	if team == "" {
		team = strings.ToUpper(strings.TrimSpace(in.TeamKey))
	}
	summary := fmt.Sprintf("%s  |  %d / %d / %d  |  Rating %.2f",
		team, in.Kills, in.Deaths, in.Assists, in.Raw.Rating)
	dc.SetFontFace(cf.sub)
	dc.SetRGBA(0.75, 0.82, 0.91, 0.95)
	dc.DrawStringAnchored(summary, PortraitCX, PortraitCY+r+185, 0.5, 0.5)
}

func loadImageFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func resizeSquareRGBA(src image.Image, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	sb := src.Bounds()
	sw := sb.Dx()
	sh := sb.Dy()
	if sw <= 0 || sh <= 0 || size <= 0 {
		return dst
	}
	for y := 0; y < size; y++ {
		sy := sb.Min.Y + y*sh/size
		for x := 0; x < size; x++ {
			sx := sb.Min.X + x*sw/size
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func circularCropRGBA(src image.Image, size int) *image.RGBA {
	scaled := resizeSquareRGBA(src, size)
	cx := float64(size) / 2.0
	cy := float64(size) / 2.0
	r := float64(size) / 2.0
	for y := 0; y < size; y++ {
		dy := float64(y) + 0.5 - cy
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - cx
			dist := math.Hypot(dx, dy)
			if dist > r {
				scaled.SetRGBA(x, y, color.RGBA{})
			} else if dist > r-2.0 {
				c := scaled.RGBAAt(x, y)
				edge := (r - dist) / 2.0
				c.A = uint8(float64(c.A) * edge)
				scaled.SetRGBA(x, y, c)
			}
		}
	}
	return scaled
}

func circularFeatheredRGBA(src image.Image, size int, baseAlpha float64) *image.RGBA {
	scaled := resizeSquareRGBA(src, size)
	cx := float64(size) / 2.0
	cy := float64(size) / 2.0
	r := float64(size) / 2.0
	for y := 0; y < size; y++ {
		dy := float64(y) + 0.5 - cy
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - cx
			dist := math.Hypot(dx, dy)
			if dist >= r {
				scaled.SetRGBA(x, y, color.RGBA{})
				continue
			}
			// Smooth radial feathering from 45% radius outward to 100%.
			feather := 1.0
			if dist > r*0.45 {
				t := (dist - r*0.45) / (r * 0.55)
				feather = 0.5 * (1.0 + math.Cos(math.Pi*t))
			}
			c := scaled.RGBAAt(x, y)
			c.A = uint8(float64(c.A) * baseAlpha * feather)
			scaled.SetRGBA(x, y, c)
		}
	}
	return scaled
}
