package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jb843051627/pellet-line/internal/model"

	_ "modernc.org/sqlite"
)

// ErrNotFound 统一引用 model 哨兵（同实例，errors.Is 才能识别）。
var ErrNotFound = model.ErrNotFound

// DB 封装磁盘 SQLite 连接，集中管理 schema 与通用查询能力。
type DB struct {
	SQL *sql.DB
}

// Open 打开磁盘数据库（默认禁止 :memory:）。文件不存在时创建目录。
func Open(path string) (*DB, error) {
	if path == ":memory:" || path == "" {
		return nil, fmt.Errorf("store: 必须使用真实磁盘文件，禁止内存模式 %q", path)
	}
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	db := &DB{SQL: conn}
	if err := db.migrate(context.Background()); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

// Close 关闭底层连接。
func (d *DB) Close() error {
	return d.SQL.Close()
}

// migrate 创建全部表。
func (d *DB) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS lots (
			id TEXT PRIMARY KEY,
			supplier_code TEXT NOT NULL,
			material TEXT NOT NULL,
			arrived_at TEXT NOT NULL,
			weight_tonnes REAL NOT NULL,
			moisture_pct REAL,
			ash_pct REAL,
			state TEXT NOT NULL,
			grade TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS batches (
			id TEXT PRIMARY KEY,
			line_code TEXT NOT NULL,
			recipe_code TEXT NOT NULL,
			lot_ids TEXT NOT NULL,
			state TEXT NOT NULL,
			started_at TEXT,
			finished_at TEXT,
			output_tonnes REAL NOT NULL,
			produced_at TEXT NOT NULL,
			grade TEXT NOT NULL,
			moisture_mean REAL,
			ash_mean REAL,
			closed_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS inspections (
			id TEXT PRIMARY KEY,
			equipment_code TEXT NOT NULL,
			kind TEXT NOT NULL,
			planned_at TEXT NOT NULL,
			state TEXT NOT NULL,
			note TEXT NOT NULL,
			completed_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS readings (
			id TEXT PRIMARY KEY,
			point_code TEXT NOT NULL,
			line_code TEXT NOT NULL,
			moisture REAL NOT NULL,
			temp_c REAL NOT NULL,
			observed_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS points (
			id TEXT PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			station TEXT NOT NULL,
			is_feed INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS equipment (
			id TEXT PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			runtime_hours REAL NOT NULL,
			service_due_hours REAL NOT NULL,
			status TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS status_changes (
			id TEXT PRIMARY KEY,
			code TEXT NOT NULL,
			old_status TEXT NOT NULL,
			new_status TEXT NOT NULL,
			changed_at TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := d.SQL.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	return nil
}

// Ping 数据库健康检查。
func (d *DB) Ping(ctx context.Context) error {
	return d.SQL.PingContext(ctx)
}

// WithTx 在事务内执行 fn，成功提交失败回滚并返回错误。
func (d *DB) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	tx = nil
	return nil
}

// NowText 返回统一格式时间文本。
func NowText(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// ParseTime 解析文本时间为 time.Time。
func ParseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
