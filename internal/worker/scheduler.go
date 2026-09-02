package worker

import (
	"context"
	"log"
	"time"

	"github.com/jb843051627/pellet-line/internal/service"
)

// Scheduler 后台定时扫描器：每间隔执行一次保养/巡检/挂起批次提醒。
type Scheduler struct {
	App   *service.App
	Every time.Duration
	stop  chan struct{}
	done  chan struct{}
}

// NewScheduler 构造调度器。
func NewScheduler(app *service.App) *Scheduler {
	return &Scheduler{
		App:   app,
		Every: 30 * time.Second,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
}

// Start 启动后台循环。
func (s *Scheduler) Start() {
	go s.loop()
}

// Stop 停止循环（幂等）。
func (s *Scheduler) Stop() {
	select {
	case <-s.stop:
		return
	default:
	}
	close(s.stop)
	<-s.done
}

func (s *Scheduler) loop() {
	defer close(s.done)
	ticker := time.NewTicker(s.Every)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			res, err := s.App.Scanner().ScanNow(ctx)
			cancel()
			if err != nil {
				log.Printf("scanner: scan failed: %v", err)
				continue
			}
			s.logResult(res)
		}
	}
}

func (s *Scheduler) logResult(res service.ScanResult) {
	for _, code := range res.ServiceDueEquipment {
		log.Printf("scanner: equipment %s service due", code)
	}
	for _, id := range res.OverdueInspections {
		log.Printf("scanner: inspection %s overdue", id)
	}
	for _, id := range res.HeldBatches {
		log.Printf("scanner: batch %s held for recheck", id)
	}
}
