package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jb843051627/pellet-line/internal/model"
)

func nullT(v *time.Time) any {
	if v == nil {
		return nil
	}
	return NowText(*v)
}

const readingColumns = `id, point_code, line_code, moisture, temp_c, observed_at`

func scanReading(row interface{ Scan(...any) error }) (model.Reading, error) {
	var r model.Reading
	var observed string
	err := row.Scan(&r.ID, &r.PointCode, &r.LineCode, &r.Moisture, &r.TempC, &observed)
	if err != nil {
		return model.Reading{}, err
	}
	t, err := ParseTime(observed)
	if err != nil {
		return model.Reading{}, err
	}
	r.ObservedAt = t
	return r, nil
}

// InsertReading 落一条在线读数。
func (d *DB) InsertReading(ctx context.Context, r *model.Reading) error {
	_, err := d.SQL.ExecContext(ctx, `INSERT INTO readings (`+readingColumns+`) VALUES (?,?,?,?,?,?)`,
		r.ID, r.PointCode, r.LineCode, r.Moisture, r.TempC, NowText(r.ObservedAt))
	if err != nil {
		return fmt.Errorf("store: insert reading: %w", err)
	}
	return nil
}

// InsertReadings 批量插入（同事务，保证整批一致性）。
func (d *DB) InsertReadings(ctx context.Context, items []model.Reading) error {
	return d.WithTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO readings (`+readingColumns+`) VALUES (?,?,?,?,?,?)`)
		if err != nil {
			return fmt.Errorf("store: prepare readings: %w", err)
		}
		defer stmt.Close()
		for _, r := range items {
			if _, err := stmt.ExecContext(ctx, r.ID, r.PointCode, r.LineCode, r.Moisture, r.TempC, NowText(r.ObservedAt)); err != nil {
				return fmt.Errorf("store: insert reading batch: %w", err)
			}
		}
		return nil
	})
}

// ListReadings 按点位与时间窗口查询读数。
func (d *DB) ListReadings(ctx context.Context, pointCode string, from, to time.Time, limit int) ([]model.Reading, error) {
	q := `SELECT ` + readingColumns + ` FROM readings WHERE observed_at >= ? AND observed_at <= ?`
	args := []any{NowText(from), NowText(to)}
	if pointCode != "" {
		q += ` AND point_code = ?`
		args = append(args, pointCode)
	}
	q += ` ORDER BY observed_at ASC`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := d.SQL.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list readings: %w", err)
	}
	defer rows.Close()
	out := []model.Reading{}
	for rows.Next() {
		r, err := scanReading(rows)
		if err != nil {
			return nil, fmt.Errorf("store: list readings scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LatestReading 某点位最新读数。
func (d *DB) LatestReading(ctx context.Context, pointCode string) (model.Reading, error) {
	row := d.SQL.QueryRowContext(ctx,
		`SELECT `+readingColumns+` FROM readings WHERE point_code = ? ORDER BY observed_at DESC LIMIT 1`, pointCode)
	r, err := scanReading(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Reading{}, ErrNotFound
		}
		return model.Reading{}, fmt.Errorf("store: latest reading: %w", err)
	}
	return r, nil
}

const inspectionColumns = `id, equipment_code, kind, planned_at, state, note, completed_at`

func scanInspection(row interface{ Scan(...any) error }) (model.Inspection, error) {
	var i model.Inspection
	var planned string
	var completed sql.NullString
	err := row.Scan(&i.ID, &i.EquipmentCode, &i.Kind, &planned, &i.State, &i.Note, &completed)
	if err != nil {
		return model.Inspection{}, err
	}
	t, err := ParseTime(planned)
	if err != nil {
		return model.Inspection{}, err
	}
	i.PlannedAt = t
	if completed.Valid {
		if ct, e := ParseTime(completed.String); e == nil {
			i.CompletedAt = &ct
		}
	}
	return i, nil
}

// CreateInspection 计划一次巡检。
func (d *DB) CreateInspection(ctx context.Context, i *model.Inspection) error {
	_, err := d.SQL.ExecContext(ctx,
		`INSERT INTO inspections (`+inspectionColumns+`) VALUES (?,?,?,?,?,?,?)`,
		i.ID, i.EquipmentCode, string(i.Kind), NowText(i.PlannedAt), string(i.State), i.Note, nullT(i.CompletedAt))
	if err != nil {
		return fmt.Errorf("store: create inspection: %w", err)
	}
	return nil
}

// ListInspections 查询巡检计划（状态过滤）。
func (d *DB) ListInspections(ctx context.Context, state string) ([]model.Inspection, error) {
	q := `SELECT ` + inspectionColumns + ` FROM inspections`
	var args []any
	if state != "" {
		q += ` WHERE state = ?`
		args = append(args, state)
	}
	q += ` ORDER BY planned_at ASC`
	rows, err := d.SQL.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list inspections: %w", err)
	}
	defer rows.Close()
	out := []model.Inspection{}
	for rows.Next() {
		i, err := scanInspection(rows)
		if err != nil {
			return nil, fmt.Errorf("store: list inspections scan: %w", err)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// GetInspection 按 ID 查巡检。
func (d *DB) GetInspection(ctx context.Context, id string) (model.Inspection, error) {
	row := d.SQL.QueryRowContext(ctx,
		`SELECT `+inspectionColumns+` FROM inspections WHERE id = ?`, id)
	i, err := scanInspection(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Inspection{}, ErrNotFound
		}
		return model.Inspection{}, fmt.Errorf("store: get inspection: %w", err)
	}
	return i, nil
}

// CompleteInspection 完成巡检（仅当计划态）。
func (d *DB) CompleteInspection(ctx context.Context, id string, at time.Time) error {
	r, err := d.SQL.ExecContext(ctx,
		`UPDATE inspections SET state = ?, completed_at = ? WHERE id = ? AND state IN (?, ?)`,
		string(model.InspDone), NowText(at), id, string(model.InspPlanned), string(model.InspDue))
	if err != nil {
		return fmt.Errorf("store: complete inspection: %w", err)
	}
	affected, _ := r.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateSamplePoint 建采样点。
func (d *DB) CreateSamplePoint(ctx context.Context, p *model.SamplePoint) error {
	feed := 0
	if p.IsFeed {
		feed = 1
	}
	_, err := d.SQL.ExecContext(ctx,
		`INSERT INTO points (id, code, station, is_feed) VALUES (?,?,?,?)`,
		p.ID, p.Code, p.Station, feed)
	if err != nil {
		return fmt.Errorf("store: create sample point: %w", err)
	}
	return nil
}

// GetSamplePoint 查采样点。
func (d *DB) GetSamplePoint(ctx context.Context, code string) (model.SamplePoint, error) {
	var p model.SamplePoint
	var feed int
	err := d.SQL.QueryRowContext(ctx, `SELECT id, code, station, is_feed FROM points WHERE code = ?`, code).
		Scan(&p.ID, &p.Code, &p.Station, &feed)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.SamplePoint{}, ErrNotFound
		}
		return model.SamplePoint{}, fmt.Errorf("store: get sample point: %w", err)
	}
	p.IsFeed = feed == 1
	return p, nil
}

// ListSamplePoints 查全部采样点。
func (d *DB) ListSamplePoints(ctx context.Context) ([]model.SamplePoint, error) {
	rows, err := d.SQL.QueryContext(ctx, `SELECT id, code, station, is_feed FROM points ORDER BY code ASC`)
	if err != nil {
		return nil, fmt.Errorf("store: list sample points: %w", err)
	}
	defer rows.Close()
	out := []model.SamplePoint{}
	for rows.Next() {
		var p model.SamplePoint
		var feed int
		if err := rows.Scan(&p.ID, &p.Code, &p.Station, &feed); err != nil {
			return nil, fmt.Errorf("store: list points scan: %w", err)
		}
		p.IsFeed = feed == 1
		out = append(out, p)
	}
	return out, rows.Err()
}
