package components

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo/ui"
	"strikeye/internal/storage"
	"strikeye/internal/ui/theme"
)

// RadarHexPreview caches and renders a 2560x1440 CS Data Radar PNG card inside MyGo UI.
type RadarHexPreview struct {
	mu       sync.Mutex
	lastPath string
	bitmap   *ui.Bitmap
	loadErr  string
}

// NewRadarHexPreview constructs an empty radar card preview widget.
func NewRadarHexPreview() *RadarHexPreview {
	return &RadarHexPreview{}
}

// SetCard loads (or reloads) a radar card PNG from disk into a GPU bitmap.
func (p *RadarHexPreview) SetCard(cardPath string, forceReload bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !forceReload && cardPath == p.lastPath && p.bitmap != nil {
		return
	}
	p.lastPath = cardPath
	p.loadErr = ""
	if cardPath == "" {
		p.bitmap = nil
		return
	}
	raw, err := os.ReadFile(filepath.Clean(cardPath))
	if err != nil {
		p.bitmap = nil
		p.loadErr = err.Error()
		return
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		p.bitmap = nil
		p.loadErr = err.Error()
		return
	}
	if p.bitmap == nil {
		p.bitmap = ui.NewBitmap(img)
	} else {
		p.bitmap.Update(img)
	}
}

// Render draws the radar card preview or an empty/error placeholder.
func (p *RadarHexPreview) Render(c *ui.Context, card *storage.RadarCard, width, height float32) {
	p.mu.Lock()
	bmp := p.bitmap
	errStr := p.loadErr
	p.mu.Unlock()

	theme.Card(c, func() {
		if card == nil {
			ui.Column(c).Width(width).Height(height).Center().Children(func() {
				theme.MutedText(c, "请选择选手或点击「一键生成 10 人战力雷达图」")
			})
			return
		}
		ui.Row(c).FillWidth().Gap(10).Children(func() {
			ui.Text(c, card.PlayerName).FontSize(15).Bold().TextColor(theme.TextPrimary)
			if card.AnimVideoPath != nil && *card.AnimVideoPath != "" {
				theme.Capsule(c, "4.0s 开场动画已就绪", theme.StatusGreen)
			} else {
				theme.Capsule(c, "仅静态卡片", theme.AccentCyan)
			}
		})
		if errStr != "" {
			ui.Text(c, "卡片加载失败: "+errStr).FontSize(12).TextColor(theme.StatusRed)
			return
		}
		if bmp != nil {
			ui.Image(c, bmp).Width(width).Height(height)
		} else {
			ui.Column(c).Width(width).Height(height).Center().Children(func() {
				theme.MutedText(c, "雷达卡片尚未生成")
			})
		}
	})
}
