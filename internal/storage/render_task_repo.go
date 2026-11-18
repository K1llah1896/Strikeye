package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

// RenderTask represents a row in render_tasks.
type RenderTask struct {
	ID                int64
	TaskUID           string
	DemoID            int64
	ClipID            *string
	RoundNumber       *int
	TargetSteamID64   string
	TargetPlayerName  string
	TargetSpecSlot    int
	Perspective       string
	TaskScope         string
	SegmentsJSON      string
	RenderOptionsJSON string
	Priority          int
	Status            string
	RetryCount        int
	MaxRetries        int
	SpawnedHLAEPID    *int
	SpawnedCS2PID     *int
	OutputMediaID     *int64
	ErrorCode         *string
	ErrorMessage      *string
	CreatedAt         string
	StartedAt         *string
	FinishedAt        *string
}

// RenderTaskRepository manages persistence for render_tasks.
type RenderTaskRepository struct {
	db *DB
}

// RenderTasks returns a RenderTaskRepository bound to db.
func (db *DB) RenderTasks() *RenderTaskRepository {
	return &RenderTaskRepository{db: db}
}

// ComputeTaskUID builds a deterministic idempotency key for a render task.
func ComputeTaskUID(demoSHA256, scope, playerSteamID64, perspective, segmentsJSON, optionsJSON string) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s", demoSHA256, scope, playerSteamID64, perspective, segmentsJSON, optionsJSON)
	return hex.EncodeToString(h.Sum(nil))
}

