package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"strikeye/internal/storage"
)

// ResolveReplayCachePath returns an existing replay cache file for a demo. The
// stored path wins; if it is gone (DataRoot moved, WSL↔Windows path) the same
// file name under the current DataRoot is tried.
func ResolveReplayCachePath(dataRoot string, stored *string) string {
	if stored == nil || *stored == "" {
		return ""
	}
	if _, err := os.Stat(*stored); err == nil {
		return *stored
	}
	name := filepath.Base(strings.ReplaceAll(*stored, "\\", "/"))
	alt := filepath.Join(dataRoot, "cache", "replay_32hz", name)
	if _, err := os.Stat(alt); err == nil {
		return alt
	}
	return ""
}

// LoadReplayCache reads every round of a demo's replay cache. A demo without a
// usable cache yields an empty map and no error: callers treat that as "no
// spatial data" rather than a failure.
func LoadReplayCache(dataRoot string, demo *storage.Demo) (map[int]ReplayRoundData, error) {
	if demo == nil {
		return map[int]ReplayRoundData{}, nil
	}
	cachePath := ResolveReplayCachePath(dataRoot, demo.ReplayCachePath)
	if cachePath == "" {
		return map[int]ReplayRoundData{}, nil
	}
	_, index, err := ReadReplayIndex(cachePath)
	if err != nil {
		return nil, fmt.Errorf("index replay cache: %w", err)
	}
	out := make(map[int]ReplayRoundData, len(index))
	for _, ent := range index {
		_, rd, err := ReadReplayRound(cachePath, ent.RoundNumber)
		if err != nil {
			continue
		}
		out[int(ent.RoundNumber)] = rd
	}
	return out, nil
}

// ReplayRoundList flattens a round map into a slice ordered by round number, the
// shape tendency and map-control analyzers take.
func ReplayRoundList(rounds map[int]ReplayRoundData) []ReplayRoundData {
	keys := make([]int, 0, len(rounds))
	for k := range rounds {
		keys = append(keys, k)
	}
	sortInts(keys)
	out := make([]ReplayRoundData, 0, len(keys))
	for _, k := range keys {
		out = append(out, rounds[k])
	}
	return out
}

// sortInts is a tiny insertion sort: round counts are small and this keeps the
// package free of a sort import for one call site.
func sortInts(v []int) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}
