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

const lotColumns = `id, supplier_code, material, arrived_at, weight_tonnes,
	moisture_pct, ash_pct, state, grade`

func scanLot(row interface{ Scan(...any) error }) (model.FeedstockLot, error) {
	var l model.FeedstockLot
	var arrived string
	var moist, ash sql.NullFloat64
	err := row.Scan(&l.ID, &l.SupplierCode, &l.Material, &arrived, &l.WeightTonnes,
		&moist, &ash, &l.State, &l.Grade)
	if err != nil {
		return model.FeedstockLot{}, err
	}
	if moist.Valid {
		l.MoisturePct = &moist.Float64
	}
	if ash.Valid {
		l.AshPct = &ash.Float64
	}
	t, err := ParseTime(arrived)
	if err != nil {
		return model.FeedstockLot{}, err
	}
	l.ArrivedAt = t
	return l, nil
}

// CreateLot 登记进场原料批次。
func (d *DB) CreateLot(ctx context.Context, l *model.FeedstockLot) error {
	_, err := d.SQL.ExecContext(ctx, `INSERT INTO lots (`+lotColumns+`) VALUES (?,?,?,?,?,?,?,?,?)`,
		l.ID, l.SupplierCode, l.Material, NowText(l.ArrivedAt), l.WeightTonnes,
		nullF(l.MoisturePct), nullF(l.AshPct), string(l.State), l.Grade)
	if err != nil {
		return fmt.Errorf("store: create lot: %w", err)
	}
	return nil
}

// GetLot 按 ID 查询原料批次。不存在返回 ErrNotFound。
func (d *DB) GetLot(ctx context.Context, id string) (model.FeedstockLot, error) {
	row := d.SQL.QueryRowContext(ctx, `SELECT `+lotColumns+` FROM lots WHERE id = ?`, id)
	l, err := scanLot(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.FeedstockLot{}, ErrNotFound
		}
		return model.FeedstockLot{}, fmt.Errorf("store: get lot: %w", err)
	}
	return l, nil
}

// ListLots 按供应商或状态过滤。
func (d *DB) ListLots(ctx context.Context, supplier string, state string) ([]model.FeedstockLot, error) {
	q := `SELECT ` + lotColumns + ` FROM lots WHERE 1=1`
	var args []any
	if supplier != "" {
		q += ` AND supplier_code = ?`
		args = append(args, supplier)
	}
	if state != "" {
		q += ` AND state = ?`
		args = append(args, state)
	}
	q += ` ORDER BY arrived_at DESC`
	rows, err := d.SQL.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list lots: %w", err)
	}
	defer rows.Close()
	out := []model.FeedstockLot{}
	for rows.Next() {
		l, err := scanLot(rows)
		if err != nil {
			return nil, fmt.Errorf("store: list lots scan: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// UpdateLotState 更新批次状态。
func (d *DB) UpdateLotState(ctx context.Context, id string, state model.LotState) error {
	r, err := d.SQL.ExecContext(ctx, `UPDATE lots SET state = ? WHERE id = ?`, string(state), id)
	if err != nil {
		return fmt.Errorf("store: update lot state: %w", err)
	}
	affected, _ := r.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// AssayLot 写化验结果并落等级。
func (d *DB) AssayLot(ctx context.Context, id string, moist, ash float64, grade string) error {
	r, err := d.SQL.ExecContext(ctx,
		`UPDATE lots SET moisture_pct = ?, ash_pct = ?, grade = ? WHERE id = ?`,
		moist, ash, grade, id)
	if err != nil {
		return fmt.Errorf("store: assay lot: %w", err)
	}
	affected, _ := r.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// ConsumeLots 批量标记原料消耗（用于建批锁定原料）。
func (d *DB) ConsumeLots(ctx context.Context, ids []string) error {
	return d.WithTx(ctx, func(tx *sql.Tx) error {
		for _, id := range ids {
			r, err := tx.ExecContext(ctx, `UPDATE lots SET state = ? WHERE id = ? AND state = ?`,
				string(model.LotConsumed), id, string(model.LotReceived))
			if err != nil {
				return fmt.Errorf("store: consume lot %s: %w", id, err)
			}
			affected, _ := r.RowsAffected()
			if affected == 0 {
				return fmt.Errorf("%w: lot %s is not receivable", ErrNotFound, id)
			}
		}
		return nil
	})
}

func nullF(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func encodeLots(ids []string) string {
	b, _ := json.Marshal(ids)
	return string(b)
}

func decodeLots(raw string) []string {
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// 确保空值时间在 batch 查询复用。
var _ = time.Now
