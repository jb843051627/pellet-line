package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jb843051627/pellet-line/internal/clock"
	"github.com/jb843051627/pellet-line/internal/model"
	"github.com/jb843051627/pellet-line/internal/store"
)

// recentPerPoint 每个点位在内存保留的最近读数条数。
const recentPerPoint = 20

// App 聚合所有业务能力（service 门面）。
type App struct {
	DB    *store.DB
	Clock clock.Clock

	seqMu    sync.Mutex
	sequence int64

	// 最新含水率缓存：并发读多写少，用 RWMutex 保护。
	latestMu sync.RWMutex
	latest   map[string]float64

	// 最近读数环状缓存：按点位保留最近读数（含排序保证），写少读多。
	recentMu sync.RWMutex
	recent   map[string][]model.Reading
}

// NewApp 构建应用。
func NewApp(db *store.DB) *App {
	return &App{
		DB:     db,
		Clock:  clock.System{},
		latest: make(map[string]float64),
		recent: make(map[string][]model.Reading),
	}
}

// Shutdown 释放资源（预留）。
func (a *App) Shutdown() {}

// HTTPStop 关闭 HTTP 服务（预留可扩展）。
func (a *App) HTTPStop(ctx context.Context) error {
	return nil
}

// NextID 生成单调递增业务 ID（前缀 + 时间戳 + 序号）。
func (a *App) NextID(prefix string) string {
	a.seqMu.Lock()
	a.sequence++
	value := a.sequence
	a.seqMu.Unlock()
	return fmt.Sprintf("%s-%s-%04d", prefix, a.Clock.Now().Format("20060102150405"), value)
}

// guardContext 请求取消守卫：ctx 已取消时返回 context 错误，供长耗时业务在
// 处理边界与循环内检查，保证请求取消能中断处理流程。
func guardContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// CacheLatest 写入某点最新含水率。
func (a *App) CacheLatest(pointCode string, moisture float64) {
	a.latestMu.Lock()
	a.latest[pointCode] = moisture
	a.latestMu.Unlock()
}

// LatestOf 读某点最新含水率（无记录返回 ok=false）。
func (a *App) LatestOf(pointCode string) (float64, bool) {
	a.latestMu.RLock()
	v, ok := a.latest[pointCode]
	a.latestMu.RUnlock()
	return v, ok
}

// LatestSnapshot 返回全部点位最新含水率的独立副本。
// 调用方写入返回结果不得影响内部缓存状态。
func (a *App) LatestSnapshot() map[string]float64 {
	a.latestMu.RLock()
	out := make(map[string]float64, len(a.latest))
	for k, v := range a.latest {
		out[k] = v
	}
	a.latestMu.RUnlock()
	return out
}

// CacheRecent 把一条读数写入点位最近缓存，保持按观测时间降序，只保留 recentPerPoint 条。
func (a *App) CacheRecent(pointCode string, r model.Reading) {
	a.recentMu.Lock()
	items := a.recent[pointCode]
	items = append(items, r)
	// 按观测时间倒序，最近在前
	for i := len(items) - 1; i > 0; i-- {
		if items[i].ObservedAt.After(items[i-1].ObservedAt) {
			items[i], items[i-1] = items[i-1], items[i]
		} else {
			break
		}
	}
	if len(items) > recentPerPoint {
		items = items[:recentPerPoint]
	}
	a.recent[pointCode] = items
	a.recentMu.Unlock()
}

// RecentReadings 返回某点位最近读数集合的独立副本。
// 调用方对返回结果做排序或修改不会改动内部缓存。
func (a *App) RecentReadings(pointCode string) []model.Reading {
	a.recentMu.RLock()
	items := a.recent[pointCode]
	out := items
	a.recentMu.RUnlock()
	return out
}

// PruneCacheBefore 清理缓存（占位，预留给清理任务）。
func (a *App) PruneCacheBefore(now time.Time) int {
	_ = now
	return 0
}

// requirePoint 查询采样点；不存在返回带哨兵的错误。返回指针便于上层业务访问字段。
func (a *App) requirePoint(ctx context.Context, code string) (*model.SamplePoint, error) {
	point, err := a.DB.GetSamplePoint(ctx, code)
	if err != nil {
		return nil, err
	}
	return &point, nil
}

// Scanner 返回后台扫描器。
func (a *App) Scanner() *Scanner {
	return &Scanner{App: a}
}
