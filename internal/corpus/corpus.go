// Package corpus turns parsed demos into a comparable metrics snapshot so
// parser or metric changes can be regression-tested against golden values.
package corpus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"strikeye/internal/parser"
	"strikeye/internal/storage"
)

// Snapshot is the subset of a parse result that must stay stable across
// refactors: identity, structural counts and per-player metrics.
type Snapshot struct {
	Source       string        `json:"source"`
	SHA256       string        `json:"sha256"`
	SizeBytes    int64         `json:"size_bytes"`
	Map          string        `json:"map"`
	TeamA        string        `json:"team_a"`
	TeamB        string        `json:"team_b"`
	ScoreA       int           `json:"score_a"`
	ScoreB       int           `json:"score_b"`
	TickRate     float64       `json:"tick_rate"`
	ParseStatus  string        `json:"parse_status"`
	RoundCount   int           `json:"round_count"`
	EventCount   int           `json:"event_count"`
	Highlights   int           `json:"highlights"`
	InputCap     string        `json:"input_capability"`
	ReplayRounds int           `json:"replay_rounds"`
	ReplayShots  int           `json:"replay_shots"`
	ReplayDeaths int           `json:"replay_deaths"`
	Purchases    int           `json:"purchases"`
	Players      []PlayerEntry `json:"players"`
}

// PlayerEntry holds the per-player metrics used for golden comparison.
type PlayerEntry struct {
	Name         string  `json:"name"`
	SteamID64    string  `json:"steam_id64"`
	TeamKey      string  `json:"team_key"`
	Kills        int     `json:"kills"`
	Deaths       int     `json:"deaths"`
	Assists      int     `json:"assists"`
	ADR          float64 `json:"adr"`
	KAST         float64 `json:"kast"`
	Rating       float64 `json:"rating"`
	Headshots    int     `json:"headshots"`
	TTKAvgMS     float64 `json:"ttk_avg_ms"`
	TTKSamples   int     `json:"ttk_samples"`
	TTDAvgMS     float64 `json:"ttd_avg_ms"`
	TTDSamples   int     `json:"ttd_samples"`
	OpeningKills int     `json:"opening_kills"`
}

// Build parses one demo into a throwaway data root and returns its snapshot.
// The temp root keeps corpus runs from touching the user's library.
func Build(ctx context.Context, demoPath string, workDir string) (Snapshot, error) {
	return BuildWithGeometry(ctx, demoPath, workDir, "")
}

