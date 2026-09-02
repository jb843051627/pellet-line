package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jb843051627/pellet-line/internal/model"
)

// ReadingService 在线含水率采样业务。
type ReadingService struct {
	App *App
}

// IngestReadingInput 单条读数入参。
type IngestReadingInput struct {
	PointCode string  `json:"point_code"`
	LineCode  string  `json:"line_code"`
	Moisture  float64 `json:"moisture"`
	TempC     float64 `json:"temp_c"`
	ObservedAt *time.Time `json:"observed_at,omitempty"`
}

// IngestReading 落一条在线读数并更新缓存。
func (s *ReadingService) IngestReading(ctx context.Context, in IngestReadingInput) (model.Reading, error) {
	point, err := s.App.requirePoint(ctx, in.PointCode)
	if err != nil {
		return model.Reading{}, err
	}
	if point.IsFeed {
		return model.Reading{}, fmt.Errorf("%w: feed point cannot accept pellet moisture", model.ErrInvalidInput)
	}
	if in.Moisture < 0 || in.Moisture > 30 {
		return model.Reading{}, fmt.Errorf("%w: moisture out of range", model.ErrInvalidInput)
	}
	observed := time.Now()
	if in.ObservedAt != nil {
		observed = *in.ObservedAt
	}
	reading := model.Reading{
		ID:         s.App.NextID("rd"),
		PointCode:  in.PointCode,
		LineCode:   in.LineCode,
		Moisture:   in.Moisture,
		TempC:      in.TempC,
		ObservedAt: observed,
	}
	if err := s.App.DB.InsertReading(ctx, &reading); err != nil {
		return model.Reading{}, err
	}
	s.App.CacheLatest(in.PointCode, in.Moisture)
	s.App.CacheRecent(in.PointCode, reading)
	return reading, nil
}

// IngestReadings 批量读数（同一请求整批写入）。
func (s *ReadingService) IngestReadings(ctx context.Context, items []IngestReadingInput) (int, error) {
	if len(items) == 0 {
		return 0, fmt.Errorf("%w: empty batch", model.ErrInvalidInput)
	}
	prepared := make([]model.Reading, 0, len(items))
	for _, in := range items {
		if err := guardContext(ctx); err != nil {
			return 0, err
		}
		point, err := s.App.DB.GetSamplePoint(ctx, in.PointCode)
		if err != nil {
			return 0, err
		}
		if point.IsFeed {
			return 0, fmt.Errorf("%w: feed point cannot accept pellet moisture", model.ErrInvalidInput)
		}
		if in.Moisture < 0 || in.Moisture > 30 {
			return 0, fmt.Errorf("%w: moisture out of range", model.ErrInvalidInput)
		}
		observed := time.Now()
		if in.ObservedAt != nil {
			observed = *in.ObservedAt
		}
		prepared = append(prepared, model.Reading{
			ID:         s.App.NextID("rd"),
			PointCode:  in.PointCode,
			LineCode:   in.LineCode,
			Moisture:   in.Moisture,
			TempC:      in.TempC,
			ObservedAt: observed,
		})
	}
	if err := s.App.DB.InsertReadings(ctx, prepared); err != nil {
		return 0, err
	}
	for _, r := range prepared {
		s.App.CacheLatest(r.PointCode, r.Moisture)
		s.App.CacheRecent(r.PointCode, r)
	}
	return len(prepared), nil
}

// ListReadings 查询读数（点 + 时间窗 + limit）。
func (s *ReadingService) ListReadings(ctx context.Context, pointCode string, from, to time.Time, limit int) ([]model.Reading, error) {
	return s.App.DB.ListReadings(ctx, pointCode, from, to, limit)
}

// LatestByPoint 读取某点最新读数。
func (s *ReadingService) LatestByPoint(ctx context.Context, pointCode string) (model.Reading, error) {
	return s.App.DB.LatestReading(ctx, pointCode)
}

// RecentByPoint 返回某点位最近缓存读数（来自内存 ring，供趋势展示）。
func (s *ReadingService) RecentByPoint(pointCode string) []model.Reading {
	items := s.App.RecentReadings(pointCode)
	// 升序返回，供趋势图从早到晚绘制。
	sort.Slice(items, func(i, j int) bool { return items[i].ObservedAt.Before(items[j].ObservedAt) })
	return items
}

// InspectionService 巡检业务。
type InspectionService struct {
	App *App
}

// PlanInspectionInput 巡检计划入参。
type PlanInspectionInput struct {
	EquipmentCode string             `json:"equipment_code"`
	Kind          model.InspectionKind `json:"kind"`
	PlannedAt     time.Time          `json:"planned_at"`
	Note          string             `json:"note"`
}

// PlanInspection 建立巡检计划。
func (s *InspectionService) PlanInspection(ctx context.Context, in PlanInspectionInput) (model.Inspection, error) {
	if in.EquipmentCode == "" {
		return model.Inspection{}, fmt.Errorf("%w: equipment required", model.ErrInvalidInput)
	}
	if _, err := s.App.DB.GetEquipmentByCode(ctx, in.EquipmentCode); err != nil {
		return model.Inspection{}, err
	}
	if in.Kind != model.InspectionRoutine && in.Kind != model.InspectionMain {
		return model.Inspection{}, fmt.Errorf("%w: unknown kind", model.ErrInvalidInput)
	}
	insp := model.Inspection{
		ID:            s.App.NextID("insp"),
		EquipmentCode: in.EquipmentCode,
		Kind:          in.Kind,
		PlannedAt:     in.PlannedAt,
		State:         model.InspPlanned,
		Note:          in.Note,
	}
	if err := s.App.DB.CreateInspection(ctx, &insp); err != nil {
		return model.Inspection{}, err
	}
	return insp, nil
}

// CompleteInspection 完成巡检。
func (s *InspectionService) CompleteInspection(ctx context.Context, id string) (model.Inspection, error) {
	insp, err := s.App.DB.GetInspection(ctx, id)
	if err != nil {
		return model.Inspection{}, err
	}
	if insp.State == model.InspDone {
		return model.Inspection{}, fmt.Errorf("%w: already done", model.ErrConflict)
	}
	if err := s.App.DB.CompleteInspection(ctx, id, s.App.Clock.Now()); err != nil {
		return model.Inspection{}, err
	}
	return s.App.DB.GetInspection(ctx, id)
}

// ListInspections 巡检列表。
func (s *InspectionService) ListInspections(ctx context.Context, state string) ([]model.Inspection, error) {
	return s.App.DB.ListInspections(ctx, state)
}
