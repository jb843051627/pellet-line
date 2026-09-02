package service

import (
	"context"
	"fmt"

	"github.com/jb843051627/pellet-line/internal/model"
)

// LotService 原料进场批次业务。
type LotService struct {
	App *App
}

// RegisterLotInput 进场登记入参。
type RegisterLotInput struct {
	SupplierCode string  `json:"supplier_code"`
	Material     string  `json:"material"`
	WeightTonnes float64 `json:"weight_tonnes"`
}

// RegisterLot 登记原料批次（draft，待化验）。
func (s *LotService) RegisterLot(ctx context.Context, in RegisterLotInput) (model.FeedstockLot, error) {
	if in.SupplierCode == "" || in.Material == "" {
		return model.FeedstockLot{}, fmt.Errorf("%w: supplier and material required", model.ErrInvalidInput)
	}
	if in.WeightTonnes <= 0 || in.WeightTonnes > 1000 {
		return model.FeedstockLot{}, fmt.Errorf("%w: weight out of range", model.ErrInvalidInput)
	}
	lot := model.FeedstockLot{
		ID:           s.App.NextID("lot"),
		SupplierCode: in.SupplierCode,
		Material:     in.Material,
		ArrivedAt:    s.App.Clock.Now(),
		WeightTonnes: in.WeightTonnes,
		State:        model.LotDraft,
		Grade:        "",
	}
	if err := s.App.DB.CreateLot(ctx, &lot); err != nil {
		return model.FeedstockLot{}, err
	}
	return lot, nil
}

// AssayLotInput 化验入参。
type AssayLotInput struct {
	LotID  string  `json:"lot_id"`
	MoisturePct float64 `json:"moisture_pct"`
	AshPct      float64 `json:"ash_pct"`
}

// AssayLot 提交化验结果并定级。
func (s *LotService) AssayLot(ctx context.Context, in AssayLotInput) (model.FeedstockLot, error) {
	lot, err := s.App.DB.GetLot(ctx, in.LotID)
	if err != nil {
		return model.FeedstockLot{}, fmt.Errorf("assay lot %q lookup: %w", in.LotID, err)
	}
	if lot.State != model.LotDraft {
		return model.FeedstockLot{}, fmt.Errorf("%w: lot already received", model.ErrStateTransition)
	}
	if in.MoisturePct < 0 || in.AshPct < 0 || in.MoisturePct > 40 || in.AshPct > 20 {
		return model.FeedstockLot{}, fmt.Errorf("%w: assay value out of range", model.ErrInvalidInput)
	}
	// 依据入参定级
	m := in.MoisturePct
	a := in.AshPct
	var finalGrade string
	switch {
	case m > model.MoistureMaxAccepted || a > model.AshMaxAccepted:
		finalGrade = model.GradeRe
	case m <= model.MoisturePerGrading && a <= model.AshPerGrading:
		finalGrade = model.GradeA
	default:
		finalGrade = model.GradeB
	}
	if err := s.App.DB.AssayLot(ctx, in.LotID, m, a, finalGrade); err != nil {
		return model.FeedstockLot{}, err
	}
	return s.App.DB.GetLot(ctx, in.LotID)
}

// ReceiveLot 收料（要求化验完成且未拒收）。
func (s *LotService) ReceiveLot(ctx context.Context, lotID string) (model.FeedstockLot, error) {
	lot, err := s.App.DB.GetLot(ctx, lotID)
	if err != nil {
		return model.FeedstockLot{}, fmt.Errorf("receive lot %q lookup: %w", lotID, err)
	}
	if lot.State != model.LotDraft {
		return model.FeedstockLot{}, fmt.Errorf("%w: lot not in draft", model.ErrStateTransition)
	}
	if err := lot.Receive(); err != nil {
		return model.FeedstockLot{}, err
	}
	if err := s.App.DB.UpdateLotState(ctx, lotID, lot.State); err != nil {
		return model.FeedstockLot{}, err
	}
	return lot, nil
}

// ListLots 原料列表。
func (s *LotService) ListLots(ctx context.Context, supplier, state string) ([]model.FeedstockLot, error) {
	return s.App.DB.ListLots(ctx, supplier, state)
}

// GetLot 详情。
func (s *LotService) GetLot(ctx context.Context, lotID string) (model.FeedstockLot, error) {
	return s.App.DB.GetLot(ctx, lotID)
}

// BatchService 制粒批次业务。
type BatchService struct {
	App *App
}

// CreateBatchInput 建批入参。
type CreateBatchInput struct {
	LineCode   string   `json:"line_code"`
	RecipeCode string   `json:"recipe_code"`
	LotIDs     []string `json:"lot_ids"`
}

// CreateBatch 建批并锁定原料（收到态原料才能进批）。
func (s *BatchService) CreateBatch(ctx context.Context, in CreateBatchInput) (model.PelletBatch, error) {
	if in.LineCode == "" || in.RecipeCode == "" {
		return model.PelletBatch{}, fmt.Errorf("%w: line and recipe required", model.ErrInvalidInput)
	}
	if len(in.LotIDs) == 0 {
		return model.PelletBatch{}, fmt.Errorf("%w: at least one lot required", model.ErrInvalidInput)
	}
	// 校验所有原料存在且已收
	for _, lotID := range in.LotIDs {
		lot, err := s.App.DB.GetLot(ctx, lotID)
		if err != nil {
			return model.PelletBatch{}, fmt.Errorf("batch material lot %q lookup: %w", lotID, err)
		}
		if lot.State != model.LotReceived {
			return model.PelletBatch{}, fmt.Errorf("%w: lot %s not received", model.ErrConflict, lotID)
		}
	}
	batch := model.PelletBatch{
		ID:          s.App.NextID("batch"),
		LineCode:    in.LineCode,
		RecipeCode:  in.RecipeCode,
		LotIDs:      append([]string(nil), in.LotIDs...),
		State:       model.BatchDraft,
		ProducedAt:  s.App.Clock.Now(),
		Grade:       "",
	}
	if err := s.App.DB.CreateBatch(ctx, &batch); err != nil {
		return model.PelletBatch{}, err
	}
	return batch, nil
}

