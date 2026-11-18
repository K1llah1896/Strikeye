package parser

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	ReplayMagicBytes          = [8]byte{'C', 'S', '2', 'R', 'P', 'L', '0', '1'}
	ReplayFormatVersion uint16 = 1

	ErrInvalidReplayMagic   = errors.New("invalid CS2RPL01 magic")
	ErrUnsupportedReplayVer = errors.New("unsupported CS2RPL01 version")
	ErrReplayRoundNotFound  = errors.New("replay round not found")
)

const replayHeaderSize = 28

// ReplayHeader is the fixed-size header at offset 0 of a CS2RPL01 file.
type ReplayHeader struct {
	Magic       [8]byte
	Version     uint16
	InputCap    uint8 // 0=none, 1=entity_buttons_only, 2=full_usercmd
	Reserved    uint8
	TickRate    float32
	RoundCount  uint32
	IndexOffset uint64
}

// RoundIndexEntry points to one round's serialized payload inside CS2RPL01.
type RoundIndexEntry struct {
	RoundNumber uint32
	StartTick   int64
	EndTick     int64
	Offset      uint64
	Length      uint64
}

// ReplayPlayerPose holds one player's 32Hz sampled state.
type ReplayPlayerPose struct {
	Slot         uint8
	Team         uint8 // 0=unknown, 1=T, 2=CT
	Alive        bool
	HasBomb      bool
	IsDefusing   bool
	Health       uint8
	Armor        uint8
	Money        uint16
	EquipValue   uint16
	FlashDur     float32
	X            float32
	Y            float32
	Z            float32
	Yaw          float32
	Pitch        float32
	ActiveWeapon string
}

// ReplayFrame holds all sampled players at one 32Hz tick.
type ReplayFrame struct {
	Tick    int64
	Players []ReplayPlayerPose
}

// ReplayRoundData holds all replay, grenade, and input edge chunks for one official round.
type ReplayRoundData struct {
	RoundNumber uint32
	StartTick   int64
	EndTick     int64
	Frames      []ReplayFrame
	Grenades    []GrenadeTrack
	Smokes      []SmokeEffect
	Infernos    []InfernoEffect
	InputEdges  []InputEdge
}

// InputCapCode converts a capability string into its binary enum code.
func InputCapCode(capStr string) uint8 {
	switch capStr {
	case InputCapFullUserCmd:
		return 2
	case InputCapEntityButtons:
		return 1
	default:
		return 0
	}
}

// InputCapString converts a binary enum code into its capability string.
func InputCapString(code uint8) string {
	switch code {
	case 2:
		return InputCapFullUserCmd
	case 1:
		return InputCapEntityButtons
	default:
		return InputCapNone
	}
}

// WriteReplayFileAtomic writes a CS2RPL01 file atomically via a .tmp sibling file.
func WriteReplayFileAtomic(path string, tickRate float32, inputCap string, rounds []ReplayRoundData) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create replay dir: %w", err)
	}
	tmpPath := path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open replay tmp: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = f.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	hdr := ReplayHeader{
		Magic:      ReplayMagicBytes,
		Version:    ReplayFormatVersion,
		InputCap:   InputCapCode(inputCap),
		TickRate:   tickRate,
		RoundCount: uint32(len(rounds)),
	}
	if err := writeHeader(f, hdr); err != nil {
		return err
	}

	entries := make([]RoundIndexEntry, 0, len(rounds))
	var curOffset uint64 = replayHeaderSize
	var buf bytes.Buffer

	for _, r := range rounds {
		buf.Reset()
		if err := encodeRoundPayload(&buf, r); err != nil {
			return fmt.Errorf("encode round %d: %w", r.RoundNumber, err)
		}
		payload := buf.Bytes()
		if _, err := f.Write(payload); err != nil {
			return fmt.Errorf("write round %d: %w", r.RoundNumber, err)
		}
		entries = append(entries, RoundIndexEntry{
			RoundNumber: r.RoundNumber,
			StartTick:   r.StartTick,
			EndTick:     r.EndTick,
			Offset:      curOffset,
			Length:      uint64(len(payload)),
		})
		curOffset += uint64(len(payload))
	}

	hdr.IndexOffset = curOffset
	bw := bufio.NewWriter(f)
	for _, e := range entries {
		if err := binary.Write(bw, binary.LittleEndian, e.RoundNumber); err != nil {
			return err
		}
		if err := binary.Write(bw, binary.LittleEndian, e.StartTick); err != nil {
			return err
		}
		if err := binary.Write(bw, binary.LittleEndian, e.EndTick); err != nil {
			return err
		}
		if err := binary.Write(bw, binary.LittleEndian, e.Offset); err != nil {
			return err
		}
		if err := binary.Write(bw, binary.LittleEndian, e.Length); err != nil {
			return err
		}
	}
	if err := bw.Flush(); err != nil {
		return err
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := writeHeader(f, hdr); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	cleanup = false

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename replay file: %w", err)
	}
	return nil
}

