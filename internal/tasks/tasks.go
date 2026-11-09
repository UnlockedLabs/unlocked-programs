// Package tasks is Programs' worker (`app worker`): class activation, on its
// schedule.
package tasks

import (
	"context"
	"fmt"
	"os"

	"github.com/UnlockedLabs/unlocked-programs/internal/database"
	"github.com/go-co-op/gocron/v2"
	log "github.com/sirupsen/logrus"
)

// Work runs the worker until ctx is canceled. Canceling ctx also stops a run
// that's in progress.
func Work(ctx context.Context, db *database.DB) error {
	s, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}
	scheduleJobs(ctx, cronScheduler{s}, db, os.Getenv("APP_ENV") == "dev")
	s.Start()
	log.Info("Worker started")

	<-ctx.Done()
	if err := s.Shutdown(); err != nil {
		return fmt.Errorf("stop scheduler: %w", err)
	}
	log.Info("Worker stopped")
	return nil
}
