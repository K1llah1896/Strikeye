package storage

import (
	"context"
	"fmt"
)

// MatchPlayer represents a row in match_players.
type MatchPlayer struct {
	ID                  int64
	DemoID              int64
	SteamID64           string
	PlayerName          string
	DisplayName         string
	TeamKey             string
	SpecSlot            *int
	Kills               int
	Deaths              int
	Assists             int
	KDRatio             float64
	KPR                 float64
	DPR                 float64
	APR                 float64
	TotalDamage         int
	ADR                 float64
	UtilityDamage       int
	UtilityADR          float64
	KASTPercent         float64
	Headshots           int
	HSPercent           float64
	FirstKills          int
	FirstDeaths         int
	OpeningDuelWinRate  float64
	TradeKills          int
	TradeDeaths         int
	SurvivalRate        float64
	OneKillRounds       int
	TwoKillRounds       int
	ThreeKillRounds     int
	FourKillRounds      int
	FiveKillRounds      int
	MultiKillRounds     int
	ClutchAttempts      int
	ClutchWins          int
	AWPKills            int
	AvgEquipmentValue   int
	ImpactScore         float64
	StrikeyeRating      float64
	FlashAssists        int
	EnemiesFlashed      int
	TotalFlashSec       float64
	RoundSwing          float64
	EcoAdjRating        float64
	EcoAdjADR           float64
	ClutchBreakdownJSON string
	TTKAvgMS            float64
	TTKSamples          int
	TTDAvgMS            float64
	TTDSamples          int
}

// MatchPlayerSide represents a row in match_player_sides for CT or T side-split stats.
type MatchPlayerSide struct {
	ID                 int64
	DemoID             int64
	SteamID64          string
	Side               string
	RoundsPlayed       int
	Kills              int
	Deaths             int
	Assists            int
	KDRatio            float64
	KPR                float64
	DPR                float64
	APR                float64
	TotalDamage        int
	ADR                float64
	UtilityDamage      int
	UtilityADR         float64
	KASTPercent        float64
	Headshots          int
	HSPercent          float64
	FirstKills         int
	FirstDeaths        int
	OpeningDuelWinRate float64
	TradeKills         int
	TradeDeaths        int
	SurvivalRate       float64
	MultiKillRounds    int
	ClutchAttempts     int
	ClutchWins         int
	FlashAssists       int
	EnemiesFlashed     int
	TotalFlashSec      float64
	ImpactScore        float64
	RoundSwing         float64
	StrikeyeRating     float64
	EcoAdjRating       float64
	EcoAdjADR          float64
}

// MatchRound represents a row in match_rounds.
type MatchRound struct {
	ID                int64
	DemoID            int64
	RoundNumber       int
	StartTick         int64
	FreezeEndTick     int64
	EndTick           int64
	DurationSec       int
	WinnerTeamKey     *string
	WinnerSide        *string
	WinReason         *string
	BombSite          *string
	TeamASide         string
	TeamBSide         string
	TeamAScoreBefore  int
	TeamBScoreBefore  int
	TeamAScoreAfter   int
	TeamBScoreAfter   int
	TeamAEquipValue   int
	TeamBEquipValue   int
	TeamAEconomyType  string
	TeamBEconomyType  string
	Headline          *string
	TagsJSON          string
	PlayerEconomyJSON string
}

// MatchEvent represents a row in match_events.
type MatchEvent struct {
	ID              int64
	DemoID          int64
	RoundNumber     int
	Tick            int64
	RoundTimeSec    float64
	EventType       string
	ActorSteamID64  *string
	ActorName       *string
	ActorTeamKey    *string
	TargetSteamID64 *string
	TargetName      *string
	TargetTeamKey   *string
	AssisterSteamID *string
	AssisterName    *string
	Weapon          *string
	IsHeadshot      bool
	IsWallbang      bool
	IsThroughSmoke  bool
	IsNoscope       bool
	IsAttackerBlind bool
	IsAssistedFlash bool
	Site            *string
	ExtraJSON       *string
}

// HighlightClip represents a row in highlight_clips.
type HighlightClip struct {
	ID              string
	DemoID          int64
	PlayerSteamID64 string
	PlayerName      string
	RoundNumber     int
	ClipType        string
	KillCount       int
	StartTick       int64
	EndTick         int64
	AnchorTicksJSON string
	PreBufferSec    float64
	PostBufferSec   float64
	TagsJSON        string
	WeaponLabel     *string
	Description     *string
	ImportanceScore float64
	IsFavorite      bool
	MediaVideoID    *int64
	CreatedAt       string
}

