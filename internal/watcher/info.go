package watcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SidecarMetadata is the advisory match metadata read from a `.info` sidecar.
// Zero values mean "not provided"; callers must only fill missing demo fields.
type SidecarMetadata struct {
	SourcePlatform string
	MatchDate      string
	TeamAName      string
	TeamBName      string
	TeamAScore     int
	TeamBScore     int
	HasScore       bool
	MatchID        string
}

// IsZero reports whether the sidecar carried no applicable metadata.
func (m SidecarMetadata) IsZero() bool {
	return m.SourcePlatform == "" && m.MatchDate == "" && m.TeamAName == "" &&
		m.TeamBName == "" && !m.HasScore && m.MatchID == ""
}

// SidecarPath resolves the `.info` file accompanying a demo, preferring
// "<demo>.info" and falling back to a sibling with the extension replaced.
func SidecarPath(demoPath string) string {
	candidates := []string{
		demoPath + ".info",
		strings.TrimSuffix(demoPath, filepath.Ext(demoPath)) + ".info",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// ReadSidecar parses the `.info` sidecar for demoPath. It returns ok=false when
// no sidecar exists. A malformed sidecar returns an error alongside ok=true so
// callers can warn without losing the demo.
func ReadSidecar(demoPath string) (SidecarMetadata, bool, error) {
	path := SidecarPath(demoPath)
	if path == "" {
		return SidecarMetadata{}, false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return SidecarMetadata{}, true, fmt.Errorf("read sidecar: %w", err)
	}
	meta, err := ParseSidecar(raw)
	return meta, true, err
}

// ParseSidecar tolerantly decodes community `.info` JSON. Keys are compared
// case-insensitively with '_' and '-' ignored, and scalar values may be strings
// or numbers.
func ParseSidecar(raw []byte) (SidecarMetadata, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return SidecarMetadata{}, fmt.Errorf("decode sidecar json: %w", err)
	}
	fields := make(map[string]any, len(doc))
	for k, v := range doc {
		fields[normalizeInfoKey(k)] = v
	}

	var meta SidecarMetadata
	if src := infoString(fields, "source", "platform", "sourceplatform", "provider"); src != "" {
		meta.SourcePlatform = canonicalInfoPlatform(src)
	}
	if date := infoString(fields, "date", "matchdate", "playedat", "starttime", "timestamp"); date != "" {
		meta.MatchDate = normalizeInfoDate(date)
	}
	meta.TeamAName = infoString(fields, "team1", "teama", "team1name", "teamaname", "clan1")
	meta.TeamBName = infoString(fields, "team2", "teamb", "team2name", "teambname", "clan2")
	if s1, ok := infoInt(fields, "score1", "scorea", "teama1score", "team1score"); ok {
		meta.TeamAScore = s1
		meta.HasScore = true
	}
	if s2, ok := infoInt(fields, "score2", "scoreb", "teambscore", "team2score"); ok {
		meta.TeamBScore = s2
		meta.HasScore = true
	}
	meta.MatchID = infoString(fields, "matchid", "match", "id", "gameid")
	return meta, nil
}

func normalizeInfoKey(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	k = strings.ReplaceAll(k, "_", "")
	k = strings.ReplaceAll(k, "-", "")
	k = strings.ReplaceAll(k, " ", "")
	return k
}

func canonicalInfoPlatform(raw string) string {
	normalized := NormalizePlatform(raw)
	for _, id := range PlatformIDs() {
		if normalized == id {
			return normalized
		}
	}
	// Vendor labels such as "Perfect World" or "ESL Play" fall back to the
	// path classifier, which understands those fragments.
	if detected := ClassifyPlatform(raw); detected != PlatformUnknown {
		return detected
	}
	return normalized
}

func infoString(fields map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := fields[k]
		if !ok {
			continue
		}
		switch typed := v.(type) {
		case string:
			if s := strings.TrimSpace(typed); s != "" {
				return s
			}
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(typed)
		}
	}
	return ""
}

func infoInt(fields map[string]any, keys ...string) (int, bool) {
	for _, k := range keys {
		v, ok := fields[k]
		if !ok {
			continue
		}
		switch typed := v.(type) {
		case float64:
			return int(typed), true
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

// normalizeInfoDate converts common timestamp shapes to RFC3339 UTC. Values it
// cannot parse are returned unchanged so the raw text is still visible in the UI.
func normalizeInfoDate(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
		if n > 1_000_000_000_000 { // milliseconds
			n /= 1000
		}
		return time.Unix(n, 0).UTC().Format(time.RFC3339)
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return s
}