// ReadReplayIndex reads and validates only the CS2RPL01 header and round directory.
func ReadReplayIndex(path string) (ReplayHeader, []RoundIndexEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return ReplayHeader{}, nil, err
	}
	defer f.Close()
	return readHeaderAndIndex(f)
}

// ReadReplayRound seeks directly to roundNum's offset and decodes only that round.
func ReadReplayRound(path string, roundNum uint32) (ReplayHeader, ReplayRoundData, error) {
	f, err := os.Open(path)
	if err != nil {
		return ReplayHeader{}, ReplayRoundData{}, err
	}
	defer f.Close()

	hdr, entries, err := readHeaderAndIndex(f)
	if err != nil {
		return ReplayHeader{}, ReplayRoundData{}, err
	}
	for _, e := range entries {
		if e.RoundNumber == roundNum {
			if _, err := f.Seek(int64(e.Offset), io.SeekStart); err != nil {
				return ReplayHeader{}, ReplayRoundData{}, err
			}
			lr := io.LimitReader(f, int64(e.Length))
			rd, err := decodeRoundPayload(lr)
			return hdr, rd, err
		}
	}
	return hdr, ReplayRoundData{}, fmt.Errorf("%w: %d", ErrReplayRoundNotFound, roundNum)
}

func writeHeader(w io.Writer, hdr ReplayHeader) error {
	if _, err := w.Write(hdr.Magic[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, hdr.Version); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, hdr.InputCap); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, hdr.Reserved); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, hdr.TickRate); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, hdr.RoundCount); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, hdr.IndexOffset)
}

func readHeaderAndIndex(f *os.File) (ReplayHeader, []RoundIndexEntry, error) {
	var hdr ReplayHeader
	if _, err := io.ReadFull(f, hdr.Magic[:]); err != nil {
		return hdr, nil, err
	}
	if hdr.Magic != ReplayMagicBytes {
		return hdr, nil, ErrInvalidReplayMagic
	}
	if err := binary.Read(f, binary.LittleEndian, &hdr.Version); err != nil {
		return hdr, nil, err
	}
	if hdr.Version != ReplayFormatVersion {
		return hdr, nil, fmt.Errorf("%w: %d", ErrUnsupportedReplayVer, hdr.Version)
	}
	if err := binary.Read(f, binary.LittleEndian, &hdr.InputCap); err != nil {
		return hdr, nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &hdr.Reserved); err != nil {
		return hdr, nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &hdr.TickRate); err != nil {
		return hdr, nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &hdr.RoundCount); err != nil {
		return hdr, nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &hdr.IndexOffset); err != nil {
		return hdr, nil, err
	}

	if _, err := f.Seek(int64(hdr.IndexOffset), io.SeekStart); err != nil {
		return hdr, nil, err
	}
	entries := make([]RoundIndexEntry, hdr.RoundCount)
	for i := uint32(0); i < hdr.RoundCount; i++ {
		if err := binary.Read(f, binary.LittleEndian, &entries[i].RoundNumber); err != nil {
			return hdr, nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &entries[i].StartTick); err != nil {
			return hdr, nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &entries[i].EndTick); err != nil {
			return hdr, nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &entries[i].Offset); err != nil {
			return hdr, nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &entries[i].Length); err != nil {
			return hdr, nil, err
		}
	}
	return hdr, entries, nil
}

