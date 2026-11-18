// Package radar implements Strikeye's pure-Go 6-axis CS Data Radar metric derivation,
// 2560x1440 neon card rendering, and 4.0-second intro animation generation.
package radar

import (
	"fmt"
	"math"

	"strikeye/internal/storage"
)

// NormalizeCeiling caps normalized radar axis ratios so high-performing players
// can overflow outside the 1.0 full-scale ring without breaking canvas bounds.
const NormalizeCeiling = 1.6

// DimensionKey identifies one of the six clockwise radar axes.
type DimensionKey string

const (
	DimKPR       DimensionKey = "kpr"
	DimSurviving DimensionKey = "surviving"
	DimADR       DimensionKey = "adr"
	DimKAST      DimensionKey = "kast"
	DimMultiKill DimensionKey = "multi_kill"
	DimRating    DimensionKey = "rating"
)

// DimensionSpec defines one radar axis's display metadata and full-scale benchmark.
type DimensionSpec struct {
	Key       DimensionKey `json:"key"`
	Label     string       `json:"label"`
	FullScale float64      `json:"full_scale"`
	FullText  string       `json:"full_text"`
	Unit      string       `json:"unit"`
}

// Dimensions lists the six clockwise axes starting from 12 o'clock.
var Dimensions = [6]DimensionSpec{
	{Key: DimKPR, Label: "KPR", FullScale: 0.85, FullText: "满分 0.85", Unit: "raw"},
	{Key: DimSurviving, Label: "Surviving", FullScale: 0.44, FullText: "满分 44%", Unit: "pct"},
	{Key: DimADR, Label: "ADR", FullScale: 85.0, FullText: "满分 85", Unit: "int"},
	{Key: DimKAST, Label: "KAST", FullScale: 0.78, FullText: "满分 78%", Unit: "pct"},
	{Key: DimMultiKill, Label: "Multi-kill", FullScale: 8.0, FullText: "满分 8", Unit: "int"},
	{Key: DimRating, Label: "Rating", FullScale: 1.30, FullText: "满分 1.30", Unit: "raw"},
}

// RawValues stores un-normalized 6-axis player metrics.
// Surviving and KAST are stored as fractions in [0, 1] (e.g. 0.44 = 44%).
type RawValues struct {
	KPR       float64 `json:"kpr"`
	Surviving float64 `json:"surviving"`
	ADR       float64 `json:"adr"`
	KAST      float64 `json:"kast"`
	MultiKill float64 `json:"multi_kill"`
	Rating    float64 `json:"rating"`
}

// NormalizedValues stores normalized 6-axis ratios clamped to [0, NormalizeCeiling].
type NormalizedValues struct {
	KPR       float64 `json:"kpr"`
	Surviving float64 `json:"surviving"`
	ADR       float64 `json:"adr"`
	KAST      float64 `json:"kast"`
	MultiKill float64 `json:"multi_kill"`
	Rating    float64 `json:"rating"`
}

// Slice returns the six normalized values in clockwise order matching Dimensions.
func (n NormalizedValues) Slice() [6]float64 {
	return [6]float64{n.KPR, n.Surviving, n.ADR, n.KAST, n.MultiKill, n.Rating}
}

// Slice returns the six raw values in clockwise order matching Dimensions.
func (r RawValues) Slice() [6]float64 {
	return [6]float64{r.KPR, r.Surviving, r.ADR, r.KAST, r.MultiKill, r.Rating}
}

// DeriveFromMatchPlayer converts a storage.MatchPlayer into RawValues.
func DeriveFromMatchPlayer(p storage.MatchPlayer) RawValues {
	multiKill := p.MultiKillRounds
	if multiKill == 0 {
		multiKill = p.TwoKillRounds + p.ThreeKillRounds + p.FourKillRounds + p.FiveKillRounds
	}
	return RawValues{
		KPR:       sanitizeNonNeg(p.KPR),
		Surviving: sanitizeNonNeg(p.SurvivalRate) / 100.0,
		ADR:       sanitizeNonNeg(p.ADR),
		KAST:      sanitizeNonNeg(p.KASTPercent) / 100.0,
		MultiKill: sanitizeNonNeg(float64(multiKill)),
		Rating:    sanitizeNonNeg(p.StrikeyeRating),
	}
}

// Normalize divides each raw dimension by its full-scale benchmark and clamps to [0, NormalizeCeiling].
func Normalize(r RawValues) NormalizedValues {
	return NormalizedValues{
		KPR:       clampRatio(r.KPR, Dimensions[0].FullScale),
		Surviving: clampRatio(r.Surviving, Dimensions[1].FullScale),
		ADR:       clampRatio(r.ADR, Dimensions[2].FullScale),
		KAST:      clampRatio(r.KAST, Dimensions[3].FullScale),
		MultiKill: clampRatio(r.MultiKill, Dimensions[4].FullScale),
		Rating:    clampRatio(r.Rating, Dimensions[5].FullScale),
	}
}

// ComputeMatchAverage computes the mean normalized radar profile across all players in a match.
func ComputeMatchAverage(players []RawValues) NormalizedValues {
	if len(players) == 0 {
		return NormalizedValues{}
	}
	var sum [6]float64
	for _, p := range players {
		norm := Normalize(p).Slice()
		for i := range sum {
			sum[i] += norm[i]
		}
	}
	n := float64(len(players))
	return NormalizedValues{
		KPR:       sum[0] / n,
		Surviving: sum[1] / n,
		ADR:       sum[2] / n,
		KAST:      sum[3] / n,
		MultiKill: sum[4] / n,
		Rating:    sum[5] / n,
	}
}

// FormatValue formats a raw metric value for card display.
func FormatValue(spec DimensionSpec, raw float64) string {
	raw = sanitizeNonNeg(raw)
	switch spec.Unit {
	case "pct":
		return fmt.Sprintf("%.0f%%", raw*100.0)
	case "int":
		return fmt.Sprintf("%.0f", math.Round(raw))
	default:
		return fmt.Sprintf("%.2f", raw)
	}
}

func clampRatio(val, fullScale float64) float64 {
	if fullScale <= 0 || math.IsNaN(val) || math.IsInf(val, 0) || val <= 0 {
		return 0
	}
	r := val / fullScale
	if r > NormalizeCeiling {
		return NormalizeCeiling
	}
	return r
}

func sanitizeNonNeg(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0
	}
	return v
}
