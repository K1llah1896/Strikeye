package weapons

import (
	"embed"
	"strings"
)

//go:embed *.svg
var fs embed.FS

var weaponAliases = map[string]string{
	"ak-47":              "ak47",
	"ak47":               "ak47",
	"m4a4":               "m4a1",
	"m4a1":               "m4a1",
	"m4a1-s":             "m4a1_silencer",
	"m4a1_silencer":      "m4a1_silencer",
	"m4a1-silencer":      "m4a1_silencer",
	"awp":                "awp",
	"ssg 08":             "ssg08",
	"ssg08":              "ssg08",
	"scout":              "ssg08",
	"aug":                "aug",
	"sg 553":             "sg556",
	"sg553":              "sg556",
	"sg556":              "sg556",
	"famas":              "famas",
	"galil ar":           "galilar",
	"galilar":            "galilar",
	"scar-20":            "scar20",
	"scar20":             "scar20",
	"g3sg1":              "g3sg1",
	"desert eagle":       "deagle",
	"deagle":             "deagle",
	"usp-s":              "usp_silencer",
	"usp_silencer":       "usp_silencer",
	"glock-18":           "glock",
	"glock":              "glock",
	"p2000":              "hkp2000",
	"hkp2000":            "hkp2000",
	"p250":               "p250",
	"five-seven":         "fiveseven",
	"fiveseven":          "fiveseven",
	"tec-9":              "tec9",
	"tec9":               "tec9",
	"cz75-auto":          "cz75a",
	"cz75a":              "cz75a",
	"dual berettas":      "elite",
	"elite":              "elite",
	"r8 revolver":        "revolver",
	"revolver":           "revolver",
	"mac-10":             "mac10",
	"mac10":              "mac10",
	"mp9":                "mp9",
	"mp7":                "mp7",
	"mp5-sd":             "mp5sd",
	"mp5sd":              "mp5sd",
	"ump-45":             "ump45",
	"ump45":              "ump45",
	"p90":                "p90",
	"pp-bizon":           "bizon",
	"bizon":              "bizon",
	"nova":               "nova",
	"xm1014":             "xm1014",
	"mag-7":              "mag7",
	"mag7":               "mag7",
	"sawed-off":          "sawedoff",
	"sawedoff":           "sawedoff",
	"m249":               "m249",
	"negev":              "negev",
	"zeus x27":           "taser",
	"taser":              "taser",
	"he grenade":         "hegrenade",
	"hegrenade":          "hegrenade",
	"flashbang":          "flashbang",
	"smoke grenade":      "smokegrenade",
	"smokegrenade":       "smokegrenade",
	"molotov":            "molotov",
	"incendiary grenade": "incgrenade",
	"incgrenade":         "incgrenade",
	"inferno":            "inferno",
	"decoy grenade":      "decoy",
	"decoy":              "decoy",
	"c4":                 "c4",
	"planted_c4":         "planted_c4",
	"knife":              "knife",
	"knife_t":            "knife_t",
	"bayonet":            "bayonet",
	"karambit":           "knife_karambit",
	"butterfly knife":    "knife_butterfly",
	"m9 bayonet":         "knife_m9_bayonet",
	"world":              "world",
	"worldspawn":         "world",
}

// NormalizeWeaponKey maps a demoinfocs or internal weapon name to an embedded SVG key.
func NormalizeWeaponKey(raw string) string {
	k := strings.ToLower(strings.TrimSpace(raw))
	k = strings.TrimPrefix(k, "weapon_")
	if mapped, ok := weaponAliases[k]; ok {
		return mapped
	}
	if strings.Contains(k, "knife") || strings.Contains(k, "bayonet") || strings.Contains(k, "karambit") || strings.Contains(k, "daggers") {
		return "knife"
	}
	sanitized := strings.NewReplacer("-", "", " ", "", "_", "_").Replace(k)
	return sanitized
}

// LoadSVG returns the raw SVG bytes for a weapon or modifier icon key.
func LoadSVG(name string) ([]byte, bool) {
	key := NormalizeWeaponKey(name)
	if key == "" {
		return nil, false
	}
	data, err := fs.ReadFile(key + ".svg")
	if err == nil {
		return data, true
	}
	rawKey := strings.ToLower(strings.TrimSpace(name))
	if rawKey != "" && rawKey != key {
		if data2, err2 := fs.ReadFile(rawKey + ".svg"); err2 == nil {
			return data2, true
		}
	}
	return nil, false
}

// ListAllKeys returns all embedded SVG base names (without .svg).
func ListAllKeys() ([]string, error) {
	entries, err := fs.ReadDir(".")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".svg") {
			out = append(out, strings.TrimSuffix(e.Name(), ".svg"))
		}
	}
	return out, nil
}
