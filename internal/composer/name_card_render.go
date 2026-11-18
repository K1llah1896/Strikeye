package composer

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/fogleman/gg"

	"strikeye/assets/fonts"
)

const (
	NameCardWidth  = 560
	NameCardHeight = 136
)

// NameCardInput configures a lower-third player name card PNG.
type NameCardInput struct {
	PlayerName   string
	TeamKey      string
	RoundNumber  int
	HighlightTag string
	AvatarPath   string
}

// RenderNameCardPNG renders a transparent-background lower-third player name card PNG.
func RenderNameCardPNG(in NameCardInput, outputPath string) error {
	rgba, err := RenderNameCardRGBA(in)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create name card dir: %w", err)
	}
	tmp := outputPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create name card file: %w", err)
	}
	if err := png.Encode(f, rgba); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("encode name card png: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, outputPath)
}

// RenderNameCardRGBA renders a 560x136 transparent-background lower-third player name card.
func RenderNameCardRGBA(in NameCardInput) (*image.RGBA, error) {
	nameFace, err := fonts.NewFace(fonts.NotoSansSCBold, 34)
	if err != nil {
		return nil, err
	}
	defer nameFace.Close()

	tagFace, err := fonts.NewFace(fonts.NotoSansSCMedium, 22)
	if err != nil {
		return nil, err
	}
	defer tagFace.Close()

	initFace, err := fonts.NewFace(fonts.NotoSansSCBold, 40)
	if err != nil {
		return nil, err
	}
	defer initFace.Close()

	dc := gg.NewContext(NameCardWidth, NameCardHeight)

	// Accent bar & dark glassmorphic card background (transparent outside rounded rect).
	dc.SetRGBA(0.06, 0.08, 0.12, 0.86)
	dc.DrawRoundedRectangle(4, 4, NameCardWidth-8, NameCardHeight-8, 18)
	dc.Fill()

	ar, ag, ab := 0.22, 0.74, 0.97 // CT cyan-blue
	if strings.EqualFold(strings.TrimSpace(in.TeamKey), "T") ||
		strings.EqualFold(strings.TrimSpace(in.TeamKey), "TERRORIST") {
		ar, ag, ab = 0.96, 0.62, 0.04 // T amber
	}
	dc.SetRGBA(ar, ag, ab, 0.95)
	dc.DrawRoundedRectangle(4, 16, 8, NameCardHeight-32, 4)
	dc.Fill()

	dc.SetRGBA(ar, ag, ab, 0.45)
	dc.SetLineWidth(2)
	dc.DrawRoundedRectangle(4, 4, NameCardWidth-8, NameCardHeight-8, 18)
	dc.Stroke()

	// Circular avatar (88x88) at (70, 68).
	const (
		acx   = 72.0
		acy   = 68.0
		asize = 88
	)
	var avatarRGBA *image.RGBA
	if in.AvatarPath != "" {
		if f, err := os.Open(in.AvatarPath); err == nil {
			img, _, decErr := image.Decode(f)
			_ = f.Close()
			if decErr == nil {
				avatarRGBA = cropCircle(img, asize)
			}
		}
	}
	if avatarRGBA != nil {
		dc.DrawImage(avatarRGBA, int(acx)-asize/2, int(acy)-asize/2)
	} else {
		dc.SetRGBA(0.14, 0.18, 0.26, 0.95)
		dc.DrawCircle(acx, acy, asize/2.0)
		dc.Fill()
		initial := "?"
		if trimmed := strings.TrimSpace(in.PlayerName); trimmed != "" {
			ch, _ := utf8.DecodeRuneInString(trimmed)
			initial = strings.ToUpper(string(ch))
		}
		dc.SetFontFace(initFace)
		dc.SetRGBA(1.0, 0.82, 0.40, 0.95)
		dc.DrawStringAnchored(initial, acx, acy+2, 0.5, 0.5)
	}
	dc.SetRGBA(ar, ag, ab, 0.95)
	dc.SetLineWidth(3)
	dc.DrawCircle(acx, acy, asize/2.0)
	dc.Stroke()

	// Player name and round/highlight tag.
	name := strings.TrimSpace(in.PlayerName)
	if name == "" {
		name = "Unknown"
	}
	dc.SetFontFace(nameFace)
	dc.SetRGBA(0.97, 0.98, 1.0, 1.0)
	dc.DrawStringAnchored(name, 134, 48, 0.0, 0.5)

	sub := strings.TrimSpace(in.HighlightTag)
	if in.RoundNumber > 0 {
		if sub != "" {
			sub = fmt.Sprintf("R%d · %s", in.RoundNumber, sub)
		} else {
			sub = fmt.Sprintf("R%d", in.RoundNumber)
		}
	}
	if sub != "" {
		dc.SetFontFace(tagFace)
		dc.SetRGBA(0.96, 0.75, 0.25, 0.95)
		dc.DrawStringAnchored(sub, 134, 92, 0.0, 0.5)
	}

	if rgba, ok := dc.Image().(*image.RGBA); ok {
		return rgba, nil
	}
	return image.NewRGBA(image.Rect(0, 0, NameCardWidth, NameCardHeight)), nil
}

func cropCircle(src image.Image, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw <= 0 || sh <= 0 || size <= 0 {
		return dst
	}
	cx := float64(size) / 2.0
	cy := float64(size) / 2.0
	r := float64(size) / 2.0
	for y := 0; y < size; y++ {
		sy := sb.Min.Y + y*sh/size
		dy := float64(y) + 0.5 - cy
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - cx
			if math.Hypot(dx, dy) > r {
				dst.SetRGBA(x, y, color.RGBA{})
				continue
			}
			sx := sb.Min.X + x*sw/size
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
