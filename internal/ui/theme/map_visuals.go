package theme

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"image/png"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	"strikeye/assets/maps"
)

// MapVisual describes the display title, signature accent color, and fallback backdrop for a CS2 map.
type MapVisual struct {
	DisplayName string
	Accent      ui.Color
	FallbackBg  ui.Color
}

var (
	mapAssetMu    sync.RWMutex
	mapIconCache  = map[string]*ui.Bitmap{}
	mapSceneCache = map[string]*ui.Bitmap{}
)

var mapVisualTable = map[string]MapVisual{
	"de_mirage":   {DisplayName: "MIRAGE", Accent: ui.Hex("#F59E0B"), FallbackBg: ui.Hex("#3B210B")},
	"de_inferno":  {DisplayName: "INFERNO", Accent: ui.Hex("#EF4444"), FallbackBg: ui.Hex("#3F1215")},
	"de_dust2":    {DisplayName: "DUST II", Accent: ui.Hex("#EAB308"), FallbackBg: ui.Hex("#3A2B09")},
	"de_ancient":  {DisplayName: "ANCIENT", Accent: ui.Hex("#22C55E"), FallbackBg: ui.Hex("#092917")},
	"de_anubis":   {DisplayName: "ANUBIS", Accent: ui.Hex("#14B8A6"), FallbackBg: ui.Hex("#0A2A2B")},
	"de_nuke":     {DisplayName: "NUKE", Accent: ui.Hex("#38BDF8"), FallbackBg: ui.Hex("#0A2239")},
	"de_overpass": {DisplayName: "OVERPASS", Accent: ui.Hex("#6366F1"), FallbackBg: ui.Hex("#17193A")},
	"de_train":    {DisplayName: "TRAIN", Accent: ui.Hex("#94A3B8"), FallbackBg: ui.Hex("#18202C")},
	"de_vertigo":  {DisplayName: "VERTIGO", Accent: ui.Hex("#0EA5E9"), FallbackBg: ui.Hex("#0B243B")},
	"de_cache":    {DisplayName: "CACHE", Accent: ui.Hex("#10B981"), FallbackBg: ui.Hex("#0B2920")},
	"cs_italy":    {DisplayName: "ITALY", Accent: ui.Hex("#F97316"), FallbackBg: ui.Hex("#3B1B0B")},
	"cs_office":   {DisplayName: "OFFICE", Accent: ui.Hex("#60A5FA"), FallbackBg: ui.Hex("#10223A")},
}

// ResolveMapVisual returns the display title, accent color, and fallback background for mapName.
func ResolveMapVisual(mapName string) MapVisual {
	key := strings.ToLower(strings.TrimSpace(mapName))
	if spec, ok := mapVisualTable[key]; ok {
		return spec
	}
	display := "UNKNOWN"
	if key != "" {
		trimmed := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(key, "de_"), "cs_"), "ar_")
		display = strings.ToUpper(trimmed)
	}
	return MapVisual{
		DisplayName: display,
		Accent:      AccentAmber,
		FallbackBg:  ui.Hex("#111C2D"),
	}
}

// CachedMapIconBitmap lazily decodes and caches the official CS2 map logo PNG for mapName.
func CachedMapIconBitmap(mapName string) *ui.Bitmap {
	key := strings.ToLower(strings.TrimSpace(mapName))
	if key == "" {
		return nil
	}
	mapAssetMu.RLock()
	bmp, ok := mapIconCache[key]
	mapAssetMu.RUnlock()
	if ok {
		return bmp
	}

	var decoded *ui.Bitmap
	if raw, found := maps.LoadMapIconPNG(key); found {
		if img, err := png.Decode(bytes.NewReader(raw)); err == nil {
			decoded = ui.NewBitmap(img)
		}
	}

	mapAssetMu.Lock()
	mapIconCache[key] = decoded
	mapAssetMu.Unlock()
	return decoded
}

