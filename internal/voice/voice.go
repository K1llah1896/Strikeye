// Package voice captures Steam voice packets from a CS2 demo.
//
// CS2 stores voice as Steam's proprietary codec: each CSVCMsg_VoiceData carries
// an opaque payload plus per-speaker metadata (XUID, sequence bytes, sample
// rate). The payload cannot be decoded without Steam's codec, so Strikeye keeps
// the packets verbatim in a small container and reports who spoke, when, and
// how much. Decoding to WAV is delegated to an external tool that understands
// Steam voice (for example csgo-voice-extractor), keeping this project free of
// cgo and native codec libraries.
package voice

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
)

// Container constants.
var magic = [4]byte{'S', 'K', 'V', 'O'}

const (
	// FormatVersion is the on-disk layout version. Version 2 widened the frame
	// payload offset from uint16 to uint32: a uint16 wrapped every 65535 samples
	// (1.37s at 48kHz), which scrambled the timeline of any longer utterance.
	FormatVersion uint16 = 2
	// legacyFormatVersion is the only older layout we still read.
	legacyFormatVersion uint16 = 1
	// maxPayload guards against corrupt length prefixes.
	maxPayload = 4096
)

// ErrInvalidContainer is returned when a file is not a Strikeye voice capture.
var ErrInvalidContainer = errors.New("voice: not a strikeye voice capture")

// Frame is one captured voice packet.
type Frame struct {
	Tick       int64
	XUID       uint64
	Format     uint8
	SampleRate uint32
	Sequence   int32
	Section    uint32
	Offset     uint32
	Payload    []byte
}

// Speaker summarises one XUID's activity.
type Speaker struct {
	XUID   uint64 `json:"xuid"`
	Frames int    `json:"frames"`
	Bytes  int    `json:"bytes"`
}

// Summary is the report returned by Extract.
type Summary struct {
	DemoPath string    `json:"demo_path"`
	Frames   int       `json:"frames"`
	Bytes    int       `json:"bytes"`
	Speakers []Speaker `json:"speakers"`
	// HasVoice is false for demos recorded without voice, which is the norm for
	// public matchmaking demos.
	HasVoice bool `json:"has_voice"`
}

// Extract parses a demo and writes every voice packet to out.
//
// The returned summary is always usable: demos recorded without voice (most
// public matchmaking demos) yield zero frames instead of an error.
func Extract(ctx context.Context, demoPath string, out io.Writer) (Summary, error) {
	sum := Summary{DemoPath: demoPath, Speakers: []Speaker{}}
	f, err := os.Open(demoPath)
	if err != nil {
		return sum, fmt.Errorf("open demo: %w", err)
	}
	defer f.Close()

	p := demoinfocs.NewParser(f)
	defer p.Close()

	w := bufio.NewWriterSize(out, 1<<16)
	if _, err := w.Write(magic[:]); err != nil {
		return sum, err
	}
	if err := binary.Write(w, binary.LittleEndian, FormatVersion); err != nil {
		return sum, err
	}

	perXUID := map[uint64]*Speaker{}
	writeErr := error(nil)
	p.RegisterNetMessageHandler(func(m *msg.CSVCMsg_VoiceData) {
		if writeErr != nil {
			return
		}
		audio := m.GetAudio()
		if audio == nil || len(audio.GetVoiceData()) == 0 {
			return
		}
		fr := Frame{
			Tick:       int64(m.GetTick()),
			XUID:       m.GetXuid(),
			Format:     uint8(audio.GetFormat()),
			SampleRate: audio.GetSampleRate(),
			Sequence:   audio.GetSequenceBytes(),
			Section:    audio.GetSectionNumber(),
			Offset:     audio.GetUncompressedSampleOffset(),
			Payload:    audio.GetVoiceData(),
		}
		if err := WriteFrame(w, fr); err != nil {
			writeErr = err
			return
		}
		sum.Frames++
		sum.Bytes += len(fr.Payload)
		sp := perXUID[fr.XUID]
		if sp == nil {
			sp = &Speaker{XUID: fr.XUID}
			perXUID[fr.XUID] = sp
		}
		sp.Frames++
		sp.Bytes += len(fr.Payload)
	})

	if err := p.ParseToEnd(); err != nil {
		return sum, fmt.Errorf("parse demo: %w", err)
	}
	if writeErr != nil {
		return sum, writeErr
	}
	if err := w.Flush(); err != nil {
		return sum, err
	}
	for _, sp := range perXUID {
		sum.Speakers = append(sum.Speakers, *sp)
	}
	sort.Slice(sum.Speakers, func(i, j int) bool {
		if sum.Speakers[i].Frames != sum.Speakers[j].Frames {
			return sum.Speakers[i].Frames > sum.Speakers[j].Frames
		}
		return sum.Speakers[i].XUID < sum.Speakers[j].XUID
	})
	sum.HasVoice = sum.Frames > 0
	return sum, ctx.Err()
}

