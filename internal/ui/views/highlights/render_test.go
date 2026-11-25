package highlights

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/storage"
	"strikeye/internal/ui/renderutil"
)

// TestRenderHighlightsView renders the montage tab with audio-track rows filled,
// which exercises the multi-track editor added in `montage-audio-tracks`.
func TestRenderHighlightsView(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	v := NewView(db, nil, "", nil)
	v.activeTab = "montage"
	v.audioTracks = []audioTrackDraft{
		{Path: "/tmp/voice.wav", OffsetText: "0.5", VolumeText: "0.7"},
		{Path: "", VolumeText: "1"},
		{Path: "/tmp/crowd.wav", Mute: true},
	}
	renderutil.AssertRendered(t, 40, func(c *ui.Context) { v.Render(c) })

	v.activeTab = "library"
	renderutil.AssertRendered(t, 40, func(c *ui.Context) { v.Render(c) })
}

// TestRenderMontageSuitePreview renders the export card that hosts the audio
// editor together with the item list.
func TestRenderMontageExportCard(t *testing.T) {
	db, err := storage.OpenDefault(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	v := NewView(db, nil, "", nil)
	v.activeTab = "montage"
	v.title = "集锦"
	v.bgmPath = "/tmp/bgm.mp3"
	v.statusMsg = "示例"
	v.exportStatus = "running"
	v.exportProgress = 0.42
	renderutil.AssertRendered(t, 30, func(c *ui.Context) { v.Render(c) })
}
