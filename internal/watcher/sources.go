package watcher

import (
	"path/filepath"
	"strings"
)

// Canonical demo source platform identifiers. These values are persisted in
// demos.source_platform and must stay stable once released.
const (
	PlatformValve          = "valve"
	PlatformPerfectWorld   = "perfect_world"
	Platform5E             = "5e"
	PlatformFACEIT         = "faceit"
	PlatformESL            = "esl"
	PlatformESEA           = "esea"
	PlatformMatchZy        = "matchzy"
	PlatformPracc          = "pracc"
	PlatformRenown         = "renown"
	PlatformChallengermode = "challengermode"
	PlatformEsportal       = "esportal"
	PlatformFastCup        = "fastcup"
	PlatformEsplay         = "esplay"
	PlatformEsportligaen   = "esportligaen"
	PlatformPopflash       = "popflash"
	PlatformEbot           = "ebot"
	PlatformCustom         = "custom"
	PlatformUnknown        = "unknown"
)

// PlatformDescriptor pairs a canonical platform ID with its display label.
type PlatformDescriptor struct {
	ID    string
	Label string
}

// platformRegistry lists every supported source in display order.
var platformRegistry = []PlatformDescriptor{
	{PlatformValve, "Valve 官方"},
	{PlatformPerfectWorld, "完美世界"},
	{Platform5E, "5EPlay"},
	{PlatformFACEIT, "FACEIT"},
	{PlatformESL, "ESL"},
	{PlatformESEA, "ESEA"},
	{PlatformMatchZy, "MatchZy"},
	{PlatformPracc, "Pracc"},
	{PlatformRenown, "Renown"},
	{PlatformChallengermode, "Challengermode"},
	{PlatformEsportal, "Esportal"},
	{PlatformFastCup, "FASTCUP"},
	{PlatformEsplay, "Esplay"},
	{PlatformEsportligaen, "Esportligaen"},
	{PlatformPopflash, "Popflash"},
	{PlatformEbot, "eBot"},
	{PlatformCustom, "自定义"},
	{PlatformUnknown, "未知来源"},
}

// platformRule matches a normalized path against one canonical platform.
type platformRule struct {
	platform string
	needles  []string
}

// platformRules are ordered most-specific-first. The path is slash-normalized
// and lowercased before matching, so rules only need lowercase fragments.
// Vendor rules precede the generic Valve rule because "/csgo/replays" can appear
// beneath vendor-managed directories.
var platformRules = []platformRule{
	{PlatformPerfectWorld, []string{"perfectworld", "perfect_world", "/pw/", "pwp"}},
	{Platform5E, []string{"5eplay", "/5e/", "5e_", "5e-"}},
	{PlatformChallengermode, []string{"challengermode"}},
	{PlatformEsportligaen, []string{"esportligaen"}},
	{PlatformEsportal, []string{"esportal"}},
	{PlatformEsplay, []string{"esplay"}},
	{PlatformFastCup, []string{"fastcup"}},
	{PlatformFACEIT, []string{"faceit"}},
	{PlatformMatchZy, []string{"matchzy"}},
	{PlatformPracc, []string{"pracc"}},
	{PlatformRenown, []string{"renown"}},
	{PlatformPopflash, []string{"popflash"}},
	{PlatformESEA, []string{"esea"}},
	{PlatformESL, []string{"/esl/", "esl_", "esl-"}},
	{PlatformEbot, []string{"ebot"}},
	{PlatformValve, []string{"/csgo/replays", "counter-strike global offensive"}},
}

// ClassifyPlatform infers the demo source platform from a directory or file path.
func ClassifyPlatform(path string) string {
	// Append a separator so directory rules such as "/pw/" also match a path
	// that ends at that directory.
	lower := strings.ToLower(filepath.ToSlash(path)) + "/"
	for _, rule := range platformRules {
		for _, needle := range rule.needles {
			if strings.Contains(lower, needle) {
				return rule.platform
			}
		}
	}
	return PlatformUnknown
}

// NormalizePlatform maps historical or user-supplied labels onto canonical IDs.
// Unknown values are lowercased and returned unchanged so custom labels survive.
func NormalizePlatform(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", "pw", "perfectworld", "perfect-world", "perfect world":
		if v == "" {
			return PlatformUnknown
		}
		return PlatformPerfectWorld
	case "5eplay", "5e_play", "5e-play":
		return Platform5E
	default:
		return v
	}
}

// PlatformLabel returns the display label for a platform ID, falling back to the
// raw value (or the unknown label for empty input).
func PlatformLabel(id string) string {
	normalized := NormalizePlatform(id)
	for _, d := range platformRegistry {
		if d.ID == normalized {
			return d.Label
		}
	}
	if normalized == "" {
		return PlatformUnknown
	}
	return id
}

// AllPlatforms returns a copy of the platform registry in display order.
func AllPlatforms() []PlatformDescriptor {
	out := make([]PlatformDescriptor, len(platformRegistry))
	copy(out, platformRegistry)
	return out
}

// PlatformIDs returns canonical IDs in display order.
func PlatformIDs() []string {
	out := make([]string, 0, len(platformRegistry))
	for _, d := range platformRegistry {
		out = append(out, d.ID)
	}
	return out
}