// CachedMapSceneBitmap lazily decodes and caches the official CS2 in-game map scene JPEG for mapName.
func CachedMapSceneBitmap(mapName string) *ui.Bitmap {
	key := strings.ToLower(strings.TrimSpace(mapName))
	if key == "" {
		return nil
	}
	mapAssetMu.RLock()
	bmp, ok := mapSceneCache[key]
	mapAssetMu.RUnlock()
	if ok {
		return bmp
	}

	var decoded *ui.Bitmap
	if raw, found := maps.LoadMapSceneJPG(key); found {
		if img, err := jpeg.Decode(bytes.NewReader(raw)); err == nil {
			decoded = ui.NewBitmap(img)
		}
	}

	mapAssetMu.Lock()
	mapSceneCache[key] = decoded
	mapAssetMu.Unlock()
	return decoded
}

// RenderMapCrest renders the borderless official CS2 map logo (or a clean radar icon fallback).
func RenderMapCrest(c *ui.Context, mapName string, spec MapVisual) {
	iconBmp := CachedMapIconBitmap(mapName)
	if iconBmp != nil {
		ui.Image(c, iconBmp).Width(48).Height(48).Fit(ui.Contain)
		return
	}
	ui.Box(c).Width(44).Height(44).Center().Children(func() {
		RenderIcon(c, IconRadar, 24, spec.Accent)
	})
}

// RenderMapScoreBanner renders the shared csnades.gg-inspired match scoreline banner
// with official map scene backdrop, left map logo, centered scoreline, and optional right slot.
func RenderMapScoreBanner(c *ui.Context, mapName, teamA, teamB string, scoreA, scoreB int, rightSlot func()) {
	spec := ResolveMapVisual(mapName)
	sceneBmp := CachedMapSceneBitmap(mapName)
	if teamA == "" {
		teamA = "TEAM CT"
	}
	if teamB == "" {
		teamB = "TEAM T"
	}

	ui.Box(c).FillWidth().Height(82).
		Background(spec.FallbackBg).
		Border(1, ui.RGBA(spec.Accent.R, spec.Accent.G, spec.Accent.B, 0.42)).
		Radius(16).
		Clip().
		Children(func() {
			if sceneBmp != nil {
				ui.Image(c, sceneBmp).Fill().Fit(ui.Cover).IgnoreInput(true)
			}
			ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).IgnoreInput(true).LinearGradient(ui.LinearGradient{
				Angle: 90,
				Stops: []ui.GradientStop{
					{Color: ui.RGBA(6, 10, 18, 0.64), At: 0},
					{Color: ui.RGBA(6, 10, 18, 0.28), At: 0.5},
					{Color: ui.RGBA(6, 10, 18, 0.64), At: 1},
				},
			})
			RowCenter(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Center().Gap(14).IgnoreInput(true).Children(func() {
				ui.Text(c, teamA).FontSize(15).Bold().TextColor(TeamCT)
				RowCenter(c).Background(ui.RGBA(8, 12, 20, 0.80)).Border(1, RimBorder).Radius(12).PaddingX(14).PaddingY(4).Gap(10).Children(func() {
					ScoreText(c, fmt.Sprintf("%d", scoreA)).FontSize(28).TextColor(TextPrimary)
					ScoreText(c, ":").FontSize(20).TextColor(TextMuted)
					ScoreText(c, fmt.Sprintf("%d", scoreB)).FontSize(28).TextColor(TextPrimary)
				})
				ui.Text(c, teamB).FontSize(15).Bold().TextColor(TeamT)
			})
			RowCenter(c).Absolute().Left(16).Top(0).Bottom(0).Gap(12).IgnoreInput(true).Children(func() {
				RenderMapCrest(c, mapName, spec)
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, spec.DisplayName).Font("Rajdhani").FontSize(16).Bold().LetterSpacing(1.3).TextColor(TextPrimary)
					ui.Text(c, strings.ToLower(mapName)).Font("Rajdhani").FontSize(10).LetterSpacing(0.8).TextColor(spec.Accent)
				})
			})
			if rightSlot != nil {
				RowCenter(c).Absolute().Right(16).Top(0).Bottom(0).IgnoreInput(true).Children(rightSlot)
			}
		})
}
