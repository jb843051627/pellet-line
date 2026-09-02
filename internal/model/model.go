package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// 领域哨兵错误。业务层与存储层统一使用，供上层以 errors.Is/errors.As 判定。
var (
	ErrNotFound        = errors.New("pellet: not found")
	ErrConflict        = errors.New("pellet: conflict")
	ErrInvalidInput    = errors.New("pellet: invalid input")
	ErrStateTransition = errors.New("pellet: illegal state transition")
	ErrDuplicate       = errors.New("pellet: duplicate")
)

// 水分/灰分等级（客户交付口径与复检门限共用）。
const (
	GradeA  = "A"
	GradeB  = "B"
	GradeRe = "R"
)

// 时间相关常量。
const (
	MoisturePerGrading   = 8.0  // A/B 级水分分界
	MoistureMaxAccepted  = 11.0 // 高于此水分判 R 级（复检）上限
	AshPerGrading        = 2.0  // A/B 级灰分分界
	AshMaxAccepted       = 4.0  // 高于此灰分判 R 级上限
	MilliKWhPerCycle     = 120  // 单批次额定能耗（kWh），用于效率计算
	BatchSizeStandard    = 500  // 标准批吨位
	SpecFirewoodMoisture = 20.0 // 松木片进料水分上限
)

// LotState 原料进场批次状态机。
type LotState string

const (
	LotDraft    LotState = "draft"     // 待化验
	LotReceived LotState = "received"  // 已进场收料
	LotRejected LotState = "rejected"  // 拒收
	LotConsumed LotState = "consumed"  // 已消耗
)

// 原料批次：进场登记 + 水分/灰分化验。
type FeedstockLot struct {
	ID           string    `json:"id"`
	SupplierCode string    `json:"supplier_code"`
	Material     string    `json:"material"`
	ArrivedAt    time.Time `json:"arrived_at"`
	WeightTonnes float64   `json:"weight_tonnes"`
	MoisturePct  *float64  `json:"moisture_pct,omitempty"`
	AshPct       *float64  `json:"ash_pct,omitempty"`
	State        LotState  `json:"state"`
	Grade        string    `json:"grade"`
}

// CanReceive 校验能否从 draft 收料（先化验）。
func (l *FeedstockLot) CanReceive() bool {
	return l.State == LotDraft && l.MoisturePct != nil && l.AshPct != nil
}

// Assess 依据化验水分/灰分输出等级，并推进接收/拒收判定。
func (l *FeedstockLot) Assess() (string, error) {
	if l.MoisturePct == nil || l.AshPct == nil {
		return "", fmt.Errorf("%w: lot missing assay result", ErrInvalidInput)
	}
	switch {
	case *l.MoisturePct > MoistureMaxAccepted || *l.AshPct > AshMaxAccepted:
		return GradeRe, nil
	case *l.MoisturePct <= MoisturePerGrading && *l.AshPct <= AshPerGrading:
		return GradeA, nil
	default:
		return GradeB, nil
	}
}

// Receive 收料。
func (l *FeedstockLot) Receive() error {
	if !l.CanReceive() {
		return fmt.Errorf("%w: lot state %s cannot receive", ErrStateTransition, l.State)
	}
	l.State = LotReceived
	return nil
}

// Consume 标记消耗。
func (l *FeedstockLot) Consume() error {
	switch l.State {
	case LotReceived:
		l.State = LotConsumed
		return nil
	case LotConsumed:
		return nil
	default:
		return fmt.Errorf("%w: lot state %s cannot consume", ErrStateTransition, l.State)
	}
}

// SamplePoint 采样点（离线/在线读数聚合键）。
type SamplePoint struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Station string `json:"station"`
	IsFeed  bool   `json:"is_feed"`
}

// 可用的样本点类型集合。
var PointKinds = []string{"feed-in", "line-out", "warehouse"}

// PelletBatchState 制粒批次状态机。
type PelletBatchState string

const (
	BatchDraft   PelletBatchState = "draft"    // 建批
	BatchRunning PelletBatchState = "running"  // 制粒中
	BatchQC      PelletBatchState = "qc"       // 待质检
	BatchPassed  PelletBatchState = "passed"   // 质检通过
	BatchHeld    PelletBatchState = "held"     // 复检挂起
	BatchClosed  PelletBatchState = "closed"   // 封批出库
)

// 允许的制粒批次状态迁移。
var pelletBatchTransitions = map[PelletBatchState][]PelletBatchState{
	BatchDraft:   {BatchRunning},
	BatchRunning: {BatchQC, BatchHeld},
	BatchQC:      {BatchPassed, BatchHeld, BatchRunning},
	BatchHeld:    {BatchQC, BatchPassed},
	BatchPassed:  {BatchClosed, BatchHeld},
	BatchClosed:  {},
}

