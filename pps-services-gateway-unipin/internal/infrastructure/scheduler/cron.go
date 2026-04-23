package scheduler

import (
	"context"

	contractsvc "pps-services-gateway-unipin/internal/domain/contract/service"

	"github.com/robfig/cron/v3"
)

// Scheduler wraps robfig/cron for scheduled tasks.
type Scheduler struct {
	cron   *cron.Cron
	logger contractsvc.Logger
}

// New creates a new Scheduler with the given timezone location.
func New(logger contractsvc.Logger) *Scheduler {
	c := cron.New()
	return &Scheduler{cron: c, logger: logger}
}

// AddGameSync registers the game list sync job.
func (s *Scheduler) AddGameSync(cronExpr string, syncSvc contractsvc.GameSyncService) error {
	_, err := s.cron.AddFunc(cronExpr, func() {
		s.logger.Info("cron: game list sync triggered")
		if err := syncSvc.SyncGameList(context.Background()); err != nil {
			s.logger.Error("cron: game list sync failed", "error", err)
		}

		s.logger.Info("cron: voucher list sync triggered")
		if err := syncSvc.SyncVoucherList(context.Background()); err != nil {
			s.logger.Error("cron: voucher list sync failed", "error", err)
		}
	})
	return err
}

// Start begins the cron scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
	s.logger.Info("cron scheduler started")
}

// Stop gracefully stops the cron scheduler.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("cron scheduler stopped")
}
