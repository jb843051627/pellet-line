package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jb843051627/pellet-line/internal/store"
)

// EquipmentService 设备台账与保养。
type EquipmentService struct {
	App *App
}

// EquipmentView 对外设备视图。
type EquipmentView struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	RuntimeHours  float64 `json:"runtime_hours"`
	ServiceDueHours float64 `json:"service_due_hours"`
	Status        string  `json:"status"`
	ServiceDue    bool    `json:"service_due"`
	OverdueHours  float64 `json:"overdue_hours"`
}

// RegisterInput 注册设备入参。
type RegisterInput struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	RuntimeHours    float64 `json:"runtime_hours"`
	ServiceDueHours float64 `json:"service_due_hours"`
	Status          string  `json:"status"`
}

// Register 注册设备。
func (s *EquipmentService) Register(ctx context.Context, in RegisterInput) (store.Equipment, error) {
	if in.Code == "" || in.Name == "" {
		return store.Equipment{}, fmt.Errorf("service: code and name required")
	}
	if err := store.ValidateEquipmentStatus(in.Status); err != nil {
		return store.Equipment{}, err
	}
	e := store.Equipment{
		ID:              s.App.NextID("eq"),
		Code:            in.Code,
		Name:            in.Name,
		RuntimeHours:    in.RuntimeHours,
		ServiceDueHours: in.ServiceDueHours,
		Status:          in.Status,
	}
	if err := s.App.DB.UpsertEquipment(ctx, &e); err != nil {
		return store.Equipment{}, err
	}
	return e, nil
}

// View 单个设备（含保养到期判定）。
func (s *EquipmentService) View(ctx context.Context, code string) (EquipmentView, error) {
	e, err := s.App.DB.GetEquipmentByCode(ctx, code)
	if err != nil {
		return EquipmentView{}, fmt.Errorf("view equipment %q: %w", code, err)
	}
	return toView(e, s.App.Clock.Now()), nil
}

// List 设备清单。
func (s *EquipmentService) List(ctx context.Context) ([]EquipmentView, error) {
	all, err := s.App.DB.ListEquipment(ctx)
	if err != nil {
		return nil, err
	}
	now := s.App.Clock.Now()
	out := make([]EquipmentView, 0, len(all))
	for _, e := range all {
		out = append(out, toView(e, now))
	}
	return out, nil
}

// ServiceDue 判定保养是否到期。
func ServiceDue(e store.Equipment) bool {
	return e.RuntimeHours >= e.ServiceDueHours
}

func toView(e store.Equipment, now time.Time) EquipmentView {
	due := ServiceDue(e)
	over := 0.0
	if due {
		over = e.RuntimeHours - e.ServiceDueHours
	}
	return EquipmentView{
		Code:            e.Code,
		Name:            e.Name,
		RuntimeHours:    e.RuntimeHours,
		ServiceDueHours: e.ServiceDueHours,
		Status:          e.Status,
		ServiceDue:      due,
		OverdueHours:    over,
	}
}

// PerformService 执行保养并清零运行小时。
func (s *EquipmentService) PerformService(ctx context.Context, code string) (EquipmentView, error) {
	e, err := s.App.DB.GetEquipmentByCode(ctx, code)
	if err != nil {
		return EquipmentView{}, err
	}
	old := e.Status
	if err := s.App.DB.ResetRuntime(ctx, code); err != nil {
		return EquipmentView{}, err
	}
	if err := s.App.DB.LogStatusChange(ctx, &store.EquipmentStatusChange{
		ID:        s.App.NextID("chg"),
		Code:      code,
		OldStatus: old,
		NewStatus: "serviced",
		ChangedAt: s.App.Clock.Now(),
	}); err != nil {
		return EquipmentView{}, err
	}
	return s.View(ctx, code)
}

// ReportHours 上报运行小时。
func (s *EquipmentService) ReportHours(ctx context.Context, code string, hours float64) error {
	if hours < 0 {
		return fmt.Errorf("service: negative hours")
	}
	return s.App.DB.AddRuntimeHours(ctx, code, hours)
}
