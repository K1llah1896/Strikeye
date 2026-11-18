// Package fonts embeds Strikeye's open-source Latin and CJK fonts for pure-Go 2D rendering.
package fonts

import (
	_ "embed"
	"fmt"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

//go:embed Rajdhani-Bold.ttf
var rajdhaniBoldTTF []byte

//go:embed Rajdhani-SemiBold.ttf
var rajdhaniSemiBoldTTF []byte

//go:embed NotoSansSC-Medium.ttf
var notoSansSCMediumTTF []byte

//go:embed NotoSansSC-Bold.ttf
var notoSansSCBoldTTF []byte

// FontID identifies one of Strikeye's embedded OpenType fonts.
type FontID string

const (
	RajdhaniBold     FontID = "Rajdhani-Bold"
	RajdhaniSemiBold FontID = "Rajdhani-SemiBold"
	NotoSansSCMedium FontID = "NotoSansSC-Medium"
	NotoSansSCBold   FontID = "NotoSansSC-Bold"
)

var (
	parseOnce sync.Once
	parsed    map[FontID]*opentype.Font
	parseErr  error
)

func initFonts() {
	raw := map[FontID][]byte{
		RajdhaniBold:     rajdhaniBoldTTF,
		RajdhaniSemiBold: rajdhaniSemiBoldTTF,
		NotoSansSCMedium: notoSansSCMediumTTF,
		NotoSansSCBold:   notoSansSCBoldTTF,
	}
	parsed = make(map[FontID]*opentype.Font, len(raw))
	for id, data := range raw {
		f, err := opentype.Parse(data)
		if err != nil {
			parseErr = fmt.Errorf("parse embedded font %s: %w", id, err)
			return
		}
		parsed[id] = f
	}
}

// RawBytes returns the embedded TTF bytes for id.
func RawBytes(id FontID) ([]byte, bool) {
	switch id {
	case RajdhaniBold:
		return rajdhaniBoldTTF, true
	case RajdhaniSemiBold:
		return rajdhaniSemiBoldTTF, true
	case NotoSansSCMedium:
		return notoSansSCMediumTTF, true
	case NotoSansSCBold:
		return notoSansSCBoldTTF, true
	default:
		return nil, false
	}
}

// Parse returns the parsed opentype.Font for id.
func Parse(id FontID) (*opentype.Font, error) {
	parseOnce.Do(initFonts)
	if parseErr != nil {
		return nil, parseErr
	}
	f, ok := parsed[id]
	if !ok {
		return nil, fmt.Errorf("unknown font id %q", id)
	}
	return f, nil
}

// NewFace creates a font.Face at the given point size (72 DPI) with full hinting.
func NewFace(id FontID, size float64) (font.Face, error) {
	f, err := Parse(id)
	if err != nil {
		return nil, err
	}
	if size <= 0 {
		size = 16
	}
	return opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}