// EnqueueIdempotent inserts a render task unless task_uid already exists.
// When task_uid already exists, the existing task is returned with created=false.
func (r *RenderTaskRepository) EnqueueIdempotent(ctx context.Context, task RenderTask) (RenderTask, bool, error) {
	if task.TaskUID == "" {
		return RenderTask{}, false, fmt.Errorf("task_uid is required")
	}
	existing, err := r.GetByUID(ctx, task.TaskUID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return RenderTask{}, false, err
	}

	if task.Perspective == "" {
		task.Perspective = "pov"
	}
	if task.Priority == 0 {
		task.Priority = 10
	}
	if task.Status == "" {
		task.Status = "queued"
	}
	if task.MaxRetries == 0 {
		task.MaxRetries = 2
	}
	if task.CreatedAt == "" {
		task.CreatedAt = utcNowISO()
	}

	res, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO render_tasks (
			task_uid, demo_id, clip_id, round_number, target_steam_id64, target_player_name,
			target_spec_slot, perspective, task_scope, segments_json, render_options_json,
			priority, status, retry_count, max_retries, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.TaskUID, task.DemoID, task.ClipID, task.RoundNumber, task.TargetSteamID64, task.TargetPlayerName,
		task.TargetSpecSlot, task.Perspective, task.TaskScope, task.SegmentsJSON, task.RenderOptionsJSON,
		task.Priority, task.Status, task.RetryCount, task.MaxRetries, task.CreatedAt,
	)
	if err != nil {
		return RenderTask{}, false, fmt.Errorf("insert render_task: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return RenderTask{}, false, err
	}
	created, err := r.GetByID(ctx, id)
	return created, true, err
}

func (r *RenderTaskRepository) GetByID(ctx context.Context, id int64) (RenderTask, error) {
	return scanRenderTask(r.db.sql.QueryRowContext(ctx, renderTaskSelectBase+` WHERE id = ?`, id))
}

func (r *RenderTaskRepository) GetByUID(ctx context.Context, uid string) (RenderTask, error) {
	return scanRenderTask(r.db.sql.QueryRowContext(ctx, renderTaskSelectBase+` WHERE task_uid = ?`, uid))
}

func (r *RenderTaskRepository) NextQueued(ctx context.Context) (RenderTask, error) {
	return scanRenderTask(r.db.sql.QueryRowContext(
		ctx,
		renderTaskSelectBase+` WHERE status = 'queued' ORDER BY priority DESC, id ASC LIMIT 1`,
	))
}

func (r *RenderTaskRepository) ListByStatus(ctx context.Context, status string) ([]RenderTask, error) {
	rows, err := r.db.sql.QueryContext(
		ctx,
		renderTaskSelectBase+` WHERE status = ? ORDER BY priority DESC, id ASC`,
		status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RenderTask
	for rows.Next() {
		t, err := scanRenderTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *RenderTaskRepository) MarkRunning(ctx context.Context, id int64, hlaePID, cs2PID *int) error {
	_, err := r.db.sql.ExecContext(
		ctx,
		`UPDATE render_tasks SET status = 'running', spawned_hlae_pid = ?, spawned_cs2_pid = ?, started_at = ? WHERE id = ?`,
		hlaePID, cs2PID, utcNowISO(), id,
	)
	return err
}

func (r *RenderTaskRepository) MarkCompleted(ctx context.Context, id int64, outputMediaID int64) error {
	_, err := r.db.sql.ExecContext(
		ctx,
		`UPDATE render_tasks SET status = 'completed', output_media_id = ?, error_code = NULL, error_message = NULL, finished_at = ? WHERE id = ?`,
		outputMediaID, utcNowISO(), id,
	)
	return err
}

func (r *RenderTaskRepository) MarkFailed(ctx context.Context, id int64, code, message string) error {
	_, err := r.db.sql.ExecContext(
		ctx,
		`UPDATE render_tasks SET status = 'failed', error_code = ?, error_message = ?, finished_at = ? WHERE id = ?`,
		code, message, utcNowISO(), id,
	)
	return err
}

func (r *RenderTaskRepository) Cancel(ctx context.Context, id int64) error {
	_, err := r.db.sql.ExecContext(
		ctx,
		`UPDATE render_tasks SET status = 'cancelled', finished_at = ? WHERE id = ? AND status IN ('queued', 'running')`,
		utcNowISO(), id,
	)
	return err
}

// Retry resets a failed or cancelled task back to 'queued' so the scheduler can pick it up.
func (r *RenderTaskRepository) Retry(ctx context.Context, id int64) error {
	_, err := r.db.sql.ExecContext(
		ctx,
		`UPDATE render_tasks
		 SET status = 'queued',
		     error_code = NULL,
		     error_message = NULL,
		     started_at = NULL,
		     finished_at = NULL,
		     spawned_hlae_pid = NULL,
		     spawned_cs2_pid = NULL
		 WHERE id = ? AND status IN ('failed', 'cancelled')`,
		id,
	)
	return err
}

// UpdatePriority updates the scheduling priority for a queued render task.
func (r *RenderTaskRepository) UpdatePriority(ctx context.Context, id int64, priority int) error {
	_, err := r.db.sql.ExecContext(
		ctx,
		`UPDATE render_tasks SET priority = ? WHERE id = ? AND status = 'queued'`,
		priority, id,
	)
	return err
}

// RecoverInterrupted transitions stale 'running' tasks after an agent crash:
// tasks with remaining retries return to 'queued'; exhausted tasks become 'failed'.
func (r *RenderTaskRepository) RecoverInterrupted(ctx context.Context) (requeued int64, failed int64, err error) {
	tx, err := r.db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}

	resFail, err := tx.ExecContext(ctx, `
		UPDATE render_tasks
		SET status = 'failed',
		    error_code = 'AGENT_INTERRUPTED',
		    error_message = 'agent restarted while task exceeded max retries',
		    spawned_hlae_pid = NULL,
		    spawned_cs2_pid = NULL,
		    finished_at = ?
		WHERE status = 'running' AND retry_count >= max_retries`,
		utcNowISO(),
	)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, err
	}
	failed, _ = resFail.RowsAffected()

	resRequeue, err := tx.ExecContext(ctx, `
		UPDATE render_tasks
		SET status = 'queued',
		    retry_count = retry_count + 1,
		    error_code = 'AGENT_INTERRUPTED',
		    spawned_hlae_pid = NULL,
		    spawned_cs2_pid = NULL
		WHERE status = 'running' AND retry_count < max_retries`,
	)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, err
	}
	requeued, _ = resRequeue.RowsAffected()

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return requeued, failed, nil
}

const renderTaskSelectBase = `SELECT
	id, task_uid, demo_id, clip_id, round_number, target_steam_id64, target_player_name,
	target_spec_slot, perspective, task_scope, segments_json, render_options_json,
	priority, status, retry_count, max_retries, spawned_hlae_pid, spawned_cs2_pid,
	output_media_id, error_code, error_message, created_at, started_at, finished_at
FROM render_tasks`

func scanRenderTask(s rowScanner) (RenderTask, error) {
	var t RenderTask
	err := s.Scan(
		&t.ID, &t.TaskUID, &t.DemoID, &t.ClipID, &t.RoundNumber, &t.TargetSteamID64, &t.TargetPlayerName,
		&t.TargetSpecSlot, &t.Perspective, &t.TaskScope, &t.SegmentsJSON, &t.RenderOptionsJSON,
		&t.Priority, &t.Status, &t.RetryCount, &t.MaxRetries, &t.SpawnedHLAEPID, &t.SpawnedCS2PID,
		&t.OutputMediaID, &t.ErrorCode, &t.ErrorMessage, &t.CreatedAt, &t.StartedAt, &t.FinishedAt,
	)
	if err != nil {
		return RenderTask{}, err
	}
	return t, nil
}
