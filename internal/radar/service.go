package radar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"strikeye/internal/config"
	"strikeye/internal/storage"
)

// Service orchestrates radar card generation, animation rendering, appearance updates,
// and SQLite persistence in radar_cards.
type Service struct {
	repo   *storage.AnalysisRepository
	layout config.DataLayout
}

// NewService creates a radar Service backed by repo and layout.
func NewService(repo *storage.AnalysisRepository, layout config.DataLayout) *Service {
	return &Service{repo: repo, layout: layout}
}

// GenerateMatchCards derives 6-axis stats for all players in demoID, renders their
// 2560x1440 static PNG cards, and upserts them into radar_cards.
func (s *Service) GenerateMatchCards(ctx context.Context, demoID int64) ([]storage.RadarCard, error) {
	players, err := s.repo.ListMatchPlayers(ctx, demoID)
	if err != nil {
		return nil, fmt.Errorf("list match players: %w", err)
	}
	if len(players) == 0 {
		return nil, nil
	}

	existingList, err := s.repo.ListRadarCards(ctx, demoID)
	if err != nil {
		return nil, fmt.Errorf("list existing radar cards: %w", err)
	}
	existingBySteam := make(map[string]storage.RadarCard, len(existingList))
	for _, c := range existingList {
		existingBySteam[c.SteamID64] = c
	}

	raws := make([]RawValues, len(players))
	for i, p := range players {
		raws[i] = DeriveFromMatchPlayer(p)
	}
	matchAvg := ComputeMatchAverage(raws)
	matchAvgBytes, err := json.Marshal(matchAvg)
	if err != nil {
		return nil, err
	}

	out := make([]storage.RadarCard, 0, len(players))
	for i, p := range players {
		cardID := fmt.Sprintf("demo_%d_%s", demoID, p.SteamID64)
		prev, hasPrev := existingBySteam[p.SteamID64]

		var portraitPath, teamLogoPath string
		var animPathPtr, portraitPtr, logoPtr *string
		if hasPrev {
			if prev.CustomPortraitPath != nil {
				portraitPath = *prev.CustomPortraitPath
				portraitPtr = prev.CustomPortraitPath
			}
			if prev.CustomTeamLogoPath != nil {
				teamLogoPath = *prev.CustomTeamLogoPath
				logoPtr = prev.CustomTeamLogoPath
			}
			animPathPtr = prev.AnimVideoPath
		}

		norm := Normalize(raws[i])
		dimPayload := struct {
			Raw        RawValues        `json:"raw"`
			Normalized NormalizedValues `json:"normalized"`
		}{
			Raw:        raws[i],
			Normalized: norm,
		}
		dimBytes, err := json.Marshal(dimPayload)
		if err != nil {
			return nil, err
		}

		teamLabel := p.TeamKey
		if teamLabel == "" {
			teamLabel = "UNKNOWN"
		}
		staticPath := filepath.Join(s.layout.CacheRadarCards, cardID+".png")
		in := CardInput{
			PlayerName:   p.DisplayName,
			TeamKey:      p.TeamKey,
			TeamLabel:    teamLabel,
			Kills:        p.Kills,
			Deaths:       p.Deaths,
			Assists:      p.Assists,
			Raw:          raws[i],
			Normalized:   norm,
			MatchAvg:     matchAvg,
			PortraitPath: portraitPath,
			TeamLogoPath: teamLogoPath,
		}
		if err := RenderCardPNG(in, staticPath); err != nil {
			return nil, fmt.Errorf("render card %s: %w", cardID, err)
		}

		card := storage.RadarCard{
			ID:                 cardID,
			DemoID:             demoID,
			SteamID64:          p.SteamID64,
			PlayerName:         p.DisplayName,
			TeamKey:            p.TeamKey,
			TeamLabel:          teamLabel,
			DimensionsJSON:     string(dimBytes),
			MatchAvgJSON:       string(matchAvgBytes),
			StaticImagePath:    staticPath,
			AnimVideoPath:      animPathPtr,
			CustomPortraitPath: portraitPtr,
			CustomTeamLogoPath: logoPtr,
			UpdatedAt:          time.Now().UTC().Format(time.RFC3339Nano),
		}
		if err := s.repo.UpsertRadarCard(ctx, card); err != nil {
			return nil, fmt.Errorf("upsert radar card %s: %w", cardID, err)
		}
		out = append(out, card)
	}
	return out, nil
}