// RadarCard represents a row in radar_cards.
type RadarCard struct {
	ID                 string
	DemoID             int64
	SteamID64          string
	PlayerName         string
	TeamKey            string
	TeamLabel          string
	DimensionsJSON     string
	MatchAvgJSON       string
	StaticImagePath    string
	AnimVideoPath      *string
	CustomPortraitPath *string
	CustomTeamLogoPath *string
	UpdatedAt          string
}

// AnalysisRepository manages match_players, match_rounds, match_events, highlight_clips, and radar_cards.
type AnalysisRepository struct {
	db *DB
}

// Analysis returns an AnalysisRepository bound to db.
func (db *DB) Analysis() *AnalysisRepository {
	return &AnalysisRepository{db: db}
}

func (r *AnalysisRepository) ReplaceMatchPlayers(ctx context.Context, demoID int64, players []MatchPlayer) error {
	tx, err := r.db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM match_players WHERE demo_id = ?`, demoID); err != nil {
		_ = tx.Rollback()
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO match_players (
			demo_id, steam_id64, player_name, display_name, team_key, spec_slot,
			kills, deaths, assists, kd_ratio, kpr, dpr, apr,
			total_damage, adr, utility_damage, utility_adr, kast_percent,
			headshots, hs_percent, first_kills, first_deaths, opening_duel_win_rate,
			trade_kills, trade_deaths, survival_rate,
			one_kill_rounds, two_kill_rounds, three_kill_rounds, four_kill_rounds, five_kill_rounds, multi_kill_rounds,
			clutch_attempts, clutch_wins, awp_kills, avg_equipment_value, impact_score, strikeye_rating,
			flash_assists, enemies_flashed, total_flash_sec, round_swing, eco_adj_rating, eco_adj_adr, clutch_breakdown_json,
			ttk_avg_ms, ttk_samples, ttd_avg_ms, ttd_samples
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, p := range players {
		clutchJSON := p.ClutchBreakdownJSON
		if clutchJSON == "" {
			clutchJSON = "{}"
		}
		if _, err := stmt.ExecContext(ctx,
			demoID, p.SteamID64, p.PlayerName, p.DisplayName, p.TeamKey, p.SpecSlot,
			p.Kills, p.Deaths, p.Assists, p.KDRatio, p.KPR, p.DPR, p.APR,
			p.TotalDamage, p.ADR, p.UtilityDamage, p.UtilityADR, p.KASTPercent,
			p.Headshots, p.HSPercent, p.FirstKills, p.FirstDeaths, p.OpeningDuelWinRate,
			p.TradeKills, p.TradeDeaths, p.SurvivalRate,
			p.OneKillRounds, p.TwoKillRounds, p.ThreeKillRounds, p.FourKillRounds, p.FiveKillRounds, p.MultiKillRounds,
			p.ClutchAttempts, p.ClutchWins, p.AWPKills, p.AvgEquipmentValue, p.ImpactScore, p.StrikeyeRating,
			p.FlashAssists, p.EnemiesFlashed, p.TotalFlashSec, p.RoundSwing, p.EcoAdjRating, p.EcoAdjADR, clutchJSON,
			p.TTKAvgMS, p.TTKSamples, p.TTDAvgMS, p.TTDSamples,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert match_player %s: %w", p.SteamID64, err)
		}
	}
	return tx.Commit()
}

func (r *AnalysisRepository) ListMatchPlayers(ctx context.Context, demoID int64) ([]MatchPlayer, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT
			id, demo_id, steam_id64, player_name, display_name, team_key, spec_slot,
			kills, deaths, assists, kd_ratio, kpr, dpr, apr,
			total_damage, adr, utility_damage, utility_adr, kast_percent,
			headshots, hs_percent, first_kills, first_deaths, opening_duel_win_rate,
			trade_kills, trade_deaths, survival_rate,
			one_kill_rounds, two_kill_rounds, three_kill_rounds, four_kill_rounds, five_kill_rounds, multi_kill_rounds,
			clutch_attempts, clutch_wins, awp_kills, avg_equipment_value, impact_score, strikeye_rating,
			flash_assists, enemies_flashed, total_flash_sec, round_swing, eco_adj_rating, eco_adj_adr, clutch_breakdown_json,
			ttk_avg_ms, ttk_samples, ttd_avg_ms, ttd_samples
		FROM match_players WHERE demo_id = ? ORDER BY strikeye_rating DESC, kills DESC`, demoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MatchPlayer
	for rows.Next() {
		var p MatchPlayer
		if err := rows.Scan(
			&p.ID, &p.DemoID, &p.SteamID64, &p.PlayerName, &p.DisplayName, &p.TeamKey, &p.SpecSlot,
			&p.Kills, &p.Deaths, &p.Assists, &p.KDRatio, &p.KPR, &p.DPR, &p.APR,
			&p.TotalDamage, &p.ADR, &p.UtilityDamage, &p.UtilityADR, &p.KASTPercent,
			&p.Headshots, &p.HSPercent, &p.FirstKills, &p.FirstDeaths, &p.OpeningDuelWinRate,
			&p.TradeKills, &p.TradeDeaths, &p.SurvivalRate,
			&p.OneKillRounds, &p.TwoKillRounds, &p.ThreeKillRounds, &p.FourKillRounds, &p.FiveKillRounds, &p.MultiKillRounds,
			&p.ClutchAttempts, &p.ClutchWins, &p.AWPKills, &p.AvgEquipmentValue, &p.ImpactScore, &p.StrikeyeRating,
			&p.FlashAssists, &p.EnemiesFlashed, &p.TotalFlashSec, &p.RoundSwing, &p.EcoAdjRating, &p.EcoAdjADR, &p.ClutchBreakdownJSON,
			&p.TTKAvgMS, &p.TTKSamples, &p.TTDAvgMS, &p.TTDSamples,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ReplaceMatchPlayerSides replaces all CT/T side-split player rows for demoID.
func (r *AnalysisRepository) ReplaceMatchPlayerSides(ctx context.Context, demoID int64, sides []MatchPlayerSide) error {
	tx, err := r.db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM match_player_sides WHERE demo_id = ?`, demoID); err != nil {
		_ = tx.Rollback()
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO match_player_sides (
			demo_id, steam_id64, side, rounds_played,
			kills, deaths, assists, kd_ratio, kpr, dpr, apr,
			total_damage, adr, utility_damage, utility_adr, kast_percent,
			headshots, hs_percent, first_kills, first_deaths, opening_duel_win_rate,
			trade_kills, trade_deaths, survival_rate, multi_kill_rounds,
			clutch_attempts, clutch_wins, flash_assists, enemies_flashed, total_flash_sec,
			impact_score, round_swing, strikeye_rating, eco_adj_rating, eco_adj_adr
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, s := range sides {
		if _, err := stmt.ExecContext(ctx,
			demoID, s.SteamID64, s.Side, s.RoundsPlayed,
			s.Kills, s.Deaths, s.Assists, s.KDRatio, s.KPR, s.DPR, s.APR,
			s.TotalDamage, s.ADR, s.UtilityDamage, s.UtilityADR, s.KASTPercent,
			s.Headshots, s.HSPercent, s.FirstKills, s.FirstDeaths, s.OpeningDuelWinRate,
			s.TradeKills, s.TradeDeaths, s.SurvivalRate, s.MultiKillRounds,
			s.ClutchAttempts, s.ClutchWins, s.FlashAssists, s.EnemiesFlashed, s.TotalFlashSec,
			s.ImpactScore, s.RoundSwing, s.StrikeyeRating, s.EcoAdjRating, s.EcoAdjADR,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert match_player_side %s/%s: %w", s.SteamID64, s.Side, err)
		}
	}
	return tx.Commit()
}

// ListMatchPlayerSides returns stored CT/T side-split player stats for demoID.
// Returns an empty slice without error for legacy v1 demos.
func (r *AnalysisRepository) ListMatchPlayerSides(ctx context.Context, demoID int64) ([]MatchPlayerSide, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT
			id, demo_id, steam_id64, side, rounds_played,
			kills, deaths, assists, kd_ratio, kpr, dpr, apr,
			total_damage, adr, utility_damage, utility_adr, kast_percent,
			headshots, hs_percent, first_kills, first_deaths, opening_duel_win_rate,
			trade_kills, trade_deaths, survival_rate, multi_kill_rounds,
			clutch_attempts, clutch_wins, flash_assists, enemies_flashed, total_flash_sec,
			impact_score, round_swing, strikeye_rating, eco_adj_rating, eco_adj_adr
		FROM match_player_sides WHERE demo_id = ? ORDER BY side ASC, strikeye_rating DESC`, demoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MatchPlayerSide
	for rows.Next() {
		var s MatchPlayerSide
		if err := rows.Scan(
			&s.ID, &s.DemoID, &s.SteamID64, &s.Side, &s.RoundsPlayed,
			&s.Kills, &s.Deaths, &s.Assists, &s.KDRatio, &s.KPR, &s.DPR, &s.APR,
			&s.TotalDamage, &s.ADR, &s.UtilityDamage, &s.UtilityADR, &s.KASTPercent,
			&s.Headshots, &s.HSPercent, &s.FirstKills, &s.FirstDeaths, &s.OpeningDuelWinRate,
			&s.TradeKills, &s.TradeDeaths, &s.SurvivalRate, &s.MultiKillRounds,
			&s.ClutchAttempts, &s.ClutchWins, &s.FlashAssists, &s.EnemiesFlashed, &s.TotalFlashSec,
			&s.ImpactScore, &s.RoundSwing, &s.StrikeyeRating, &s.EcoAdjRating, &s.EcoAdjADR,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *AnalysisRepository) ReplaceMatchRounds(ctx context.Context, demoID int64, rounds []MatchRound) error {
	tx, err := r.db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM match_rounds WHERE demo_id = ?`, demoID); err != nil {
		_ = tx.Rollback()
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO match_rounds (
			demo_id, round_number, start_tick, freeze_end_tick, end_tick, duration_sec,
			winner_team_key, winner_side, win_reason, bomb_site, team_a_side, team_b_side,
			team_a_score_before, team_b_score_before, team_a_score_after, team_b_score_after,
			team_a_equip_value, team_b_equip_value, team_a_economy_type, team_b_economy_type,
			headline, tags_json, player_economy_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, rd := range rounds {
		tags := rd.TagsJSON
		if tags == "" {
			tags = "[]"
		}
		econ := rd.PlayerEconomyJSON
		if econ == "" {
			econ = "[]"
		}
		econA := rd.TeamAEconomyType
		if econA == "" {
			econA = "full"
		}
		econB := rd.TeamBEconomyType
		if econB == "" {
			econB = "full"
		}
		if _, err := stmt.ExecContext(ctx,
			demoID, rd.RoundNumber, rd.StartTick, rd.FreezeEndTick, rd.EndTick, rd.DurationSec,
			rd.WinnerTeamKey, rd.WinnerSide, rd.WinReason, rd.BombSite, rd.TeamASide, rd.TeamBSide,
			rd.TeamAScoreBefore, rd.TeamBScoreBefore, rd.TeamAScoreAfter, rd.TeamBScoreAfter,
			rd.TeamAEquipValue, rd.TeamBEquipValue, econA, econB,
			rd.Headline, tags, econ,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert match_round %d: %w", rd.RoundNumber, err)
		}
	}
	return tx.Commit()
}

func (r *AnalysisRepository) ListMatchRounds(ctx context.Context, demoID int64) ([]MatchRound, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT
			id, demo_id, round_number, start_tick, freeze_end_tick, end_tick, duration_sec,
			winner_team_key, winner_side, win_reason, bomb_site, team_a_side, team_b_side,
			team_a_score_before, team_b_score_before, team_a_score_after, team_b_score_after,
			team_a_equip_value, team_b_equip_value, team_a_economy_type, team_b_economy_type,
			headline, tags_json, player_economy_json
		FROM match_rounds WHERE demo_id = ? ORDER BY round_number ASC`, demoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MatchRound
	for rows.Next() {
		var rd MatchRound
		if err := rows.Scan(
			&rd.ID, &rd.DemoID, &rd.RoundNumber, &rd.StartTick, &rd.FreezeEndTick, &rd.EndTick, &rd.DurationSec,
			&rd.WinnerTeamKey, &rd.WinnerSide, &rd.WinReason, &rd.BombSite, &rd.TeamASide, &rd.TeamBSide,
			&rd.TeamAScoreBefore, &rd.TeamBScoreBefore, &rd.TeamAScoreAfter, &rd.TeamBScoreAfter,
			&rd.TeamAEquipValue, &rd.TeamBEquipValue, &rd.TeamAEconomyType, &rd.TeamBEconomyType,
			&rd.Headline, &rd.TagsJSON, &rd.PlayerEconomyJSON,
		); err != nil {
			return nil, err
		}
		out = append(out, rd)
	}
	return out, rows.Err()
}

func (r *AnalysisRepository) ReplaceMatchEvents(ctx context.Context, demoID int64, events []MatchEvent) error {
	tx, err := r.db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM match_events WHERE demo_id = ?`, demoID); err != nil {
		_ = tx.Rollback()
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO match_events (
			demo_id, round_number, tick, round_time_sec, event_type,
			actor_steam_id64, actor_name, actor_team_key,
			target_steam_id64, target_name, target_team_key,
			assister_steam_id64, assister_name, weapon,
			is_headshot, is_wallbang, is_through_smoke, is_noscope,
			is_attacker_blind, is_assisted_flash, site, extra_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, ev := range events {
		if _, err := stmt.ExecContext(ctx,
			demoID, ev.RoundNumber, ev.Tick, ev.RoundTimeSec, ev.EventType,
			ev.ActorSteamID64, ev.ActorName, ev.ActorTeamKey,
			ev.TargetSteamID64, ev.TargetName, ev.TargetTeamKey,
			ev.AssisterSteamID, ev.AssisterName, ev.Weapon,
			boolToInt(ev.IsHeadshot), boolToInt(ev.IsWallbang), boolToInt(ev.IsThroughSmoke), boolToInt(ev.IsNoscope),
			boolToInt(ev.IsAttackerBlind), boolToInt(ev.IsAssistedFlash), ev.Site, ev.ExtraJSON,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert match_event tick=%d: %w", ev.Tick, err)
		}
	}
	return tx.Commit()
}

func (r *AnalysisRepository) ListMatchEvents(ctx context.Context, demoID int64, roundNumber *int) ([]MatchEvent, error) {
	query := `
		SELECT
			id, demo_id, round_number, tick, round_time_sec, event_type,
			actor_steam_id64, actor_name, actor_team_key,
			target_steam_id64, target_name, target_team_key,
			assister_steam_id64, assister_name, weapon,
			is_headshot, is_wallbang, is_through_smoke, is_noscope,
			is_attacker_blind, is_assisted_flash, site, extra_json
		FROM match_events WHERE demo_id = ?`
	args := []any{demoID}
	if roundNumber != nil {
		query += ` AND round_number = ?`
		args = append(args, *roundNumber)
	}
	query += ` ORDER BY round_number ASC, tick ASC, id ASC`

	rows, err := r.db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MatchEvent
	for rows.Next() {
		var ev MatchEvent
		var hs, wb, ts, ns, ab, af int
		if err := rows.Scan(
			&ev.ID, &ev.DemoID, &ev.RoundNumber, &ev.Tick, &ev.RoundTimeSec, &ev.EventType,
			&ev.ActorSteamID64, &ev.ActorName, &ev.ActorTeamKey,
			&ev.TargetSteamID64, &ev.TargetName, &ev.TargetTeamKey,
			&ev.AssisterSteamID, &ev.AssisterName, &ev.Weapon,
			&hs, &wb, &ts, &ns, &ab, &af, &ev.Site, &ev.ExtraJSON,
		); err != nil {
			return nil, err
		}
		ev.IsHeadshot = hs != 0
		ev.IsWallbang = wb != 0
		ev.IsThroughSmoke = ts != 0
		ev.IsNoscope = ns != 0
		ev.IsAttackerBlind = ab != 0
		ev.IsAssistedFlash = af != 0
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (r *AnalysisRepository) ReplaceHighlightClips(ctx context.Context, demoID int64, clips []HighlightClip) error {
	tx, err := r.db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM highlight_clips WHERE demo_id = ?`, demoID); err != nil {
		_ = tx.Rollback()
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO highlight_clips (
			id, demo_id, player_steam_id64, player_name, round_number, clip_type,
			kill_count, start_tick, end_tick, anchor_ticks_json, pre_buffer_sec, post_buffer_sec,
			tags_json, weapon_label, description, importance_score, is_favorite, media_video_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, c := range clips {
		anchors := c.AnchorTicksJSON
		if anchors == "" {
			anchors = "[]"
		}
		tags := c.TagsJSON
		if tags == "" {
			tags = "[]"
		}
		created := c.CreatedAt
		if created == "" {
			created = utcNowISO()
		}
		if _, err := stmt.ExecContext(ctx,
			c.ID, demoID, c.PlayerSteamID64, c.PlayerName, c.RoundNumber, c.ClipType,
			c.KillCount, c.StartTick, c.EndTick, anchors, c.PreBufferSec, c.PostBufferSec,
			tags, c.WeaponLabel, c.Description, c.ImportanceScore, boolToInt(c.IsFavorite), c.MediaVideoID, created,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert highlight_clip %s: %w", c.ID, err)
		}
	}
	return tx.Commit()
}

func (r *AnalysisRepository) ListHighlightClips(ctx context.Context, demoID int64) ([]HighlightClip, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT
			id, demo_id, player_steam_id64, player_name, round_number, clip_type,
			kill_count, start_tick, end_tick, anchor_ticks_json, pre_buffer_sec, post_buffer_sec,
			tags_json, weapon_label, description, importance_score, is_favorite, media_video_id, created_at
		FROM highlight_clips WHERE demo_id = ? ORDER BY importance_score DESC, round_number ASC`, demoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HighlightClip
	for rows.Next() {
		var c HighlightClip
		var fav int
		if err := rows.Scan(
			&c.ID, &c.DemoID, &c.PlayerSteamID64, &c.PlayerName, &c.RoundNumber, &c.ClipType,
			&c.KillCount, &c.StartTick, &c.EndTick, &c.AnchorTicksJSON, &c.PreBufferSec, &c.PostBufferSec,
			&c.TagsJSON, &c.WeaponLabel, &c.Description, &c.ImportanceScore, &fav, &c.MediaVideoID, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		c.IsFavorite = fav != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *AnalysisRepository) UpsertRadarCard(ctx context.Context, card RadarCard) error {
	if card.UpdatedAt == "" {
		card.UpdatedAt = utcNowISO()
	}
	_, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO radar_cards (
			id, demo_id, steam_id64, player_name, team_key, team_label,
			dimensions_json, match_avg_json, static_image_path, anim_video_path,
			custom_portrait_path, custom_team_logo_path, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(demo_id, steam_id64) DO UPDATE SET
			player_name = excluded.player_name,
			team_key = excluded.team_key,
			team_label = excluded.team_label,
			dimensions_json = excluded.dimensions_json,
			match_avg_json = excluded.match_avg_json,
			static_image_path = excluded.static_image_path,
			anim_video_path = COALESCE(excluded.anim_video_path, radar_cards.anim_video_path),
			custom_portrait_path = COALESCE(excluded.custom_portrait_path, radar_cards.custom_portrait_path),
			custom_team_logo_path = COALESCE(excluded.custom_team_logo_path, radar_cards.custom_team_logo_path),
			updated_at = excluded.updated_at`,
		card.ID, card.DemoID, card.SteamID64, card.PlayerName, card.TeamKey, card.TeamLabel,
		card.DimensionsJSON, card.MatchAvgJSON, card.StaticImagePath, card.AnimVideoPath,
		card.CustomPortraitPath, card.CustomTeamLogoPath, card.UpdatedAt,
	)
	return err
}

func (r *AnalysisRepository) ListRadarCards(ctx context.Context, demoID int64) ([]RadarCard, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT
			id, demo_id, steam_id64, player_name, team_key, team_label,
			dimensions_json, match_avg_json, static_image_path, anim_video_path,
			custom_portrait_path, custom_team_logo_path, updated_at
		FROM radar_cards WHERE demo_id = ? ORDER BY team_key ASC, player_name ASC`, demoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RadarCard
	for rows.Next() {
		var c RadarCard
		if err := rows.Scan(
			&c.ID, &c.DemoID, &c.SteamID64, &c.PlayerName, &c.TeamKey, &c.TeamLabel,
			&c.DimensionsJSON, &c.MatchAvgJSON, &c.StaticImagePath, &c.AnimVideoPath,
			&c.CustomPortraitPath, &c.CustomTeamLogoPath, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