func encodeRoundPayload(w io.Writer, r ReplayRoundData) error {
	_ = binary.Write(w, binary.LittleEndian, r.RoundNumber)
	_ = binary.Write(w, binary.LittleEndian, r.StartTick)
	_ = binary.Write(w, binary.LittleEndian, r.EndTick)

	// Chunk A: 32Hz Frames
	_ = binary.Write(w, binary.LittleEndian, uint32(len(r.Frames)))
	for _, fr := range r.Frames {
		_ = binary.Write(w, binary.LittleEndian, fr.Tick)
		_ = binary.Write(w, binary.LittleEndian, uint16(len(fr.Players)))
		for _, p := range fr.Players {
			var flags uint8
			if p.Alive {
				flags |= 1 << 0
			}
			if p.HasBomb {
				flags |= 1 << 1
			}
			if p.IsDefusing {
				flags |= 1 << 2
			}
			_ = binary.Write(w, binary.LittleEndian, p.Slot)
			_ = binary.Write(w, binary.LittleEndian, p.Team)
			_ = binary.Write(w, binary.LittleEndian, flags)
			_ = binary.Write(w, binary.LittleEndian, p.Health)
			_ = binary.Write(w, binary.LittleEndian, p.Armor)
			_ = binary.Write(w, binary.LittleEndian, p.Money)
			_ = binary.Write(w, binary.LittleEndian, p.EquipValue)
			_ = binary.Write(w, binary.LittleEndian, p.FlashDur)
			_ = binary.Write(w, binary.LittleEndian, p.X)
			_ = binary.Write(w, binary.LittleEndian, p.Y)
			_ = binary.Write(w, binary.LittleEndian, p.Z)
			_ = binary.Write(w, binary.LittleEndian, p.Yaw)
			_ = binary.Write(w, binary.LittleEndian, p.Pitch)
			writeShortString(w, p.ActiveWeapon)
		}
	}

	// Chunk B: Grenades, Smokes, Infernos
	_ = binary.Write(w, binary.LittleEndian, uint32(len(r.Grenades)))
	for _, g := range r.Grenades {
		_ = binary.Write(w, binary.LittleEndian, g.EntityID)
		writeShortString(w, g.ThrowerSteamID)
		writeShortString(w, g.GrenadeType)
		_ = binary.Write(w, binary.LittleEndian, g.ThrowTick)
		_ = binary.Write(w, binary.LittleEndian, g.EndTick)
		_ = binary.Write(w, binary.LittleEndian, uint32(len(g.Points)))
		for _, pt := range g.Points {
			_ = binary.Write(w, binary.LittleEndian, pt.Tick)
			_ = binary.Write(w, binary.LittleEndian, pt.Pos.X)
			_ = binary.Write(w, binary.LittleEndian, pt.Pos.Y)
			_ = binary.Write(w, binary.LittleEndian, pt.Pos.Z)
		}
	}

	_ = binary.Write(w, binary.LittleEndian, uint32(len(r.Smokes)))
	for _, s := range r.Smokes {
		_ = binary.Write(w, binary.LittleEndian, s.EntityID)
		writeShortString(w, s.ThrowerSteamID)
		_ = binary.Write(w, binary.LittleEndian, s.StartTick)
		_ = binary.Write(w, binary.LittleEndian, s.EndTick)
		_ = binary.Write(w, binary.LittleEndian, s.Pos.X)
		_ = binary.Write(w, binary.LittleEndian, s.Pos.Y)
		_ = binary.Write(w, binary.LittleEndian, s.Pos.Z)
	}

	_ = binary.Write(w, binary.LittleEndian, uint32(len(r.Infernos)))
	for _, inf := range r.Infernos {
		_ = binary.Write(w, binary.LittleEndian, inf.UniqueID)
		writeShortString(w, inf.ThrowerSteamID)
		_ = binary.Write(w, binary.LittleEndian, inf.StartTick)
		_ = binary.Write(w, binary.LittleEndian, inf.EndTick)
		_ = binary.Write(w, binary.LittleEndian, uint32(len(inf.Hull)))
		for _, pt := range inf.Hull {
			_ = binary.Write(w, binary.LittleEndian, pt.X)
			_ = binary.Write(w, binary.LittleEndian, pt.Y)
		}
	}

	// Chunk C: Raw-Tick Input Edge Transitions
	_ = binary.Write(w, binary.LittleEndian, uint32(len(r.InputEdges)))
	for _, e := range r.InputEdges {
		_ = binary.Write(w, binary.LittleEndian, e.Tick)
		_ = binary.Write(w, binary.LittleEndian, e.PlayerSlot)
		_ = binary.Write(w, binary.LittleEndian, e.Buttons)
	}
	return nil
}

