package components

import (
	"github.com/egoist/mygo/ui"
	"strikeye/internal/parser"
	"strikeye/internal/ui/theme"
)

// KeyHUDState holds the resolved button mask and capability for the current playback tick.
type KeyHUDState struct {
	Visible    bool
	HasData    bool
	Buttons    uint16
	PlayerName string
}

// ResolveButtonsAtTick finds the active button bitmask for targetSlot at tick
// from a slice of InputEdges ordered by tick.
func ResolveButtonsAtTick(edges []parser.InputEdge, targetSlot uint8, tick int64, inputCap uint8) (uint16, bool) {
	if inputCap == 0 {
		return 0, false
	}
	var mask uint16
	for _, e := range edges {
		if e.PlayerSlot != targetSlot {
			continue
		}
		if e.Tick > tick {
			break
		}
		mask = e.Buttons
	}
	return mask, true
}

// RenderKeyHUD renders the compact real-time key overlay widget (WASD / Crouch / Jump / Walk / Fire / Scope)
// or an explicit notice when the demo lacks underlying input command data.
func RenderKeyHUD(c *ui.Context, st KeyHUDState) {
	if !st.Visible {
		return
	}
	ui.Column(c).
		Background(theme.BgSurface).
		Border(1, theme.BorderSubtle).
		Radius(8).
		Padding(10).
		Gap(6).
		Children(func() {
			if !st.HasData {
				ui.Text(c, "当前 Demo 无底层按键指令记录").FontSize(11).TextColor(theme.TextMuted)
				return
			}
			ui.Row(c).Gap(4).Children(func() {
				keyBox(c, " ", false, 28)
				keyBox(c, "W", st.Buttons&parser.KeyW != 0, 28)
				keyBox(c, " ", false, 28)
				keyBox(c, "FIRE", st.Buttons&parser.KeyFire != 0, 48)
			})
			ui.Row(c).Gap(4).Children(func() {
				keyBox(c, "A", st.Buttons&parser.KeyA != 0, 28)
				keyBox(c, "S", st.Buttons&parser.KeyS != 0, 28)
				keyBox(c, "D", st.Buttons&parser.KeyD != 0, 28)
				keyBox(c, "SCOPE", st.Buttons&parser.KeyScope != 0, 48)
			})
			ui.Row(c).Gap(4).Children(func() {
				keyBox(c, "CTRL", st.Buttons&parser.KeyCrouch != 0, 44)
				keyBox(c, "SHIFT", st.Buttons&parser.KeyWalk != 0, 44)
				keyBox(c, "JUMP", st.Buttons&parser.KeyJump != 0, 48)
			})
		})
}

func keyBox(c *ui.Context, label string, active bool, width float32) {
	if label == " " {
		ui.Column(c).Width(width).Height(24)
		return
	}
	bg := theme.BgInput
	border := theme.BorderSubtle
	fg := theme.TextMuted
	if active {
		bg = theme.AccentAmber
		border = theme.AccentAmber
		fg = theme.BgBase
	}
	ui.Column(c).
		Width(width).
		Height(24).
		Center().
		Background(bg).
		Border(1, border).
		Radius(4).
		Children(func() {
			ui.Text(c, label).Font("Rajdhani").FontSize(11).Bold().TextColor(fg)
		})
}
