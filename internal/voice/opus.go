package voice

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pion/opus"
)

// VoiceFormatOpus is the CMsgVoiceAudio format used by CS2 for raw Opus
// payloads; formats 0 and 1 wrap Steam's own voice container instead.
const VoiceFormatOpus = 2

// DecodedSpeaker reports what the built-in decoder recovered for one speaker.
type DecodedSpeaker struct {
	XUID      uint64  `json:"xuid"`
	Frames    int     `json:"frames"`
	Samples   int     `json:"samples"`
	Seconds   float64 `json:"seconds"`
	DecodeErr int     `json:"decode_errors"`
}

// DecodeSummary reports a built-in decode run.
type DecodeSummary struct {
	Speakers []DecodedSpeaker `json:"speakers"`
	Frames   int              `json:"frames"`
	Samples  int              `json:"samples"`
	Skipped  int              `json:"skipped_frames"`
	Enabled  bool             `json:"enabled"`
}

// ErrNoOpusFrames reports that no payload could be decoded with the built-in
// decoder, which happens when a demo used Steam's wrapped voice codec.
var ErrNoOpusFrames = fmt.Errorf("no opus-format voice frames found")

// speakerTrack is one decoded speaker on a sample timeline. Offsets from the
// demo are honoured so gaps become silence instead of a compressed timeline.
type speakerTrack struct {
	entry DecodedSpeaker
	pcm   []int16
}

// decodeSpeakers decodes every opus-format frame into per-speaker PCM.
func decodeSpeakers(frames []Frame) ([]uint64, map[uint64]*speakerTrack, DecodeSummary, error) {
	sum := DecodeSummary{Enabled: true}
	byXUID := map[uint64][]Frame{}
	for _, fr := range frames {
		if fr.Format != VoiceFormatOpus || len(fr.Payload) == 0 {
			sum.Skipped++
			continue
		}
		byXUID[fr.XUID] = append(byXUID[fr.XUID], fr)
	}
	if len(byXUID) == 0 {
		return nil, nil, sum, ErrNoOpusFrames
	}

	xuids := make([]uint64, 0, len(byXUID))
	for x := range byXUID {
		xuids = append(xuids, x)
	}
	sort.Slice(xuids, func(i, j int) bool { return xuids[i] < xuids[j] })

	tracks := make(map[uint64]*speakerTrack, len(xuids))
	for _, xuid := range xuids {
		list := byXUID[xuid]
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Section != list[j].Section {
				return list[i].Section < list[j].Section
			}
			if list[i].Offset != list[j].Offset {
				return list[i].Offset < list[j].Offset
			}
			return list[i].Tick < list[j].Tick
		})
		dec := opus.NewDecoder()
		if err := dec.Init(48000, 1); err != nil {
			return nil, nil, sum, fmt.Errorf("init opus decoder: %w", err)
		}
		tr := &speakerTrack{entry: DecodedSpeaker{XUID: xuid}}
		pcm := make([]int16, 5760) // 120ms at 48kHz is the Opus maximum
		for _, fr := range list {
			n, err := dec.DecodeToInt16(fr.Payload, pcm)
			if err != nil || n <= 0 {
				tr.entry.DecodeErr++
				continue
			}
			tr.entry.Frames++
			tr.entry.Samples += n
			// Place the packet at its uncompressed offset, but never rewind: a
			// demo may repeat or omit offsets and audio must stay monotonic.
			at := int(fr.Offset)
			if at < len(tr.pcm) || at > len(tr.pcm)+maxVoiceGap {
				at = len(tr.pcm)
			}
			if at > len(tr.pcm) {
				tr.pcm = append(tr.pcm, make([]int16, at-len(tr.pcm))...)
			}
			tr.pcm = append(tr.pcm, pcm[:n]...)
		}
		tr.entry.Seconds = float64(tr.entry.Samples) / 48000
		tracks[xuid] = tr
		sum.Frames += tr.entry.Frames
		sum.Samples += tr.entry.Samples
		sum.Speakers = append(sum.Speakers, tr.entry)
	}
	if sum.Frames == 0 {
		return nil, nil, sum, ErrNoOpusFrames
	}
	return xuids, tracks, sum, nil
}

// maxVoiceGap caps how much silence a single offset may insert (60s at 48kHz),
// so a corrupt offset cannot allocate gigabytes.
const maxVoiceGap = 48000 * 60

// DecodeOpus decodes Opus-format voice frames into a mono 48kHz PCM WAV on out
// and returns per-speaker sample counts. All speakers are mixed into one track,
// which is what a review clip needs.
func DecodeOpus(frames []Frame, out io.Writer) (DecodeSummary, error) {
	xuids, tracks, sum, err := decodeSpeakers(frames)
	if err != nil {
		return sum, err
	}
	var mix []int16
	for _, xuid := range xuids {
		tr := tracks[xuid]
		if len(tr.pcm) > len(mix) {
			mix = append(mix, make([]int16, len(tr.pcm)-len(mix))...)
		}
		for i, v := range tr.pcm {
			// Sum with saturation: comms overlapping is loud, not clipped garbage.
			sum := int32(mix[i]) + int32(v)
			if sum > 32767 {
				sum = 32767
			} else if sum < -32768 {
				sum = -32768
			}
			mix[i] = int16(sum)
		}
	}
	if err := writeWAVMono(out, mix, 48000); err != nil {
		return sum, err
	}
	return sum, nil
}

