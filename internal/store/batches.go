package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jb843051627/pellet-line/internal/model"
)

const batchColumns = `id, line_code, recipe_code, lot_ids, state,
	started_at, finished_at, output_tonnes, produced_at, grade, moisture_mean, ash_mean, closed_at`

func scanBatch(row interface{ Scan(...any) error }) (model.PelletBatch, error) {
	var b model.PelletBatch
	var lotIDs string
	var started, finished, closed sql.NullString
	var produced string
	var moistMean, ashMean sql.NullFloat64
	err := row.Scan(&b.ID, &b.LineCode, &b.RecipeCode, &lotIDs, &b.State,
		&started, &finished, &b.OutputTonnes, &produced, &b.Grade, &moistMean, &ashMean, &closed)
	if err != nil {
		return model.PelletBatch{}, err
	}
	b.LotIDs = decodeLots(lotIDs)
	if started.Valid {
		if t, e := ParseTime(started.String); e == nil {
			b.StartedAt = &t
		}
	}
	if finished.Valid {
		if t, e := ParseTime(finished.String); e == nil {
			b.FinishedAt = &t
		}
	}
	if closed.Valid {
		if t, e := ParseTime(closed.String); e == nil {
			b.ClosedAt = &t
		}
	}
	if moistMean.Valid {
		b.MoistureMean = &moistMean.Float64
	}
	if ashMean.Valid {
		b.AshMean = &ashMean.Float64
	}
	t, err := ParseTime(produced)
	if err != nil {
		return model.PelletBatch{}, err
	}
	b.ProducedAt = t
	return b, nil
}

// CreateBatch 创建制粒批次。
func (d *DB) CreateBatch(ctx context.Context, b *model.PelletBatch) error {
	_, err := d.SQL.ExecContext(ctx, `INSERT INTO batches (`+batchColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.ID, b.LineCode, b.RecipeCode, encodeLots(b.LotIDs), string(b.State),
		nullT(b.StartedAt), nullT(b.FinishedAt), b.OutputTonnes, NowText(b.ProducedAt),
		b.Grade, nullF(b.MoistureMean), nullF(b.AshMean), nullT(b.ClosedAt))
	if err != nil {
		return fmt.Errorf("store: create batch: %w", err)
	}
	return nil
}

// GetBatch 按 ID 查询批次。
func (d *DB) GetBatch(ctx context.Context, id string) (model.PelletBatch, error) {
	row := d.SQL.QueryRowContext(ctx, `SELECT `+batchColumns+` FROM batches WHERE id = ?`, id)
	b, err := scanBatch(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.PelletBatch{}, ErrNotFound
		}
		return model.PelletBatch{}, fmt.Errorf("store: get batch: %w", err)
	}
	return b, nil
}

