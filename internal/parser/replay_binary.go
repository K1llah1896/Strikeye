package parser

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

var (
	ReplayMagicBytes           = [8]byte{'C', 'S', '2', 'R', 'P', 'L', '0', '1'}
	ReplayFormatVersion uint16 = 3

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
	Slot       uint8
	Team       uint8 // 0=unknown, 1=T, 2=CT
	Alive      bool
	HasBomb    bool
	IsDefusing bool
	Health     uint8
	Armor      uint8
	Money      uint16
	EquipValue uint16
	FlashDur   float32
	X          float32
	Y          float32
	Z          float32
	// Yaw is the eye-angle yaw in degrees (0 = +X, counter-clockwise positive),
	// which is what demoinfocs exposes as Player.ViewDirectionX().
	Yaw   float32
	Pitch float32
	// Zoom is the scope level: 0 = hip fire, 1 = half zoom, 2 = full zoom.
	// Only stored from replay format 3 on; older files read back as 0.
	Zoom         uint8
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
	Shots       []ReplayShotEvent
	Deaths      []ReplayDeathEvent
	Bombs       []ReplayBombEvent
}

// ReplayBombEvent kinds persisted in the combat chunk.
const (
	ReplayBombPlant   uint8 = 1
	ReplayBombDefuse  uint8 = 2
	ReplayBombExplode uint8 = 3
)

// ReplayShotEvent is one firing trace sampled from WeaponFire.
type ReplayShotEvent struct {
	Tick       int64
	PlayerSlot uint8
	Weapon     string
	Start      Point3D
	End        Point3D
}

// ReplayDeathEvent records where a victim died and where the killer stood.
type ReplayDeathEvent struct {
	Tick       int64
	KillerSlot uint8
	VictimSlot uint8
	Headshot   bool
	Victim     Point3D
	Killer     Point3D
}