// BuildWithGeometry is Build plus an optional directory of `.tri` map geometry
// copied into the scratch cache, so visibility-derived metrics (TTD) are
// covered by the golden snapshot instead of silently reading zero.
func BuildWithGeometry(ctx context.Context, demoPath string, workDir string, geometryDir string) (Snapshot, error) {
	abs, err := filepath.Abs(demoPath)
	if err != nil {
		return Snapshot{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Snapshot{}, err
	}
	sum, err := fileSHA256(abs)
	if err != nil {
		return Snapshot{}, err
	}

	if workDir == "" {
		workDir, err = os.MkdirTemp("", "strikeye-corpus-*")
		if err != nil {
			return Snapshot{}, err
		}
		defer os.RemoveAll(workDir)
	}
	if geometryDir != "" {
		if err := copyGeometry(geometryDir, filepath.Join(workDir, "cache", "map_geometry")); err != nil {
			return Snapshot{}, err
		}
	}
	db, err := storage.OpenDefault(workDir)
	if err != nil {
		return Snapshot{}, err
	}
	defer db.Close()

	parse, err := parser.ParseAndPersist(ctx, db, parser.ParseOptions{
		DataRoot: workDir,
		FilePath: abs,
		Force:    true,
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse %s: %w", filepath.Base(abs), err)
	}

	snap := Snapshot{
		Source:      filepath.Base(abs),
		SHA256:      sum,
		SizeBytes:   info.Size(),
		TickRate:    parse.Demo.TickRate,
		ParseStatus: parse.Demo.ParseStatus,
		TeamA:       parse.Demo.TeamAName,
		TeamB:       parse.Demo.TeamBName,
		ScoreA:      parse.Demo.TeamAScore,
		ScoreB:      parse.Demo.TeamBScore,
		RoundCount:  parse.RoundCount,
		EventCount:  parse.EventCount,
		Highlights:  parse.HighlightCount,
		InputCap:    parse.InputDataCapability,
	}
	if parse.Demo.MapName != nil {
		snap.Map = *parse.Demo.MapName
	}

	players, err := db.Analysis().ListMatchPlayers(ctx, parse.Demo.ID)
	if err != nil {
		return Snapshot{}, err
	}
	sort.Slice(players, func(i, j int) bool {
		if players[i].PlayerName != players[j].PlayerName {
			return players[i].PlayerName < players[j].PlayerName
		}
		return players[i].SteamID64 < players[j].SteamID64
	})
	for _, p := range players {
		snap.Players = append(snap.Players, PlayerEntry{
			Name: p.PlayerName, SteamID64: p.SteamID64, TeamKey: p.TeamKey,
			Kills: p.Kills, Deaths: p.Deaths, Assists: p.Assists,
			ADR: p.ADR, KAST: p.KASTPercent, Rating: p.StrikeyeRating, Headshots: p.Headshots,
			TTKAvgMS: p.TTKAvgMS, TTKSamples: p.TTKSamples,
			TTDAvgMS: p.TTDAvgMS, TTDSamples: p.TTDSamples,
			OpeningKills: p.FirstKills,
		})
	}

	if parse.ReplayCachePath != "" {
		if _, idx, err := parser.ReadReplayIndex(parse.ReplayCachePath); err == nil {
			snap.ReplayRounds = len(idx)
			for _, entry := range idx {
				_, rd, err := parser.ReadReplayRound(parse.ReplayCachePath, entry.RoundNumber)
				if err != nil {
					continue
				}
				snap.ReplayShots += len(rd.Shots)
				snap.ReplayDeaths += len(rd.Deaths)
			}
		}
	}
	if purchases, err := db.Analysis().ListMatchPurchases(ctx, parse.Demo.ID); err == nil {
		snap.Purchases = len(purchases)
	}
	return snap, nil
}

// Diff reports human-readable differences between two snapshots. Floating point
// metrics compare with a small tolerance so unrelated rounding does not fail.
func Diff(want, got Snapshot) []string {
	var out []string
	cmpStr := func(name, w, g string) {
		if w != g {
			out = append(out, fmt.Sprintf("%s: want %q, got %q", name, w, g))
		}
	}
	cmpInt := func(name string, w, g int) {
		if w != g {
			out = append(out, fmt.Sprintf("%s: want %d, got %d", name, w, g))
		}
	}
	cmpFloat := func(name string, w, g float64) {
		if diff := w - g; diff > 0.11 || diff < -0.11 {
			out = append(out, fmt.Sprintf("%s: want %.2f, got %.2f", name, w, g))
		}
	}
	cmpStr("sha256", want.SHA256, got.SHA256)
	cmpStr("map", want.Map, got.Map)
	cmpStr("team_a", want.TeamA, got.TeamA)
	cmpStr("team_b", want.TeamB, got.TeamB)
	cmpInt("score_a", want.ScoreA, got.ScoreA)
	cmpInt("score_b", want.ScoreB, got.ScoreB)
	cmpFloat("tick_rate", want.TickRate, got.TickRate)
	cmpStr("parse_status", want.ParseStatus, got.ParseStatus)
	cmpInt("round_count", want.RoundCount, got.RoundCount)
	cmpInt("event_count", want.EventCount, got.EventCount)
	cmpInt("highlights", want.Highlights, got.Highlights)
	cmpStr("input_capability", want.InputCap, got.InputCap)
	cmpInt("replay_rounds", want.ReplayRounds, got.ReplayRounds)
	cmpInt("replay_shots", want.ReplayShots, got.ReplayShots)
	cmpInt("replay_deaths", want.ReplayDeaths, got.ReplayDeaths)
	cmpInt("purchases", want.Purchases, got.Purchases)

	byName := map[string]PlayerEntry{}
	for _, p := range got.Players {
		byName[p.Name] = p
	}
	if len(want.Players) != len(got.Players) {
		out = append(out, fmt.Sprintf("players: want %d, got %d", len(want.Players), len(got.Players)))
	}
	for _, wantP := range want.Players {
		gotP, ok := byName[wantP.Name]
		if !ok {
			out = append(out, fmt.Sprintf("player %s: missing", wantP.Name))
			continue
		}
		for _, f := range []struct {
			name string
			w, g int
		}{
			{"kills", wantP.Kills, gotP.Kills},
			{"deaths", wantP.Deaths, gotP.Deaths},
			{"assists", wantP.Assists, gotP.Assists},
			{"headshots", wantP.Headshots, gotP.Headshots},
			{"ttk_samples", wantP.TTKSamples, gotP.TTKSamples},
			{"ttd_samples", wantP.TTDSamples, gotP.TTDSamples},
			{"opening_kills", wantP.OpeningKills, gotP.OpeningKills},
		} {
			if f.w != f.g {
				out = append(out, fmt.Sprintf("player %s %s: want %d, got %d", wantP.Name, f.name, f.w, f.g))
			}
		}
		for _, f := range []struct {
			name string
			w, g float64
		}{
			{"adr", wantP.ADR, gotP.ADR},
			{"kast", wantP.KAST, gotP.KAST},
			{"rating", wantP.Rating, gotP.Rating},
			{"ttk_avg_ms", wantP.TTKAvgMS, gotP.TTKAvgMS},
			{"ttd_avg_ms", wantP.TTDAvgMS, gotP.TTDAvgMS},
		} {
			if diff := f.w - f.g; diff > 0.11 || diff < -0.11 {
				out = append(out, fmt.Sprintf("player %s %s: want %.2f, got %.2f", wantP.Name, f.name, f.w, f.g))
			}
		}
	}
	return out
}

// ReadSnapshot loads a golden JSON file.
func ReadSnapshot(path string) (Snapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var snap Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return snap, nil
}

// WriteSnapshot stores a snapshot as indented JSON.
func WriteSnapshot(path string, snap Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

// copyGeometry mirrors .tri files into the scratch geometry cache.
func copyGeometry(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read geometry dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".tri") {
			continue
		}
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(dstDir, e.Name())
		info, err := os.Stat(dst)
		if err == nil {
			if srcInfo, serr := e.Info(); serr == nil && info.Size() == srcInfo.Size() {
				continue
			}
		}
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return err
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
