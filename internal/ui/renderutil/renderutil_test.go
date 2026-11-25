package renderutil

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAssertRenderedWithBox(t *testing.T) {
	colors := AssertRendered(t, 2, func(c *ui.Context) {
		ui.Column(c).FillWidth().FillHeight().Background(ui.RGBA(20, 30, 40, 1)).Children(func() {
			ui.Text(c, "hello").FontSize(24)
			ui.Text(c, "world").FontSize(24).TextColor(ui.RGBA(255, 200, 0, 1))
		})
	})
	if colors < 2 {
		t.Fatalf("colors = %d", colors)
	}
}