// Frame header sizes: version 2 widened the offset field to uint32.
const (
	frameHeaderSize       = 33
	legacyFrameHeaderSize = 31
)

// WriteFrame appends one frame to w.
func WriteFrame(w io.Writer, fr Frame) error {
	if len(fr.Payload) > maxPayload {
		return fmt.Errorf("voice: payload %d exceeds %d bytes", len(fr.Payload), maxPayload)
	}
	var hdr [frameHeaderSize]byte
	binary.LittleEndian.PutUint64(hdr[0:], uint64(fr.Tick))
	binary.LittleEndian.PutUint64(hdr[8:], fr.XUID)
	hdr[16] = fr.Format
	binary.LittleEndian.PutUint32(hdr[17:], fr.SampleRate)
	binary.LittleEndian.PutUint32(hdr[21:], uint32(fr.Sequence))
	binary.LittleEndian.PutUint32(hdr[25:], fr.Section)
	binary.LittleEndian.PutUint32(hdr[29:], fr.Offset)
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(len(fr.Payload)))
	if _, err := w.Write(size[:]); err != nil {
		return err
	}
	_, err := w.Write(fr.Payload)
	return err
}

// ReadFrames decodes a container written by Extract. It is used by tests and by
// downstream tools that want to re-emit packets into another format.
func ReadFrames(r io.Reader) ([]Frame, error) {
	br := bufio.NewReader(r)
	var head [6]byte
	if _, err := io.ReadFull(br, head[:]); err != nil {
		return nil, ErrInvalidContainer
	}
	if [4]byte(head[:4]) != magic {
		return nil, ErrInvalidContainer
	}
	v := binary.LittleEndian.Uint16(head[4:])
	if v != FormatVersion && v != legacyFormatVersion {
		return nil, fmt.Errorf("%w: unsupported version %d", ErrInvalidContainer, v)
	}
	headerSize := frameHeaderSize
	if v == legacyFormatVersion {
		headerSize = legacyFrameHeaderSize
	}
	var out []Frame
	for {
		hdr := make([]byte, headerSize)
		if _, err := io.ReadFull(br, hdr[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return out, nil
			}
			return out, err
		}
		var size [4]byte
		if _, err := io.ReadFull(br, size[:]); err != nil {
			return out, err
		}
		n := binary.LittleEndian.Uint32(size[:])
		if n == 0 || n > maxPayload {
			return out, fmt.Errorf("%w: bad payload length %d", ErrInvalidContainer, n)
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(br, payload); err != nil {
			return out, err
		}
		fr := Frame{
			Tick:       int64(binary.LittleEndian.Uint64(hdr[0:])),
			XUID:       binary.LittleEndian.Uint64(hdr[8:]),
			Format:     hdr[16],
			SampleRate: binary.LittleEndian.Uint32(hdr[17:]),
			Sequence:   int32(binary.LittleEndian.Uint32(hdr[21:])),
			Section:    binary.LittleEndian.Uint32(hdr[25:]),
			Payload:    payload,
		}
		if v == legacyFormatVersion {
			fr.Offset = uint32(binary.LittleEndian.Uint16(hdr[29:]))
		} else {
			fr.Offset = binary.LittleEndian.Uint32(hdr[29:33])
		}
		out = append(out, fr)
	}
}