// DecodedTrack is one speaker track written by DecodeOpusDir.
type DecodedTrack struct {
	XUID    uint64  `json:"xuid"`
	File    string  `json:"file"`
	Seconds float64 `json:"seconds"`
	Frames  int     `json:"frames"`
}

// DecodeOpusDir writes one WAV per speaker plus a mixed track. Per-speaker files
// let an editor pick comms apart; the mixed file is the quick-listen default.
func DecodeOpusDir(frames []Frame, dir string) (DecodeSummary, []DecodedTrack, error) {
	var tracksOut []DecodedTrack
	xuids, tracks, sum, err := decodeSpeakers(frames)
	if err != nil {
		return sum, nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return sum, nil, err
	}
	var mix []int16
	for _, xuid := range xuids {
		tr := tracks[xuid]
		path := filepath.Join(dir, fmt.Sprintf("speaker_%d.wav", xuid))
		if err := writeWAVFile(path, tr.pcm); err != nil {
			return sum, tracksOut, err
		}
		tracksOut = append(tracksOut, DecodedTrack{XUID: xuid, File: path, Seconds: tr.entry.Seconds, Frames: tr.entry.Frames})
		if len(tr.pcm) > len(mix) {
			mix = append(mix, make([]int16, len(tr.pcm)-len(mix))...)
		}
		for i, v := range tr.pcm {
			s := int32(mix[i]) + int32(v)
			if s > 32767 {
				s = 32767
			} else if s < -32768 {
				s = -32768
			}
			mix[i] = int16(s)
		}
	}
	if err := writeWAVFile(filepath.Join(dir, "mixed.wav"), mix); err != nil {
		return sum, tracksOut, err
	}
	return sum, tracksOut, nil
}

// writeWAVFile writes samples to path atomically.
func writeWAVFile(path string, samples []int16) error {
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := writeWAVMono(f, samples, 48000); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// writeWAVMono writes 16-bit mono PCM with a self-describing RIFF header.
func writeWAVMono(w io.Writer, samples []int16, sampleRate int) error {
	dataLen := len(samples) * 2
	hdr := make([]byte, 44)
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(36+dataLen))
	copy(hdr[8:12], "WAVE")
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)
	binary.LittleEndian.PutUint16(hdr[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(hdr[22:24], 1) // mono
	binary.LittleEndian.PutUint32(hdr[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(hdr[28:32], uint32(sampleRate*2)) // byte rate
	binary.LittleEndian.PutUint16(hdr[32:34], 2)                    // block align
	binary.LittleEndian.PutUint16(hdr[34:36], 16)                   // bits
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], uint32(dataLen))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	pcm := make([]byte, 2)
	for _, s := range samples {
		binary.LittleEndian.PutUint16(pcm, uint16(s))
		if _, err := w.Write(pcm); err != nil {
			return err
		}
	}
	return nil
}

// ReadContainer decodes a .cs2voice container into its frames.
func ReadContainer(r io.Reader) ([]Frame, error) {
	return ReadFrames(r)
}

// NewContainerWriter writes the container header so callers (CLI, tests,
// importers) can append frames with WriteFrame.
func NewContainerWriter(w io.Writer) error {
	if _, err := w.Write(magic[:]); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, FormatVersion)
}

// ParseXOggPackets extracts Opus packets from an Ogg Opus stream. It exists so
// tests (and imported .opus recordings) can be replayed through the decoder
// without a second codec implementation.
func ParseXOggPackets(data []byte) ([][]byte, error) {
	if len(data) < 4 || string(data[:4]) != "OggS" {
		return nil, fmt.Errorf("not an ogg stream")
	}
	var packets [][]byte
	var cur []byte
	pages := 0
	for off := 0; off+27 <= len(data); {
		if string(data[off:off+4]) != "OggS" {
			return nil, fmt.Errorf("bad ogg page at %d", off)
		}
		pages++
		segCount := int(data[off+26])
		segTable := data[off+27 : off+27+segCount]
		body := off + 27 + segCount
		for _, lacing := range segTable {
			n := int(lacing)
			if body+n > len(data) {
				return nil, fmt.Errorf("truncated ogg page at %d", off)
			}
			cur = append(cur, data[body:body+n]...)
			body += n
			if n < 255 {
				packets = append(packets, cur)
				cur = nil
			}
		}
		off = body
	}
	if pages == 0 {
		return nil, fmt.Errorf("no ogg pages found")
	}
	// The first two packets of an Ogg Opus stream are OpusHead and OpusTags,
	// neither of which is audio.
	for len(packets) > 0 {
		head := string(packets[0])
		if strings.HasPrefix(head, "OpusHead") || strings.HasPrefix(head, "OpusTags") {
			packets = packets[1:]
			continue
		}
		break
	}
	return packets, nil
}