// CanTransition 校验批次是否允许 from -> to。
func CanTransition(from, to PelletBatchState) bool {
	for _, next := range pelletBatchTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// PelletBatch 制粒批次主记录。
type PelletBatch struct {
	ID            string          `json:"id"`
	LineCode      string          `json:"line_code"`
	RecipeCode    string          `json:"recipe_code"`
	LotIDs        []string        `json:"lot_ids"`
	State         PelletBatchState `json:"state"`
	StartedAt     *time.Time      `json:"started_at,omitempty"`
	FinishedAt    *time.Time      `json:"finished_at,omitempty"`
	OutputTonnes  float64         `json:"output_tonnes"`
	ProducedAt    time.Time       `json:"produced_at"`
	Grade         string          `json:"grade"`
	MoistureMean  *float64        `json:"moisture_mean,omitempty"`
	AshMean       *float64        `json:"ash_mean,omitempty"`
	ClosedAt      *time.Time      `json:"closed_at,omitempty"`
}

// InProduction 判断是否处于可被质量判定中断的生产状态。
func (b *PelletBatch) InProduction() bool {
	return b.State == BatchRunning || b.State == BatchDraft
}

// AllowedTransitions 返回当前批次允许迁移到的下一状态列表。
// 返回的是独立副本，调用方增删不会改动内部状态机表。
func (b *PelletBatch) AllowedTransitions() []PelletBatchState {
	allowed := pelletBatchTransitions[b.State]
	out := make([]PelletBatchState, len(allowed))
	copy(out, allowed)
	return out
}

// Quality 质检判定后落定的状态。
func (b *PelletBatch) Quality(moisture, ash float64) PelletBatchState {
	switch {
	case moisture > MoistureMaxAccepted || ash > AshMaxAccepted:
		return BatchHeld
	case moisture <= MoisturePerGrading && ash <= AshPerGrading:
		return BatchPassed
	default:
		return BatchPassed
	}
}

// GradeOf 由含水率/灰分归一为交付等级。
func GradeOf(moisture, ash float64) string {
	switch {
	case moisture > MoistureMaxAccepted || ash > AshMaxAccepted:
		return GradeRe
	case moisture <= MoisturePerGrading && ash <= AshPerGrading:
		return GradeA
	default:
		return GradeB
	}
}

// InspectionKind 巡检/维护类别。
type InspectionKind string

const (
	InspectionRoutine InspectionKind = "routine"
	InspectionMain    InspectionKind = "maintenance"
)

// InspectionState 巡检任务状态。
type InspectionState string

const (
	InspPlanned InspectionState = "planned"
	InspDue     InspectionState = "due"
	InspDone    InspectionState = "done"
	InspMissed  InspectionState = "missed"
)

// Inspection 设备巡检任务。
type Inspection struct {
	ID            string          `json:"id"`
	EquipmentCode string          `json:"equipment_code"`
	Kind          InspectionKind  `json:"kind"`
	PlannedAt     time.Time       `json:"planned_at"`
	State         InspectionState `json:"state"`
	Note          string          `json:"note"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
}

// OverdueInspection 超过计划时间仍未完成。
func (i *Inspection) OverdueInspection(now time.Time) bool {
	return i.State == InspPlanned && now.After(i.PlannedAt)
}

// Complete 完成巡检。
func (i *Inspection) Complete() error {
	switch i.State {
	case InspPlanned, InspDue:
		i.State = InspDone
		return nil
	case InspDone:
		return fmt.Errorf("%w: inspection already done", ErrConflict)
	default:
		return fmt.Errorf("%w: inspection %s cannot complete", ErrStateTransition, i.State)
	}
}

// Reading 在线颗粒出料含水率采样记录。
type Reading struct {
	ID        string    `json:"id"`
	PointCode string    `json:"point_code"`
	LineCode  string    `json:"line_code"`
	Moisture  float64   `json:"moisture"`
	TempC     float64   `json:"temp_c"`
	ObservedAt time.Time `json:"observed_at"`
}

// Measure 在线水分读数，返回是否超目标线。
func (r *Reading) Measure() float64 {
	return r.Moisture
}

// Flush 指标统计快照。
type FlushSummary struct {
	LineCode     string  `json:"line_code"`
	Batches      int     `json:"batches"`
	TotalTonnes  float64 `json:"total_tonnes"`
	MeanMoisture float64 `json:"mean_moisture"`
	HeldCount    int     `json:"held_count"`
}

// ValidateStateToken 统一校验状态迁移（通用守卫）。
func ValidateStateToken(current, next string, allowed map[string][]string) error {
	candidates, ok := allowed[current]
	if !ok {
		return fmt.Errorf("%w: unknown current state %s", ErrStateTransition, current)
	}
	for _, c := range candidates {
		if c == next {
			return nil
		}
	}
	return fmt.Errorf("%w: %s -> %s", ErrStateTransition, current, next)
}

// TrimID 内部工具：清洗外部传入的标识。
func TrimID(raw string) string {
	return strings.TrimSpace(raw)
}
