package storage

import (
	"context"
	"fmt"
)

// MontageProject represents a row in montage_projects.
type MontageProject struct {
	ID         int64
	Name       string
	ConfigJSON string
	CreatedAt  string
	UpdatedAt  string
}

// MontageExport represents a row in montage_exports.
type MontageExport struct {
	ID              int64
	ProjectID       *int64
	OutputPath      string
	Status          string
	ProgressPercent float64
	ErrorMessage    *string
	MediaVideoID    *int64
	CreatedAt       string
	FinishedAt      *string
}

// MontageRepository manages montage_projects and montage_exports.
type MontageRepository struct {
	db *DB
}

// Montage returns a MontageRepository bound to db.
func (db *DB) Montage() *MontageRepository {
	return &MontageRepository{db: db}
}

func (r *MontageRepository) CreateProject(ctx context.Context, name, configJSON string) (MontageProject, error) {
	if name == "" {
		return MontageProject{}, fmt.Errorf("project name is required")
	}
	if configJSON == "" {
		configJSON = "{}"
	}
	now := utcNowISO()
	res, err := r.db.sql.ExecContext(
		ctx,
		`INSERT INTO montage_projects (name, config_json, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		name, configJSON, now, now,
	)
	if err != nil {
		return MontageProject{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return MontageProject{}, err
	}
	return r.GetProject(ctx, id)
}

func (r *MontageRepository) UpdateProject(ctx context.Context, id int64, name, configJSON string) error {
	_, err := r.db.sql.ExecContext(
		ctx,
		`UPDATE montage_projects SET name = ?, config_json = ?, updated_at = ? WHERE id = ?`,
		name, configJSON, utcNowISO(), id,
	)
	return err
}

func (r *MontageRepository) GetProject(ctx context.Context, id int64) (MontageProject, error) {
	var p MontageProject
	err := r.db.sql.QueryRowContext(
		ctx,
		`SELECT id, name, config_json, created_at, updated_at FROM montage_projects WHERE id = ?`,
		id,
	).Scan(&p.ID, &p.Name, &p.ConfigJSON, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *MontageRepository) ListProjects(ctx context.Context) ([]MontageProject, error) {
	rows, err := r.db.sql.QueryContext(
		ctx,
		`SELECT id, name, config_json, created_at, updated_at FROM montage_projects ORDER BY updated_at DESC, id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MontageProject
	for rows.Next() {
		var p MontageProject
		if err := rows.Scan(&p.ID, &p.Name, &p.ConfigJSON, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *MontageRepository) DeleteProject(ctx context.Context, id int64) error {
	_, err := r.db.sql.ExecContext(ctx, `DELETE FROM montage_projects WHERE id = ?`, id)
	return err
}

func (r *MontageRepository) CreateExport(ctx context.Context, projectID *int64, outputPath string) (MontageExport, error) {
	if outputPath == "" {
		return MontageExport{}, fmt.Errorf("output_path is required")
	}
	now := utcNowISO()
	res, err := r.db.sql.ExecContext(
		ctx,
		`INSERT INTO montage_exports (project_id, output_path, status, progress_percent, created_at) VALUES (?, ?, 'pending', 0.0, ?)`,
		projectID, outputPath, now,
	)
	if err != nil {
		return MontageExport{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return MontageExport{}, err
	}
	return r.GetExport(ctx, id)
}

func (r *MontageRepository) UpdateExportProgress(ctx context.Context, id int64, status string, progress float64) error {
	_, err := r.db.sql.ExecContext(
		ctx,
		`UPDATE montage_exports SET status = ?, progress_percent = ? WHERE id = ?`,
		status, progress, id,
	)
	return err
}

func (r *MontageRepository) CompleteExport(ctx context.Context, id int64, mediaVideoID int64) error {
	_, err := r.db.sql.ExecContext(
		ctx,
		`UPDATE montage_exports SET status = 'completed', progress_percent = 100.0, media_video_id = ?, finished_at = ? WHERE id = ?`,
		mediaVideoID, utcNowISO(), id,
	)
	return err
}

func (r *MontageRepository) FailExport(ctx context.Context, id int64, errMsg string) error {
	_, err := r.db.sql.ExecContext(
		ctx,
		`UPDATE montage_exports SET status = 'failed', error_message = ?, finished_at = ? WHERE id = ?`,
		errMsg, utcNowISO(), id,
	)
	return err
}

func (r *MontageRepository) GetExport(ctx context.Context, id int64) (MontageExport, error) {
	var e MontageExport
	err := r.db.sql.QueryRowContext(
		ctx,
		`SELECT id, project_id, output_path, status, progress_percent, error_message, media_video_id, created_at, finished_at FROM montage_exports WHERE id = ?`,
		id,
	).Scan(&e.ID, &e.ProjectID, &e.OutputPath, &e.Status, &e.ProgressPercent, &e.ErrorMessage, &e.MediaVideoID, &e.CreatedAt, &e.FinishedAt)
	return e, err
}

func (r *MontageRepository) ListExports(ctx context.Context) ([]MontageExport, error) {
	rows, err := r.db.sql.QueryContext(
		ctx,
		`SELECT id, project_id, output_path, status, progress_percent, error_message, media_video_id, created_at, finished_at FROM montage_exports ORDER BY id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MontageExport
	for rows.Next() {
		var e MontageExport
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.OutputPath, &e.Status, &e.ProgressPercent, &e.ErrorMessage, &e.MediaVideoID, &e.CreatedAt, &e.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
