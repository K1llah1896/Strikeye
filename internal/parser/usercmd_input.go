package parser

import (
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"google.golang.org/protobuf/encoding/protowire"
)

const (
	InputCapNone          = "none"
	InputCapEntityButtons = "entity_buttons_only"
	InputCapFullUserCmd   = "full_usercmd"
)

// Normalized Strikeye button bitmask bits.
const (
	KeyW          uint16 = 1 << 0
	KeyA          uint16 = 1 << 1
	KeyS          uint16 = 1 << 2
	KeyD          uint16 = 1 << 3
	KeyJump       uint16 = 1 << 4
	KeyCrouch     uint16 = 1 << 5
	KeyWalk       uint16 = 1 << 6
	KeyReload     uint16 = 1 << 7
	KeyFire       uint16 = 1 << 8
	KeyScope      uint16 = 1 << 9
	KeyUse        uint16 = 1 << 10
	KeyInspect    uint16 = 1 << 11
	KeyScoreboard uint16 = 1 << 12
)

// InputEdge records a raw-tick button state transition for a player slot.
type InputEdge struct {
	Tick       int64
	PlayerSlot uint8
	Buttons    uint16
}

// InputCollector tracks UserCmd net-message presence and m_nButtons edge transitions.
type InputCollector struct {
	sawUserCmd       bool
	sawEntityButtons bool
	lastMask         map[uint8]uint16
	edgesByRound     map[int][]InputEdge
}

// NewInputCollector creates an InputCollector.
func NewInputCollector() *InputCollector {
	return &InputCollector{
		lastMask:     make(map[uint8]uint16),
		edgesByRound: make(map[int][]InputEdge),
	}
}

// ObserveUserCmd marks that CSVCMsg_UserCommands / CDemoUserCmd data is present in the demo.
func (ic *InputCollector) ObserveUserCmd() {
	ic.sawUserCmd = true
}

// ObserveButtons records a player's button mask at tick in roundNum, emitting an edge on change.
func (ic *InputCollector) ObserveButtons(roundNum int, tick int64, slot uint8, mask uint16) {
	if mask != 0 {
		ic.sawEntityButtons = true
	}
	prev, seen := ic.lastMask[slot]
	if seen && prev == mask {
		return
	}
	if !seen && mask == 0 {
		ic.lastMask[slot] = 0
		return
	}
	ic.lastMask[slot] = mask
	ic.edgesByRound[roundNum] = append(ic.edgesByRound[roundNum], InputEdge{
		Tick:       tick,
		PlayerSlot: slot,
		Buttons:    mask,
	})
}

// Capability returns full_usercmd, entity_buttons_only, or none.
func (ic *InputCollector) Capability() string {
	if ic.sawUserCmd {
		return InputCapFullUserCmd
	}
	if ic.sawEntityButtons {
		return InputCapEntityButtons
	}
	return InputCapNone
}

// RoundEdges returns raw-tick input edge transitions collected for roundNum.
func (ic *InputCollector) RoundEdges(roundNum int) []InputEdge {
	return ic.edgesByRound[roundNum]
}

// ButtonsFromPlayer reads entity m_nButtons via demoinfocs-golang/v5 and maps to Strikeye Key* bits.
func ButtonsFromPlayer(p *common.Player) uint16 {
	if p == nil {
		return 0
	}
	return ButtonsFromBitmask(p.ButtonsPressedState)
}

// ButtonsFromBitmask maps raw Source 2 InputBitMask_t bits to Strikeye Key* bits.
func ButtonsFromBitmask(raw uint64) uint16 {
	var out uint16
	if raw&uint64(common.ButtonForward) != 0 {
		out |= KeyW
	}
	if raw&uint64(common.ButtonMoveLeft) != 0 {
		out |= KeyA
	}
	if raw&uint64(common.ButtonBack) != 0 {
		out |= KeyS
	}
	if raw&uint64(common.ButtonMoveRight) != 0 {
		out |= KeyD
	}
	if raw&uint64(common.ButtonJump) != 0 {
		out |= KeyJump
	}
	if raw&uint64(common.ButtonDuck) != 0 {
		out |= KeyCrouch
	}
	if raw&uint64(common.ButtonSpeed) != 0 {
		out |= KeyWalk
	}
	if raw&uint64(common.ButtonReload) != 0 {
		out |= KeyReload
	}
	if raw&uint64(common.ButtonAttack) != 0 {
		out |= KeyFire
	}
	if raw&uint64(common.ButtonZoom) != 0 || raw&uint64(common.ButtonAttack2) != 0 {
		out |= KeyScope
	}
	if raw&uint64(common.ButtonUse) != 0 {
		out |= KeyUse
	}
	if raw&uint64(common.ButtonLookAtWeapon) != 0 {
		out |= KeyInspect
	}
	if raw&uint64(common.ButtonScore) != 0 {
		out |= KeyScoreboard
	}
	return out
}

// DecodeUserCmdButtonState extracts base.buttons_pb.buttonstate1 from raw CSGOUserCmdPB bytes.
func DecodeUserCmdButtonState(data []byte) (uint64, bool) {
	baseBytes, ok := findLengthDelimitedField(data, 1)
	if !ok {
		return 0, false
	}
	btnBytes, ok := findLengthDelimitedField(baseBytes, 1)
	if !ok {
		return 0, false
	}
	for len(btnBytes) > 0 {
		num, typ, n := protowire.ConsumeTag(btnBytes)
		if n < 0 {
			return 0, false
		}
		btnBytes = btnBytes[n:]
		if num == 1 && typ == protowire.VarintType {
			v, vn := protowire.ConsumeVarint(btnBytes)
			if vn < 0 {
				return 0, false
			}
			return v, true
		}
		sn := protowire.ConsumeFieldValue(num, typ, btnBytes)
		if sn < 0 {
			return 0, false
		}
		btnBytes = btnBytes[sn:]
	}
	return 0, false
}

func findLengthDelimitedField(b []byte, target protowire.Number) ([]byte, bool) {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, false
		}
		b = b[n:]
		if num == target && typ == protowire.BytesType {
			val, vn := protowire.ConsumeBytes(b)
			if vn < 0 {
				return nil, false
			}
			return val, true
		}
		sn := protowire.ConsumeFieldValue(num, typ, b)
		if sn < 0 {
			return nil, false
		}
		b = b[sn:]
	}
	return nil, false
}
