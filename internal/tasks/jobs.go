package tasks

import (
	"context"
	"time"

	"github.com/UnlockedLabs/unlocked-contracts/identity"
	"github.com/UnlockedLabs/unlocked-programs/internal/database"
	"github.com/go-co-op/gocron/v2"
	log "github.com/sirupsen/logrus"
)

const (
	// everyMorningAt5AM is when class activation runs, in the worker's time
	// zone.
	everyMorningAt5AM = "0 5 * * *"
	activationTimeout = 30 * time.Minute
)

// scheduler runs the scheduled jobs.
type scheduler interface {
	// Cron runs task on a standard five-field cron schedule.
	Cron(spec string, task func()) error
	// AtStart runs task once, when the scheduler starts.
	AtStart(task func()) error
}

// cronScheduler is the scheduler, on gocron, as core's was. Unlike core's, it
// runs the at-start job right away: core held it back a minute for workers
// that start alongside it, and class activation needs none.
type cronScheduler struct{ gocron.Scheduler }

func (s cronScheduler) Cron(spec string, task func()) error {
	_, err := s.NewJob(gocron.CronJob(spec, false), gocron.NewTask(task))
	return err
}

func (s cronScheduler) AtStart(task func()) error {
	_, err := s.NewJob(gocron.OneTimeJob(gocron.OneTimeJobStartImmediately()), gocron.NewTask(task))
	return err
}

// scheduleJobs runs class activation every morning and, outside dev, once
// at start. Canceling ctx stops a run that's in progress.
func scheduleJobs(ctx context.Context, s scheduler, db *database.DB, dev bool) {
	run := func() {
		ctx, cancel := context.WithTimeout(ctx, activationTimeout)
		defer cancel()
		if err := activateScheduledClasses(ctx, db); err != nil {
			log.Errorf("failed to activate scheduled classes: %v", err)
		}
	}
	if err := s.Cron(everyMorningAt5AM, run); err != nil {
		log.Errorf("Failed to schedule class activation: %v", err)
	}
	if dev {
		return
	}
	if err := s.AtStart(run); err != nil {
		log.Errorf("Failed to schedule class activation at start: %v", err)
	}
}

// activateScheduledClasses makes Active every Scheduled class whose start
// date has come in its facility's time zone, and gives its Enrolled
// enrollments that have no enrolled date today's date. It makes the changes
// as the system_batch user, which the models' update hooks record.
func activateScheduledClasses(ctx context.Context, db *database.DB) error {
	batchUserID, err := db.SystemBatchUserID(ctx)
	if err != nil {
		return err
	}
	cohortIDs, err := db.ScheduledCohortsDue(ctx)
	if err != nil {
		return err
	}
	if len(cohortIDs) == 0 {
		log.Infoln("no scheduled classes are due for activation")
		return nil
	}
	log.Infof("activating %d scheduled class(es): %v", len(cohortIDs), cohortIDs)
	batchCtx := identity.NewContext(ctx, &identity.Claims{UserID: batchUserID})
	return db.ActivateCohorts(batchCtx, cohortIDs, batchUserID, time.Now().UTC())
}
