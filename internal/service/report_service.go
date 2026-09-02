package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jb843051627/pellet-line/internal/model"
	"github.com/jb843051627/pellet-line/internal/store"
)

// ReportService 质量报表与汇总。
type ReportService struct {
	App *App
}

// BatchSummary 单批质量明细行。
type BatchSummary struct {
	BatchID      string  `json:"batch_id"`
	LineCode     string  `json:"line_code"`
	Grade        string  `json:"grade"`
	State        string  `json:"state"`
	MoistureMean float64 `json:"moisture_mean"`
	AshMean      float64 `json:"ash_mean"`
	OutputTonnes float64 `json:"output_tonnes"`
}

// ShiftReport 班次质量报告。
type ShiftReport struct {
	From          time.Time      `json:"from"`
	To            time.Time      `json:"to"`
	TotalBatches  int            `json:"total_batches"`
	PassedBatches int            `json:"passed_batches"`
	HeldBatches   int            `json:"held_batches"`
	AvgMoisture   float64        `json:"avg_moisture"`
	Rows          []BatchSummary `json:"rows"`
}

// BuildShiftReport 统计某时间窗内批次质量。
func (s *ReportService) BuildShiftReport(ctx context.Context, from, to time.Time) (ShiftReport, error) {
	if !to.After(from) {
		return ShiftReport{}, fmt.Errorf("%w: window reversed", model.ErrInvalidInput)
	}
	batchRows, err := s.App.DB.ListBatches(ctx, "", "")
	if err != nil {
		return ShiftReport{}, err
	}
	rep := ShiftReport{From: from, To: to, Rows: []BatchSummary{}}
	for _, b := range batchRows {
		if err := guardContext(ctx); err != nil {
			return ShiftReport{}, err
		}
		if b.ProducedAt.Before(from) || b.ProducedAt.After(to) {
			continue
		}
		rep.TotalBatches++
		if b.State == model.BatchPassed || b.State == model.BatchClosed {
			rep.PassedBatches++
		}
		if b.State == model.BatchHeld {
			rep.HeldBatches++
		}
		rep.Rows = append(rep.Rows, BatchSummary{
			BatchID:      b.ID,
			LineCode:     b.LineCode,
			Grade:        b.Grade,
			State:        string(b.State),
			MoistureMean: val(b.MoistureMean),
			AshMean:      val(b.AshMean),
			OutputTonnes: b.OutputTonnes,
		})
	}
	sort.Slice(rep.Rows, func(i, j int) bool { return rep.Rows[i].BatchID < rep.Rows[j].BatchID })
	var sum float64
	var n int
	for _, r := range rep.Rows {
		if r.Grade == model.GradeA || r.Grade == model.GradeB {
			sum += r.MoistureMean
			n++
		}
	}
	if n > 0 {
		rep.AvgMoisture = sum / float64(n)
	}
	return rep, nil
}

// ExportBatchCSV 导出批次质量 CSV。
func (s *ReportService) ExportBatchCSV(ctx context.Context, since time.Time) (string, error) {
	stats, err := s.App.DB.SumBatchesSince(ctx, since)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("since,total,passed,held,tonnes,avg_moisture\n")
	sb.WriteString(fmt.Sprintf("%s,%d,%d,%d,%.1f,%.2f\n",
		since.Format(time.RFC3339), stats.Total, stats.Passed, stats.Held, stats.Tonnes, stats.AvgMoisture))
	return sb.String(), nil
}

// DashboardSummary 看板聚合。
type DashboardSummary struct {
	OpenBatches         int                `json:"open_batches"`
	OpenLots            int                `json:"open_lots"`
	HeldBatches         int                `json:"held_batches"`
	DueInspections      int                `json:"due_inspections"`
	ServiceDueEquipment int                `json:"service_due_equipment"`
	LatestByStation     map[string]float64 `json:"latest_by_station"`
}

// BuildDashboard 汇总看板指标。
func (s *ReportService) BuildDashboard(ctx context.Context) (DashboardSummary, error) {
	sum := DashboardSummary{LatestByStation: map[string]float64{}}
	batches, err := s.App.DB.ListBatches(ctx, "", "")
	if err != nil {
		return sum, err
	}
	for _, b := range batches {
		switch b.State {
		case model.BatchDraft, model.BatchRunning, model.BatchQC, model.BatchHeld:
			sum.OpenBatches++
		}
		if b.State == model.BatchHeld {
			sum.HeldBatches++
		}
	}
	lots, err := s.App.DB.ListLots(ctx, "", "")
	if err != nil {
		return sum, err
	}
	for _, l := range lots {
		if l.State == model.LotReceived || l.State == model.LotDraft {
			sum.OpenLots++
		}
	}
	insps, err := s.App.DB.ListInspections(ctx, string(model.InspPlanned))
	if err != nil {
		return sum, err
	}
	now := s.App.Clock.Now()
	for _, i := range insps {
		if i.OverdueInspection(now) {
			sum.DueInspections++
		}
	}
	equip, err := s.App.DB.ListEquipment(ctx)
	if err != nil {
		return sum, err
	}
	for _, e := range equip {
		if ServiceDue(e) && e.Status != "serviced" {
			sum.ServiceDueEquipment++
		}
	}
	points, err := s.App.DB.ListSamplePoints(ctx)
	if err != nil {
		return sum, err
	}
	stationOf := make(map[string]string, len(points))
	for _, p := range points {
		if p.IsFeed {
			continue
		}
		stationOf[p.Code] = p.Station
	}
	latest := s.App.LatestSnapshot()
	for code, moisture := range latest {
		if station, ok := stationOf[code]; ok {
			sum.LatestByStation[station] = moisture
		}
	}
	return sum, nil
}
var _ = store.ErrNotFound