// ReplayBombEvent records a plant, defuse, or detonation position.
type ReplayBombEvent struct {
	Tick int64
	Kind uint8
	Pos  Point3D
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
		if err := encodeRoundPayload(&buf, r, hdr.Version); err != nil {
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
			if e.Length > 256<<20 {
				return ReplayHeader{}, ReplayRoundData{}, io.ErrUnexpectedEOF
			}
			if _, err := f.Seek(int64(e.Offset), io.SeekStart); err != nil {
				return ReplayHeader{}, ReplayRoundData{}, err
			}
			buf := make([]byte, e.Length)
			if _, err := io.ReadFull(f, buf); err != nil {
				return ReplayHeader{}, ReplayRoundData{}, err
			}
			rd, err := decodeRoundPayloadBytes(buf, hdr.Version)
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
	if hdr.Version > ReplayFormatVersion {
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

	if hdr.RoundCount > 4096 {
		return hdr, nil, io.ErrUnexpectedEOF
	}
	if _, err := f.Seek(int64(hdr.IndexOffset), io.SeekStart); err != nil {
		return hdr, nil, err
	}
	indexBytes := make([]byte, int(hdr.RoundCount)*36)
	if _, err := io.ReadFull(f, indexBytes); err != nil {
		return hdr, nil, err
	}
	entries := make([]RoundIndexEntry, hdr.RoundCount)
	sr := sliceReader{b: indexBytes}
	for i := uint32(0); i < hdr.RoundCount; i++ {
		entries[i].RoundNumber, _ = sr.u32()
		entries[i].StartTick, _ = sr.i64()
		entries[i].EndTick, _ = sr.i64()
		entries[i].Offset, _ = sr.u64()
		entries[i].Length, _ = sr.u64()
	}
	return hdr, entries, nil
}

func encodeRoundPayload(w io.Writer, r ReplayRoundData, version uint16) error {
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
			if version >= 3 {
				_ = binary.Write(w, binary.LittleEndian, p.Zoom)
			}
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

	// Chunk D (v2): Combat events.
	if ReplayFormatVersion < 2 {
		return nil
	}
	_ = binary.Write(w, binary.LittleEndian, uint32(len(r.Shots)))
	for _, sh := range r.Shots {
		_ = binary.Write(w, binary.LittleEndian, sh.Tick)
		_ = binary.Write(w, binary.LittleEndian, sh.PlayerSlot)
		_ = binary.Write(w, binary.LittleEndian, sh.Start.X)
		_ = binary.Write(w, binary.LittleEndian, sh.Start.Y)
		_ = binary.Write(w, binary.LittleEndian, sh.Start.Z)
		_ = binary.Write(w, binary.LittleEndian, sh.End.X)
		_ = binary.Write(w, binary.LittleEndian, sh.End.Y)
		_ = binary.Write(w, binary.LittleEndian, sh.End.Z)
		writeShortString(w, sh.Weapon)
	}

	_ = binary.Write(w, binary.LittleEndian, uint32(len(r.Deaths)))
	for _, d := range r.Deaths {
		var flags uint8
		if d.Headshot {
			flags = 1
		}
		_ = binary.Write(w, binary.LittleEndian, d.Tick)
		_ = binary.Write(w, binary.LittleEndian, d.KillerSlot)
		_ = binary.Write(w, binary.LittleEndian, d.VictimSlot)
		_ = binary.Write(w, binary.LittleEndian, flags)
		_ = binary.Write(w, binary.LittleEndian, d.Victim.X)
		_ = binary.Write(w, binary.LittleEndian, d.Victim.Y)
		_ = binary.Write(w, binary.LittleEndian, d.Victim.Z)
		_ = binary.Write(w, binary.LittleEndian, d.Killer.X)
		_ = binary.Write(w, binary.LittleEndian, d.Killer.Y)
		_ = binary.Write(w, binary.LittleEndian, d.Killer.Z)
	}

	_ = binary.Write(w, binary.LittleEndian, uint32(len(r.Bombs)))
	for _, b := range r.Bombs {
		_ = binary.Write(w, binary.LittleEndian, b.Tick)
		_ = binary.Write(w, binary.LittleEndian, b.Kind)
		_ = binary.Write(w, binary.LittleEndian, b.Pos.X)
		_ = binary.Write(w, binary.LittleEndian, b.Pos.Y)
		_ = binary.Write(w, binary.LittleEndian, b.Pos.Z)
	}
	return nil
}

type sliceReader struct {
	b []byte
	o int
}

func (s *sliceReader) rem() int { return len(s.b) - s.o }

func (s *sliceReader) u8() (uint8, error) {
	if s.rem() < 1 {
		return 0, io.ErrUnexpectedEOF
	}
	v := s.b[s.o]
	s.o++
	return v, nil
}

func (s *sliceReader) u16() (uint16, error) {
	if s.rem() < 2 {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.LittleEndian.Uint16(s.b[s.o:])
	s.o += 2
	return v, nil
}

func (s *sliceReader) u32() (uint32, error) {
	if s.rem() < 4 {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.LittleEndian.Uint32(s.b[s.o:])
	s.o += 4
	return v, nil
}

func (s *sliceReader) i64() (int64, error) {
	if s.rem() < 8 {
		return 0, io.ErrUnexpectedEOF
	}
	v := int64(binary.LittleEndian.Uint64(s.b[s.o:]))
	s.o += 8
	return v, nil
}

func (s *sliceReader) u64() (uint64, error) {
	if s.rem() < 8 {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.LittleEndian.Uint64(s.b[s.o:])
	s.o += 8
	return v, nil
}

func (s *sliceReader) f32() (float32, error) {
	u, err := s.u32()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(u), nil
}

func (s *sliceReader) shortString(intern map[string]string) (string, error) {
	l, err := s.u8()
	if err != nil {
		return "", err
	}
	if l == 0 {
		return "", nil
	}
	if s.rem() < int(l) {
		return "", io.ErrUnexpectedEOF
	}
	raw := s.b[s.o : s.o+int(l)]
	s.o += int(l)
	if intern != nil {
		if cached, ok := intern[string(raw)]; ok {
			return cached, nil
		}
		str := string(raw)
		intern[str] = str
		return str, nil
	}
	return string(raw), nil
}

func decodeRoundPayloadBytes(buf []byte, version uint16) (ReplayRoundData, error) {
	var out ReplayRoundData
	sr := sliceReader{b: buf}
	intern := make(map[string]string, 48)

	var err error
	if out.RoundNumber, err = sr.u32(); err != nil {
		return out, err
	}
	if out.StartTick, err = sr.i64(); err != nil {
		return out, err
	}
	if out.EndTick, err = sr.i64(); err != nil {
		return out, err
	}

	frameCount, err := sr.u32()
	if err != nil {
		return out, err
	}
	// Each frame is at least 10 bytes (8 tick + 2 pCount).
	if uint64(frameCount)*10 > uint64(sr.rem()) {
		return out, io.ErrUnexpectedEOF
	}
	out.Frames = make([]ReplayFrame, frameCount)
	for i := uint32(0); i < frameCount; i++ {
		if out.Frames[i].Tick, err = sr.i64(); err != nil {
			return out, err
		}
		pCount, err := sr.u16()
		if err != nil {
			return out, err
		}
		// Each player pose is at least 34 bytes (fixed fields + 1-byte string len).
		if int(pCount)*34 > sr.rem() {
			return out, io.ErrUnexpectedEOF
		}
		players := make([]ReplayPlayerPose, pCount)
		for j := uint16(0); j < pCount; j++ {
			if sr.rem() < 34 {
				return out, io.ErrUnexpectedEOF
			}
			b := sr.b[sr.o:]
			flags := b[2]
			p := ReplayPlayerPose{
				Slot:       b[0],
				Team:       b[1],
				Alive:      flags&(1<<0) != 0,
				HasBomb:    flags&(1<<1) != 0,
				IsDefusing: flags&(1<<2) != 0,
				Health:     b[3],
				Armor:      b[4],
				Money:      binary.LittleEndian.Uint16(b[5:7]),
				EquipValue: binary.LittleEndian.Uint16(b[7:9]),
				FlashDur:   math.Float32frombits(binary.LittleEndian.Uint32(b[9:13])),
				X:          math.Float32frombits(binary.LittleEndian.Uint32(b[13:17])),
				Y:          math.Float32frombits(binary.LittleEndian.Uint32(b[17:21])),
				Z:          math.Float32frombits(binary.LittleEndian.Uint32(b[21:25])),
				Yaw:        math.Float32frombits(binary.LittleEndian.Uint32(b[25:29])),
				Pitch:      math.Float32frombits(binary.LittleEndian.Uint32(b[29:33])),
			}
			sr.o += 33
			if version >= 3 {
				zoom, err := sr.u8()
				if err != nil {
					return out, err
				}
				p.Zoom = zoom
			}
			p.ActiveWeapon, err = sr.shortString(intern)
			if err != nil {
				return out, err
			}
			players[j] = p
		}
		out.Frames[i].Players = players
	}

	gCount, err := sr.u32()
	if err != nil {
		return out, err
	}
	if uint64(gCount)*30 > uint64(sr.rem()) {
		return out, io.ErrUnexpectedEOF
	}
	out.Grenades = make([]GrenadeTrack, gCount)
	for i := uint32(0); i < gCount; i++ {
		g := &out.Grenades[i]
		g.RoundNumber = int(out.RoundNumber)
		if g.EntityID, err = sr.i64(); err != nil {
			return out, err
		}
		if g.ThrowerSteamID, err = sr.shortString(intern); err != nil {
			return out, err
		}
		if g.GrenadeType, err = sr.shortString(intern); err != nil {
			return out, err
		}
		if g.ThrowTick, err = sr.i64(); err != nil {
			return out, err
		}
		if g.EndTick, err = sr.i64(); err != nil {
			return out, err
		}
		ptCount, err := sr.u32()
		if err != nil {
			return out, err
		}
		if uint64(ptCount)*20 > uint64(sr.rem()) {
			return out, io.ErrUnexpectedEOF
		}
		g.Points = make([]GrenadeSample, ptCount)
		for j := uint32(0); j < ptCount; j++ {
			b := sr.b[sr.o : sr.o+20]
			sr.o += 20
			g.Points[j] = GrenadeSample{
				Tick: int64(binary.LittleEndian.Uint64(b[0:8])),
				Pos: Point3D{
					X: math.Float32frombits(binary.LittleEndian.Uint32(b[8:12])),
					Y: math.Float32frombits(binary.LittleEndian.Uint32(b[12:16])),
					Z: math.Float32frombits(binary.LittleEndian.Uint32(b[16:20])),
				},
			}
		}
	}

	sCount, err := sr.u32()
	if err != nil {
		return out, err
	}
	if uint64(sCount)*37 > uint64(sr.rem()) {
		return out, io.ErrUnexpectedEOF
	}
	out.Smokes = make([]SmokeEffect, sCount)
	for i := uint32(0); i < sCount; i++ {
		s := &out.Smokes[i]
		s.RoundNumber = int(out.RoundNumber)
		if s.EntityID, err = sr.i64(); err != nil {
			return out, err
		}
		if s.ThrowerSteamID, err = sr.shortString(intern); err != nil {
			return out, err
		}
		if s.StartTick, err = sr.i64(); err != nil {
			return out, err
		}
		if s.EndTick, err = sr.i64(); err != nil {
			return out, err
		}
		if s.Pos.X, err = sr.f32(); err != nil {
			return out, err
		}
		if s.Pos.Y, err = sr.f32(); err != nil {
			return out, err
		}
		if s.Pos.Z, err = sr.f32(); err != nil {
			return out, err
		}
	}

	iCount, err := sr.u32()
	if err != nil {
		return out, err
	}
	if uint64(iCount)*29 > uint64(sr.rem()) {
		return out, io.ErrUnexpectedEOF
	}
	out.Infernos = make([]InfernoEffect, iCount)
	for i := uint32(0); i < iCount; i++ {
		inf := &out.Infernos[i]
		inf.RoundNumber = int(out.RoundNumber)
		if inf.UniqueID, err = sr.i64(); err != nil {
			return out, err
		}
		if inf.ThrowerSteamID, err = sr.shortString(intern); err != nil {
			return out, err
		}
		if inf.StartTick, err = sr.i64(); err != nil {
			return out, err
		}
		if inf.EndTick, err = sr.i64(); err != nil {
			return out, err
		}
		hCount, err := sr.u32()
		if err != nil {
			return out, err
		}
		if uint64(hCount)*8 > uint64(sr.rem()) {
			return out, io.ErrUnexpectedEOF
		}
		inf.Hull = make([]Point2D, hCount)
		for j := uint32(0); j < hCount; j++ {
			b := sr.b[sr.o : sr.o+8]
			sr.o += 8
			inf.Hull[j] = Point2D{
				X: math.Float32frombits(binary.LittleEndian.Uint32(b[0:4])),
				Y: math.Float32frombits(binary.LittleEndian.Uint32(b[4:8])),
			}
		}
	}

	eCount, err := sr.u32()
	if err != nil {
		return out, err
	}
	if uint64(eCount)*11 > uint64(sr.rem()) {
		return out, io.ErrUnexpectedEOF
	}
	out.InputEdges = make([]InputEdge, eCount)
	for i := uint32(0); i < eCount; i++ {
		b := sr.b[sr.o : sr.o+11]
		sr.o += 11
		out.InputEdges[i] = InputEdge{
			Tick:       int64(binary.LittleEndian.Uint64(b[0:8])),
			PlayerSlot: b[8],
			Buttons:    binary.LittleEndian.Uint16(b[9:11]),
		}
	}

	// Chunk D (v2 combat events) is absent in version 1 payloads.
	if sr.rem() == 0 {
		return out, nil
	}
	shotCount, err := sr.u32()
	if err != nil {
		return out, err
	}
	if uint64(shotCount)*34 > uint64(sr.rem()) {
		return out, io.ErrUnexpectedEOF
	}
	out.Shots = make([]ReplayShotEvent, shotCount)
	for i := uint32(0); i < shotCount; i++ {
		sh := &out.Shots[i]
		if sh.Tick, err = sr.i64(); err != nil {
			return out, err
		}
		if sh.PlayerSlot, err = sr.u8(); err != nil {
			return out, err
		}
		if sh.Start.X, err = sr.f32(); err != nil {
			return out, err
		}
		if sh.Start.Y, err = sr.f32(); err != nil {
			return out, err
		}
		if sh.Start.Z, err = sr.f32(); err != nil {
			return out, err
		}
		if sh.End.X, err = sr.f32(); err != nil {
			return out, err
		}
		if sh.End.Y, err = sr.f32(); err != nil {
			return out, err
		}
		if sh.End.Z, err = sr.f32(); err != nil {
			return out, err
		}
		if sh.Weapon, err = sr.shortString(intern); err != nil {
			return out, err
		}
	}

	deathCount, err := sr.u32()
	if err != nil {
		return out, err
	}
	if uint64(deathCount)*35 > uint64(sr.rem()) {
		return out, io.ErrUnexpectedEOF
	}
	out.Deaths = make([]ReplayDeathEvent, deathCount)
	for i := uint32(0); i < deathCount; i++ {
		d := &out.Deaths[i]
		if d.Tick, err = sr.i64(); err != nil {
			return out, err
		}
		if d.KillerSlot, err = sr.u8(); err != nil {
			return out, err
		}
		if d.VictimSlot, err = sr.u8(); err != nil {
			return out, err
		}
		flags, err := sr.u8()
		if err != nil {
			return out, err
		}
		d.Headshot = flags&1 != 0
		if d.Victim.X, err = sr.f32(); err != nil {
			return out, err
		}
		if d.Victim.Y, err = sr.f32(); err != nil {
			return out, err
		}
		if d.Victim.Z, err = sr.f32(); err != nil {
			return out, err
		}
		if d.Killer.X, err = sr.f32(); err != nil {
			return out, err
		}
		if d.Killer.Y, err = sr.f32(); err != nil {
			return out, err
		}
		if d.Killer.Z, err = sr.f32(); err != nil {
			return out, err
		}
	}

	bombCount, err := sr.u32()
	if err != nil {
		return out, err
	}
	if uint64(bombCount)*21 > uint64(sr.rem()) {
		return out, io.ErrUnexpectedEOF
	}
	out.Bombs = make([]ReplayBombEvent, bombCount)
	for i := uint32(0); i < bombCount; i++ {
		b := &out.Bombs[i]
		if b.Tick, err = sr.i64(); err != nil {
			return out, err
		}
		if b.Kind, err = sr.u8(); err != nil {
			return out, err
		}
		if b.Pos.X, err = sr.f32(); err != nil {
			return out, err
		}
		if b.Pos.Y, err = sr.f32(); err != nil {
			return out, err
		}
		if b.Pos.Z, err = sr.f32(); err != nil {
			return out, err
		}
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
