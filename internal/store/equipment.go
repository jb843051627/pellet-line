package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jb843051627/pellet-line/internal/model"
)

// Equipment 设备台账与运行/保养数据。
type Equipment struct {
	ID            string    `json:"id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	RuntimeHours  float64   `json:"runtime_hours"`
	ServiceDueHours float64 `json:"service_due_hours"`
	Status        string    `json:"status"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const equipmentColumns = `id, code, name, runtime_hours, service_due_hours, status`

// UpsertEquipment 写入/更新设备。
func (d *DB) UpsertEquipment(ctx context.Context, e *Equipment) error {
	_, err := d.SQL.ExecContext(ctx,
		`INSERT INTO equipment (`+equipmentColumns+`) VALUES (?,?,?,?,?,?)
		 ON CONFLICT(code) DO UPDATE SET runtime_hours = excluded.runtime_hours,
		   service_due_hours = excluded.service_due_hours, status = excluded.status`,
		e.ID, e.Code, e.Name, e.RuntimeHours, e.ServiceDueHours, e.Status)
	if err != nil {
		return fmt.Errorf("store: upsert equipment: %w", err)
	}
	return nil
}

// GetEquipmentByCode 查设备。
func (d *DB) GetEquipmentByCode(ctx context.Context, code string) (Equipment, error) {
	var e Equipment
	err := d.SQL.QueryRowContext(ctx,
		`SELECT `+equipmentColumns+` FROM equipment WHERE code = ?`, code).
		Scan(&e.ID, &e.Code, &e.Name, &e.RuntimeHours, &e.ServiceDueHours, &e.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Equipment{}, ErrNotFound
		}
		return Equipment{}, fmt.Errorf("store: get equipment: %w", err)
	}
	return e, nil
}

// ListEquipment 设备清单。
func (d *DB) ListEquipment(ctx context.Context) ([]Equipment, error) {
	rows, err := d.SQL.QueryContext(ctx,
		`SELECT `+equipmentColumns+` FROM equipment ORDER BY code ASC`)
	if err != nil {
		return nil, fmt.Errorf("store: list equipment: %w", err)
	}
	defer rows.Close()
	out := []Equipment{}
	for rows.Next() {
		var e Equipment
		if err := rows.Scan(&e.ID, &e.Code, &e.Name, &e.RuntimeHours, &e.ServiceDueHours, &e.Status); err != nil {
			return nil, fmt.Errorf("store: list equipment scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AddRuntimeHours 累加设备运行小时并写状态。
func (d *DB) AddRuntimeHours(ctx context.Context, code string, hours float64) error {
	r, err := d.SQL.ExecContext(ctx,
		`UPDATE equipment SET runtime_hours = runtime_hours + ? WHERE code = ?`, hours, code)
	if err != nil {
		return fmt.Errorf("store: add runtime: %w", err)
	}
	affected, _ := r.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// ResetRuntime 保养后清零运行小时。
func (d *DB) ResetRuntime(ctx context.Context, code string) error {
	r, err := d.SQL.ExecContext(ctx,
		`UPDATE equipment SET runtime_hours = 0, status = ? WHERE code = ?`, "serviced", code)
	if err != nil {
		return fmt.Errorf("store: reset runtime: %w", err)
	}
	affected, _ := r.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// EquipmentStatusChange 设备状态变更记录（审计）。
type EquipmentStatusChange struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
	ChangedAt time.Time `json:"changed_at"`
}

// LogStatusChange 追加设备状态变更审计。
func (d *DB) LogStatusChange(ctx context.Context, c *EquipmentStatusChange) error {
	_, err := d.SQL.ExecContext(ctx,
		`INSERT INTO status_changes (id, code, old_status, new_status, changed_at) VALUES (?,?,?,?,?)`,
		c.ID, c.Code, c.OldStatus, c.NewStatus, NowText(c.ChangedAt))
	if err != nil {
		return fmt.Errorf("store: log status change: %w", err)
	}
	return nil
}

// 设备状态合法值。
var EquipmentStatuses = []string{"online", "idle", "maintenance", "fault", "serviced"}

// ValidateEquipmentStatus 校验状态可接受。
func ValidateEquipmentStatus(status string) error {
	for _, s := range EquipmentStatuses {
		if s == status {
			return nil
		}
	}
	return fmt.Errorf("%w: unknown equipment status %s", model.ErrInvalidInput, status)
}