// UpdateCardAppearance updates a card's custom portrait and/or team logo, re-renders its
// static PNG, and synchronizes any existing intro animation MP4.
func (s *Service) UpdateCardAppearance(ctx context.Context, demoID int64, steamID64 string, portraitPath, teamLogoPath *string, animOpts RenderAnimOptions) (storage.RadarCard, error) {
	cards, err := s.repo.ListRadarCards(ctx, demoID)
	if err != nil {
		return storage.RadarCard{}, err
	}
	var target *storage.RadarCard
	for i := range cards {
		if cards[i].SteamID64 == steamID64 {
			target = &cards[i]
			break
		}
	}
	if target == nil {
		return storage.RadarCard{}, fmt.Errorf("radar card not found for demo=%d steam=%s", demoID, steamID64)
	}

	var dimPayload struct {
		Raw        RawValues        `json:"raw"`
		Normalized NormalizedValues `json:"normalized"`
	}
	if err := json.Unmarshal([]byte(target.DimensionsJSON), &dimPayload); err != nil {
		return storage.RadarCard{}, fmt.Errorf("decode dimensions_json: %w", err)
	}
	var matchAvg NormalizedValues
	if err := json.Unmarshal([]byte(target.MatchAvgJSON), &matchAvg); err != nil {
		return storage.RadarCard{}, fmt.Errorf("decode match_avg_json: %w", err)
	}

	players, err := s.repo.ListMatchPlayers(ctx, demoID)
	if err != nil {
		return storage.RadarCard{}, err
	}
	var k, d, a int
	for _, p := range players {
		if p.SteamID64 == steamID64 {
			k, d, a = p.Kills, p.Deaths, p.Assists
			break
		}
	}

	target.CustomPortraitPath = portraitPath
	target.CustomTeamLogoPath = teamLogoPath
	in := CardInput{
		PlayerName: target.PlayerName,
		TeamKey:    target.TeamKey,
		TeamLabel:  target.TeamLabel,
		Kills:      k,
		Deaths:     d,
		Assists:    a,
		Raw:        dimPayload.Raw,
		Normalized: dimPayload.Normalized,
		MatchAvg:   matchAvg,
	}
	if portraitPath != nil {
		in.PortraitPath = *portraitPath
	}
	if teamLogoPath != nil {
		in.TeamLogoPath = *teamLogoPath
	}

	if err := RenderCardPNG(in, target.StaticImagePath); err != nil {
		return storage.RadarCard{}, err
	}

	if target.AnimVideoPath != nil && *target.AnimVideoPath != "" {
		synced, syncErr := SyncAppearanceAnimation(ctx, in, *target.AnimVideoPath, animOpts)
		if syncErr != nil {
			target.AnimVideoPath = nil
		} else if synced == "" {
			target.AnimVideoPath = nil
		} else {
			target.AnimVideoPath = &synced
		}
	}
	target.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.repo.UpsertRadarCard(ctx, *target); err != nil {
		return storage.RadarCard{}, err
	}
	return *target, nil
}

// GenerateCardAnimation renders the 4.0s MP4 intro animation for a persisted radar card
// and updates anim_video_path in SQLite.
func (s *Service) GenerateCardAnimation(ctx context.Context, demoID int64, steamID64 string, animOpts RenderAnimOptions) (storage.RadarCard, error) {
	cards, err := s.repo.ListRadarCards(ctx, demoID)
	if err != nil {
		return storage.RadarCard{}, err
	}
	var target *storage.RadarCard
	for i := range cards {
		if cards[i].SteamID64 == steamID64 {
			target = &cards[i]
			break
		}
	}
	if target == nil {
		return storage.RadarCard{}, fmt.Errorf("radar card not found for demo=%d steam=%s", demoID, steamID64)
	}

	var dimPayload struct {
		Raw        RawValues        `json:"raw"`
		Normalized NormalizedValues `json:"normalized"`
	}
	if err := json.Unmarshal([]byte(target.DimensionsJSON), &dimPayload); err != nil {
		return storage.RadarCard{}, err
	}
	var matchAvg NormalizedValues
	if err := json.Unmarshal([]byte(target.MatchAvgJSON), &matchAvg); err != nil {
		return storage.RadarCard{}, err
	}

	players, err := s.repo.ListMatchPlayers(ctx, demoID)
	if err != nil {
		return storage.RadarCard{}, err
	}
	var k, d, a int
	for _, p := range players {
		if p.SteamID64 == steamID64 {
			k, d, a = p.Kills, p.Deaths, p.Assists
			break
		}
	}

	in := CardInput{
		PlayerName: target.PlayerName,
		TeamKey:    target.TeamKey,
		TeamLabel:  target.TeamLabel,
		Kills:      k,
		Deaths:     d,
		Assists:    a,
		Raw:        dimPayload.Raw,
		Normalized: dimPayload.Normalized,
		MatchAvg:   matchAvg,
	}
	if target.CustomPortraitPath != nil {
		in.PortraitPath = *target.CustomPortraitPath
	}
	if target.CustomTeamLogoPath != nil {
		in.TeamLogoPath = *target.CustomTeamLogoPath
	}

	animPath := filepath.Join(s.layout.CacheRadarAnims, target.ID+".mp4")
	if err := RenderAnimationMP4(ctx, in, animPath, animOpts); err != nil {
		_ = os.Remove(animPath)
		return storage.RadarCard{}, err
	}
	target.AnimVideoPath = &animPath
	target.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.repo.UpsertRadarCard(ctx, *target); err != nil {
		return storage.RadarCard{}, err
	}
	return *target, nil
}
