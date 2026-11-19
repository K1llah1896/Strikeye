package components

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo/ui"
	xdraw "golang.org/x/image/draw"
	"strikeye/internal/storage"
	"strikeye/internal/ui/theme"
)

// RadarHexPreview caches and renders a 2560x1440 CS Data Radar PNG card inside MyGo UI.
type RadarHexPreview struct {
	mu           sync.Mutex
	lastPath     string
	loadSeq      uint64
	bitmap       *ui.Bitmap // owned and mutated exclusively on the UI main thread in Render
	pendingImg   image.Image
	pendingClear bool
	pendingDirty bool
	loadErr      string
}

// NewRadarHexPreview constructs an empty radar card preview widget.
func NewRadarHexPreview() *RadarHexPreview {
	return &RadarHexPreview{}
}

func decodePreviewImage(cardPath string) (image.Image, error) {
	raw, err := os.ReadFile(filepath.Clean(cardPath))
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	// Downscale 2560x1440 cards to 1280x720 for crisp yet lightweight UI preview rendering.
	if b.Dx() > 1280 || b.Dy() > 720 {
		dst := image.NewRGBA(image.Rect(0, 0, 1280, 720))
		xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
		return dst, nil
	}
	return img, nil
}

// SetCard loads (or reloads) a radar card PNG from disk and stages it for main-thread GPU upload.
func (p *RadarHexPreview) SetCard(cardPath string, forceReload bool) {
	p.mu.Lock()
	if !forceReload && cardPath == p.lastPath && (p.bitmap != nil || p.pendingImg != nil) && p.loadErr == "" {
		p.mu.Unlock()
		return
	}
	p.loadSeq++
	seq := p.loadSeq
	p.lastPath = cardPath
	p.loadErr = ""
	if cardPath == "" {
		p.pendingImg = nil
		p.pendingClear = true
		p.pendingDirty = true
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	img, err := decodePreviewImage(cardPath)

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.loadSeq != seq {
		return
	}
	if err != nil {
		p.pendingImg = nil
		p.pendingClear = true
		p.pendingDirty = true
		p.loadErr = err.Error()
		return
	}
	p.pendingImg = img
	p.pendingClear = false
	p.pendingDirty = true
	p.loadErr = ""
}

// SetCardAsync decodes cardPath in a background goroutine and invokes onReady when staged.
func (p *RadarHexPreview) SetCardAsync(cardPath string, forceReload bool, onReady func()) {
	p.mu.Lock()
	if !forceReload && cardPath == p.lastPath && (p.bitmap != nil || p.pendingImg != nil) && p.loadErr == "" {
		p.mu.Unlock()
		return
	}
	p.loadSeq++
	seq := p.loadSeq
	p.lastPath = cardPath
	p.loadErr = ""
	if cardPath == "" {
		p.pendingImg = nil
		p.pendingClear = true
		p.pendingDirty = true
		p.mu.Unlock()
		if onReady != nil {
			onReady()
		}
		return
	}
	p.mu.Unlock()

	go func() {
		img, err := decodePreviewImage(cardPath)
		p.mu.Lock()
		stale := p.loadSeq != seq
		if !stale {
			if err != nil {
				p.pendingImg = nil
				p.pendingClear = true
				p.pendingDirty = true
				p.loadErr = err.Error()
			} else {
				p.pendingImg = img
				p.pendingClear = false
				p.pendingDirty = true
				p.loadErr = ""
			}
		}
		p.mu.Unlock()
		if !stale && onReady != nil {
			onReady()
		}
	}()
}

func (p *RadarHexPreview) flushPendingLocked() {
	if !p.pendingDirty {
		return
	}
	if p.pendingClear || p.pendingImg == nil {
		p.bitmap = nil
	} else if p.bitmap == nil {
		p.bitmap = ui.NewBitmap(p.pendingImg)
	} else {
		p.bitmap.Update(p.pendingImg)
	}
	p.pendingImg = nil
	p.pendingClear = false
	p.pendingDirty = false
}

// Render draws the radar card preview or an empty/error placeholder.
// It flushes any staged image into ui.Bitmap on the main UI thread.
func (p *RadarHexPreview) Render(c *ui.Context, card *storage.RadarCard, width, height float32) {
	p.mu.Lock()
	p.flushPendingLocked()
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
				theme.MutedText(c, "雷达卡片加载中...")
			})
		}
	})
}