// ListBatches 查询批次集合（按产线/状态过滤）。
func (d *DB) ListBatches(ctx context.Context, line string, state string) ([]model.PelletBatch, error) {
	q := `SELECT ` + batchColumns + ` FROM batches WHERE 1=1`
	var args []any
	if line != "" {
		q += ` AND line_code = ?`
		args = append(args, line)
	}
	if state != "" {
		q += ` AND state = ?`
		args = append(args, state)
	}
	q += ` ORDER BY produced_at DESC`
	rows, err := d.SQL.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list batches: %w", err)
	}
	defer rows.Close()
	out := []model.PelletBatch{}
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, fmt.Errorf("store: list batches scan: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateBatchState 更新批次状态（不写 finish/grade 之外字段）。
func (d *DB) UpdateBatchState(ctx context.Context, id string, state model.PelletBatchState) error {
	r, err := d.SQL.ExecContext(ctx, `UPDATE batches SET state = ? WHERE id = ?`, string(state), id)
	if err != nil {
		return fmt.Errorf("store: update batch state: %w", err)
	}
	affected, _ := r.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetBatchMetrics 写入质检均值与等级，并可在同一调用中切换状态。
func (d *DB) SetBatchMetrics(ctx context.Context, id string, moist, ash float64, grade string, state model.PelletBatchState) error {
	r, err := d.SQL.ExecContext(ctx,
		`UPDATE batches SET moisture_mean = ?, ash_mean = ?, grade = ?, state = ? WHERE id = ?`,
		moist, ash, grade, string(state), id)
	if err != nil {
		return fmt.Errorf("store: set batch metrics: %w", err)
	}
	affected, _ := r.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkBatchStarted 记录开始时刻。
func (d *DB) MarkBatchStarted(ctx context.Context, id string, at time.Time) error {
	r, err := d.SQL.ExecContext(ctx, `UPDATE batches SET state = ?, started_at = ? WHERE id = ?`,
		string(model.BatchRunning), NowText(at), id)
	if err != nil {
		return fmt.Errorf("store: mark batch started: %w", err)
	}
	affected, _ := r.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkBatchFinished 记录结束时刻并回到质检前态。
func (d *DB) MarkBatchFinished(ctx context.Context, id string, at time.Time) error {
	r, err := d.SQL.ExecContext(ctx, `UPDATE batches SET state = ?, finished_at = ? WHERE id = ?`,
		string(model.BatchQC), NowText(at), id)
	if err != nil {
		return fmt.Errorf("store: mark batch finished: %w", err)
	}
	affected, _ := r.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetBatchOutput 落产出吨位（完工后写入）。
func (d *DB) SetBatchOutput(ctx context.Context, id string, tonnes float64) error {
	r, err := d.SQL.ExecContext(ctx, `UPDATE batches SET output_tonnes = ? WHERE id = ?`, tonnes, id)
	if err != nil {
		return fmt.Errorf("store: set batch output: %w", err)
	}
	affected, _ := r.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// CloseBatch 封批（写出库时刻）。
func (d *DB) CloseBatch(ctx context.Context, id string, at time.Time) error {
	r, err := d.SQL.ExecContext(ctx,
		`UPDATE batches SET state = ?, closed_at = ? WHERE id = ? AND state IN (?, ?)`,
		string(model.BatchClosed), NowText(at), id,
		string(model.BatchPassed), string(model.BatchHeld))
	if err != nil {
		return fmt.Errorf("store: close batch: %w", err)
	}
	affected, _ := r.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// BatchStats 汇总最近若干批次关键统计（供报表）。
type BatchStats struct {
	Total   int
	Passed  int
	Held    int
	Tonnes  float64
	AvgMoisture float64
}

// SumBatchesSince 统计 since 之后批次质量。
func (d *DB) SumBatchesSince(ctx context.Context, since time.Time) (BatchStats, error) {
	rows, err := d.SQL.QueryContext(ctx,
		`SELECT state, output_tonnes, COALESCE(moisture_mean, 0) FROM batches WHERE produced_at >= ?`,
		NowText(since))
	if err != nil {
		return BatchStats{}, fmt.Errorf("store: sum batches: %w", err)
	}
	defer rows.Close()
	var s BatchStats
	var sumMoisture float64
	var countMoisture int
	for rows.Next() {
		var state string
		var tonnes, moist float64
		if err := rows.Scan(&state, &tonnes, &moist); err != nil {
			return BatchStats{}, err
		}
		s.Total++
		s.Tonnes += tonnes
		switch model.PelletBatchState(state) {
		case model.BatchPassed, model.BatchClosed:
			s.Passed++
		case model.BatchHeld:
			s.Held++
		}
		if model.PelletBatchState(state) == model.BatchPassed || model.PelletBatchState(state) == model.BatchClosed {
			sumMoisture += moist
			countMoisture++
		}
	}
	if countMoisture > 0 {
		s.AvgMoisture = sumMoisture / float64(countMoisture)
	}
	return s, rows.Err()
}

// 兼容占位。
var _ = json.Marshal
