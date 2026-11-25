// Package sharecode converts CS2/CS:GO match share codes to their identifiers.
//
// The format is Valve's: 25 characters from a 57-symbol alphabet, grouped as
// CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx, encoding 18 bytes little-endian
// (8 match id, 8 reservation id, 2 TV port).
package sharecode

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
)

const alphabet = "ABCDEFGHJKLMNOPQRSTUVWXYZabcdefhijkmnopqrstuvwxyz23456789"

// Match is the decoded payload of a share code.
type Match struct {
	MatchID       uint64 `json:"match_id"`
	ReservationID uint64 `json:"reservation_id"`
	TVPort        uint32 `json:"tv_port"`
	ShareCode     string `json:"share_code"`
}

// Decode parses a share code with or without the CSGO prefix and dashes.
func Decode(code string) (Match, error) {
	raw := strings.TrimSpace(code)
	upper := strings.ToUpper(raw)
	if strings.HasPrefix(upper, "CSGO-") {
		raw = raw[5:]
	}
	raw = strings.ReplaceAll(raw, "-", "")
	if len(raw) != 25 {
		return Match{}, fmt.Errorf("share code must contain 25 symbols, got %d", len(raw))
	}
	total := new(big.Int)
	base := big.NewInt(57)
	for i := len(raw) - 1; i >= 0; i-- {
		r := rune(raw[i])
		idx := strings.IndexRune(alphabet, r)
		if idx < 0 {
			return Match{}, fmt.Errorf("share code contains invalid symbol %q", r)
		}
		total.Mul(total, base)
		total.Add(total, big.NewInt(int64(idx)))
	}
	bytes := total.Bytes()
	if len(bytes) > 18 {
		return Match{}, fmt.Errorf("share code payload is too large")
	}
	buf := make([]byte, 18)
	copy(buf[18-len(bytes):], bytes)
	return Match{
		MatchID:       binary.LittleEndian.Uint64(buf[0:8]),
		ReservationID: binary.LittleEndian.Uint64(buf[8:16]),
		TVPort:        uint32(binary.LittleEndian.Uint16(buf[16:18])),
		ShareCode:     Canonical(buf),
	}, nil
}

// Canonical formats 18 payload bytes as a share code.
func Canonical(payload []byte) string {
	if len(payload) != 18 {
		return ""
	}
	total := new(big.Int).SetBytes(payload)
	base := big.NewInt(57)
	var b strings.Builder
	b.Grow(25)
	for i := 0; i < 25; i++ {
		rem := new(big.Int)
		total.DivMod(total, base, rem)
		b.WriteByte(alphabet[rem.Int64()])
	}
	s := b.String()
	return fmt.Sprintf("CSGO-%s-%s-%s-%s-%s", s[0:5], s[5:10], s[10:15], s[15:20], s[20:25])
}

// Encode builds a share code from identifiers.
func Encode(matchID, reservationID uint64, tvPort uint32) string {
	buf := make([]byte, 18)
	binary.LittleEndian.PutUint64(buf[0:8], matchID)
	binary.LittleEndian.PutUint64(buf[8:16], reservationID)
	binary.LittleEndian.PutUint16(buf[16:18], uint16(tvPort))
	return Canonical(buf)
}