func decodeRoundPayload(r io.Reader) (ReplayRoundData, error) {
	var out ReplayRoundData
	if err := binary.Read(r, binary.LittleEndian, &out.RoundNumber); err != nil {
		return out, err
	}
	if err := binary.Read(r, binary.LittleEndian, &out.StartTick); err != nil {
		return out, err
	}
	if err := binary.Read(r, binary.LittleEndian, &out.EndTick); err != nil {
		return out, err
	}

	var frameCount uint32
	if err := binary.Read(r, binary.LittleEndian, &frameCount); err != nil {
		return out, err
	}
	out.Frames = make([]ReplayFrame, frameCount)
	for i := uint32(0); i < frameCount; i++ {
		if err := binary.Read(r, binary.LittleEndian, &out.Frames[i].Tick); err != nil {
			return out, err
		}
		var pCount uint16
		if err := binary.Read(r, binary.LittleEndian, &pCount); err != nil {
			return out, err
		}
		out.Frames[i].Players = make([]ReplayPlayerPose, pCount)
		for j := uint16(0); j < pCount; j++ {
			var p ReplayPlayerPose
			var flags uint8
			_ = binary.Read(r, binary.LittleEndian, &p.Slot)
			_ = binary.Read(r, binary.LittleEndian, &p.Team)
			_ = binary.Read(r, binary.LittleEndian, &flags)
			p.Alive = flags&(1<<0) != 0
			p.HasBomb = flags&(1<<1) != 0
			p.IsDefusing = flags&(1<<2) != 0
			_ = binary.Read(r, binary.LittleEndian, &p.Health)
			_ = binary.Read(r, binary.LittleEndian, &p.Armor)
			_ = binary.Read(r, binary.LittleEndian, &p.Money)
			_ = binary.Read(r, binary.LittleEndian, &p.EquipValue)
			_ = binary.Read(r, binary.LittleEndian, &p.FlashDur)
			_ = binary.Read(r, binary.LittleEndian, &p.X)
			_ = binary.Read(r, binary.LittleEndian, &p.Y)
			_ = binary.Read(r, binary.LittleEndian, &p.Z)
			_ = binary.Read(r, binary.LittleEndian, &p.Yaw)
			_ = binary.Read(r, binary.LittleEndian, &p.Pitch)
			var err error
			p.ActiveWeapon, err = readShortString(r)
			if err != nil {
				return out, err
			}
			out.Frames[i].Players[j] = p
		}
	}

	var gCount uint32
	if err := binary.Read(r, binary.LittleEndian, &gCount); err != nil {
		return out, err
	}
	out.Grenades = make([]GrenadeTrack, gCount)
	for i := uint32(0); i < gCount; i++ {
		g := &out.Grenades[i]
		g.RoundNumber = int(out.RoundNumber)
		_ = binary.Read(r, binary.LittleEndian, &g.EntityID)
		var err error
		g.ThrowerSteamID, err = readShortString(r)
		if err != nil {
			return out, err
		}
		g.GrenadeType, err = readShortString(r)
		if err != nil {
			return out, err
		}
		_ = binary.Read(r, binary.LittleEndian, &g.ThrowTick)
		_ = binary.Read(r, binary.LittleEndian, &g.EndTick)
		var ptCount uint32
		if err := binary.Read(r, binary.LittleEndian, &ptCount); err != nil {
			return out, err
		}
		g.Points = make([]GrenadeSample, ptCount)
		for j := uint32(0); j < ptCount; j++ {
			_ = binary.Read(r, binary.LittleEndian, &g.Points[j].Tick)
			_ = binary.Read(r, binary.LittleEndian, &g.Points[j].Pos.X)
			_ = binary.Read(r, binary.LittleEndian, &g.Points[j].Pos.Y)
			_ = binary.Read(r, binary.LittleEndian, &g.Points[j].Pos.Z)
		}
	}

	var sCount uint32
	if err := binary.Read(r, binary.LittleEndian, &sCount); err != nil {
		return out, err
	}
	out.Smokes = make([]SmokeEffect, sCount)
	for i := uint32(0); i < sCount; i++ {
		s := &out.Smokes[i]
		s.RoundNumber = int(out.RoundNumber)
		_ = binary.Read(r, binary.LittleEndian, &s.EntityID)
		var err error
		s.ThrowerSteamID, err = readShortString(r)
		if err != nil {
			return out, err
		}
		_ = binary.Read(r, binary.LittleEndian, &s.StartTick)
		_ = binary.Read(r, binary.LittleEndian, &s.EndTick)
		_ = binary.Read(r, binary.LittleEndian, &s.Pos.X)
		_ = binary.Read(r, binary.LittleEndian, &s.Pos.Y)
		_ = binary.Read(r, binary.LittleEndian, &s.Pos.Z)
	}

	var iCount uint32
	if err := binary.Read(r, binary.LittleEndian, &iCount); err != nil {
		return out, err
	}
	out.Infernos = make([]InfernoEffect, iCount)
	for i := uint32(0); i < iCount; i++ {
		inf := &out.Infernos[i]
		inf.RoundNumber = int(out.RoundNumber)
		_ = binary.Read(r, binary.LittleEndian, &inf.UniqueID)
		var err error
		inf.ThrowerSteamID, err = readShortString(r)
		if err != nil {
			return out, err
		}
		_ = binary.Read(r, binary.LittleEndian, &inf.StartTick)
		_ = binary.Read(r, binary.LittleEndian, &inf.EndTick)
		var hCount uint32
		if err := binary.Read(r, binary.LittleEndian, &hCount); err != nil {
			return out, err
		}
		inf.Hull = make([]Point2D, hCount)
		for j := uint32(0); j < hCount; j++ {
			_ = binary.Read(r, binary.LittleEndian, &inf.Hull[j].X)
			_ = binary.Read(r, binary.LittleEndian, &inf.Hull[j].Y)
		}
	}

	var eCount uint32
	if err := binary.Read(r, binary.LittleEndian, &eCount); err != nil {
		return out, err
	}
	out.InputEdges = make([]InputEdge, eCount)
	for i := uint32(0); i < eCount; i++ {
		_ = binary.Read(r, binary.LittleEndian, &out.InputEdges[i].Tick)
		_ = binary.Read(r, binary.LittleEndian, &out.InputEdges[i].PlayerSlot)
		_ = binary.Read(r, binary.LittleEndian, &out.InputEdges[i].Buttons)
	}
	return out, nil
}

func writeShortString(w io.Writer, s string) {
	if len(s) > 255 {
		s = s[:255]
	}
	_ = binary.Write(w, binary.LittleEndian, uint8(len(s)))
	_, _ = w.Write([]byte(s))
}

func readShortString(r io.Reader) (string, error) {
	var l uint8
	if err := binary.Read(r, binary.LittleEndian, &l); err != nil {
		return "", err
	}
	if l == 0 {
		return "", nil
	}
	buf := make([]byte, l)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}
