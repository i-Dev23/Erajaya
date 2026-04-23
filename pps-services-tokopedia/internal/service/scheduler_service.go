package service

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// SchedulerService defines the interface for scheduling tasks
type SchedulerService interface {
	Start()
	Stop() context.Context
	AddJob(spec string, cmd func()) (cron.EntryID, error)
	AddJobWithContext(spec string, cmd func(ctx context.Context)) (cron.EntryID, error)
}

type schedulerServiceImpl struct {
	cron   *cron.Cron
	logger Logger
}

// NewSchedulerService creates a new scheduler service
func NewSchedulerService(logger Logger) SchedulerService {
	// Create cron with seconds support and logging
	c := cron.New(
		cron.WithSeconds(),
		cron.WithLocation(time.Local),
		cron.WithLogger(cron.VerbosePrintfLogger(&cronLoggerAdapter{logger: logger})),
	)

	return &schedulerServiceImpl{
		cron:   c,
		logger: logger,
	}
}

// Start starts the scheduler
func (s *schedulerServiceImpl) Start() {
	s.logger.Info("Starting scheduler service")
	s.cron.Start()
}

// Stop stops the scheduler gracefully
func (s *schedulerServiceImpl) Stop() context.Context {
	s.logger.Info("Stopping scheduler service")
	return s.cron.Stop()
}

// AddJob adds a new job to the scheduler
// spec format: "seconds minutes hours day month weekday"
// Example: "0 0 1 * * *" means "at 1:00 AM every day"
func (s *schedulerServiceImpl) AddJob(spec string, cmd func()) (cron.EntryID, error) {
	entryID, err := s.cron.AddFunc(spec, cmd)
	if err != nil {
		s.logger.Error("Failed to add job to scheduler", "spec", spec, "error", err)
		return 0, fmt.Errorf("failed to add job: %w", err)
	}

	s.logger.Info("Job added to scheduler", "entryID", entryID, "spec", spec)
	return entryID, nil
}

// AddJobWithContext adds a new job with context support to the scheduler
func (s *schedulerServiceImpl) AddJobWithContext(spec string, cmd func(ctx context.Context)) (cron.EntryID, error) {
	entryID, err := s.cron.AddFunc(spec, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd(ctx)
	})
	if err != nil {
		s.logger.Error("Failed to add job with context to scheduler", "spec", spec, "error", err)
		return 0, fmt.Errorf("failed to add job: %w", err)
	}

	s.logger.Info("Job with context added to scheduler", "entryID", entryID, "spec", spec)
	return entryID, nil
}

// cronLoggerAdapter adapts our Logger interface to cron's logger interface
type cronLoggerAdapter struct {
	logger Logger
}

func (a *cronLoggerAdapter) Printf(format string, args ...interface{}) {
	a.logger.Info(fmt.Sprintf(format, args...))
}

func (a *cronLoggerAdapter) Print(args ...interface{}) {
	a.logger.Info(fmt.Sprint(args...))
}