// StartBatch 启动制粒（draft -> running）。
func (s *BatchService) StartBatch(ctx context.Context, batchID string) (model.PelletBatch, error) {
	batch, err := s.App.DB.GetBatch(ctx, batchID)
	if err != nil {
		return model.PelletBatch{}, err
	}
	if !model.CanTransition(batch.State, model.BatchRunning) {
		return model.PelletBatch{}, fmt.Errorf("%w: %s -> running", model.ErrStateTransition, batch.State)
	}
	if err := s.App.DB.MarkBatchStarted(ctx, batchID, s.App.Clock.Now()); err != nil {
		return model.PelletBatch{}, err
	}
	return s.App.DB.GetBatch(ctx, batchID)
}

// FinishProduction 产线端报完工（running -> qc，待质检）。
func (s *BatchService) FinishProduction(ctx context.Context, batchID string, outputTonnes float64) (model.PelletBatch, error) {
	batch, err := s.App.DB.GetBatch(ctx, batchID)
	if err != nil {
		return model.PelletBatch{}, err
	}
	if batch.State != model.BatchRunning {
		return model.PelletBatch{}, fmt.Errorf("%w: batch not running", model.ErrStateTransition)
	}
	if outputTonnes < 0 {
		return model.PelletBatch{}, fmt.Errorf("%w: negative output", model.ErrInvalidInput)
	}
	// 记录完工时刻并把产出吨位落库
	if err := s.App.DB.MarkBatchFinished(ctx, batchID, s.App.Clock.Now()); err != nil {
		return model.PelletBatch{}, err
	}
	if err := s.App.DB.SetBatchOutput(ctx, batchID, outputTonnes); err != nil {
		return model.PelletBatch{}, err
	}
	return s.App.DB.GetBatch(ctx, batchID)
}

// QCResult 质检判定入参。
type QCResult struct {
	BatchID      string  `json:"batch_id"`
	MoistureMean float64 `json:"moisture_mean"`
	AshMean      float64 `json:"ash_mean"`
}

// SubmitQC 质检提交：判定 passed 或 held。
func (s *BatchService) SubmitQC(ctx context.Context, in QCResult) (model.PelletBatch, error) {
	batch, err := s.App.DB.GetBatch(ctx, in.BatchID)
	if err != nil {
		return model.PelletBatch{}, err
	}
	if batch.State != model.BatchQC && batch.State != model.BatchHeld {
		return model.PelletBatch{}, fmt.Errorf("%w: batch not awaiting qc", model.ErrStateTransition)
	}
	grade := model.GradeOf(in.MoistureMean, in.AshMean)
	state := model.BatchPassed
	if grade == model.GradeRe {
		state = model.BatchHeld
	}
	if err := s.App.DB.SetBatchMetrics(ctx, in.BatchID, in.MoistureMean, in.AshMean, grade, state); err != nil {
		return model.PelletBatch{}, err
	}
	return s.App.DB.GetBatch(ctx, in.BatchID)
}

// RecheckQC 复检通过（held -> passed）。
func (s *BatchService) RecheckQC(ctx context.Context, batchID string) (model.PelletBatch, error) {
	batch, err := s.App.DB.GetBatch(ctx, batchID)
	if err != nil {
		return model.PelletBatch{}, err
	}
	if batch.State != model.BatchHeld {
		return model.PelletBatch{}, fmt.Errorf("%w: batch not held", model.ErrStateTransition)
	}
	grade := model.GradeB
	if batch.MoistureMean != nil && batch.AshMean != nil {
		grade = model.GradeOf(*batch.MoistureMean, *batch.AshMean)
	}
	if err := s.App.DB.SetBatchMetrics(ctx, batchID, val(batch.MoistureMean), val(batch.AshMean), grade, model.BatchPassed); err != nil {
		return model.PelletBatch{}, err
	}
	return s.App.DB.GetBatch(ctx, batchID)
}

// CloseBatch 封批出库（passed -> closed）。
func (s *BatchService) CloseBatch(ctx context.Context, batchID string) (model.PelletBatch, error) {
	batch, err := s.App.DB.GetBatch(ctx, batchID)
	if err != nil {
		return model.PelletBatch{}, err
	}
	if batch.State != model.BatchPassed {
		return model.PelletBatch{}, fmt.Errorf("%w: batch not passed", model.ErrStateTransition)
	}
	if err := s.App.DB.CloseBatch(ctx, batchID, s.App.Clock.Now()); err != nil {
		return model.PelletBatch{}, err
	}
	return s.App.DB.GetBatch(ctx, batchID)
}

// GetBatch 详情。
func (s *BatchService) GetBatch(ctx context.Context, batchID string) (model.PelletBatch, error) {
	return s.App.DB.GetBatch(ctx, batchID)
}

// ListBatches 批次列表。
func (s *BatchService) ListBatches(ctx context.Context, line, state string) ([]model.PelletBatch, error) {
	return s.App.DB.ListBatches(ctx, line, state)
}

func val(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
