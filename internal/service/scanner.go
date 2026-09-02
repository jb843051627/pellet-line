package service

import (
	"context"
	"time"

	"github.com/jb843051627/pellet-line/internal/model"
	"github.com/jb843051627/pellet-line/internal/store"
)

// Scanner 后台任务：设备保养提醒 / 巡检超期 / 原料积压扫描。
type Scanner struct {
	App *App
}

// ScanResult 单次扫描摘要。
type ScanResult struct {
	ServiceDueEquipment []string `json:"service_due_equipment"`
	OverdueInspections  []string `json:"overdue_inspections"`
	HeldBatches         []string `json:"held_batches"`
}

// ScanNow 执行一轮扫描（供 worker 定时与手动触发）。
func (s *Scanner) ScanNow(ctx context.Context) (ScanResult, error) {
	var res ScanResult
	equip, err := s.App.DB.ListEquipment(ctx)
	if err != nil {
		return res, err
	}
	for _, e := range equip {
		if ServiceDue(e) && e.Status != "serviced" {
			res.ServiceDueEquipment = append(res.ServiceDueEquipment, e.Code)
		}
	}
	insps, err := s.App.DB.ListInspections(ctx, string(model.InspPlanned))
	if err != nil {
		return res, err
	}
	now := s.App.Clock.Now()
	for _, i := range insps {
		if i.OverdueInspection(now) {
			res.OverdueInspections = append(res.OverdueInspections, i.ID)
		}
	}
	batches, err := s.App.DB.ListBatches(ctx, "", string(model.BatchHeld))
	if err != nil {
		return res, err
	}
	for _, b := range batches {
		res.HeldBatches = append(res.HeldBatches, b.ID)
	}
	return res, nil
}

// 兼容时钟导入。
var _ = store.NowText

// 周期常量。
const (
	scanInterval = 30 * time.Second
)
