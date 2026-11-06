package tasks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/UnlockedLabs/unlocked-contracts/identity"
	"github.com/UnlockedLabs/unlocked-programs/internal/database"
	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"github.com/UnlockedLabs/unlocked-programs/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeScheduler records what a module schedules.
type fakeScheduler struct {
	specs   []string
	atStart int
}

func (s *fakeScheduler) Cron(spec string, task func()) error {
	s.specs = append(s.specs, spec)
	return nil
}

func (s *fakeScheduler) AtStart(task func()) error {
	s.atStart++
	return nil
}

// Class activation runs every morning at 5 and, outside dev, once at start.
func TestScheduleJobs(t *testing.T) {
	for _, dev := range []bool{false, true} {
		s := &fakeScheduler{}
		scheduleJobs(context.Background(), s, database.NewDB(testutil.NewDB(t)), dev)
		assert.Equal(t, []string{"0 5 * * *"}, s.specs)
		wantAtStart := 1
		if dev {
			wantAtStart = 0
		}
		assert.Equal(t, wantAtStart, s.atStart, "dev=%v", dev)
	}
}

func createCohort(t *testing.T, db *gorm.DB, name string, status models.ClassStatus) models.ProgramClassCohort {
	t.Helper()
	program := models.Program{Name: name}
	require.NoError(t, db.Create(&program).Error)
	cohort := models.ProgramClassCohort{
		ProgramID:   program.ID,
		FacilityID:  1,
		Capacity:    10,
		Description: name,
		StartDt:     time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Status:      status,
	}
	require.NoError(t, db.Create(&cohort).Error)
	return cohort
}

func createEnrollment(t *testing.T, db *gorm.DB, cohort models.ProgramClassCohort, userID uint, status models.ProgramEnrollmentStatus, enrolledAt *time.Time) models.ProgramClassEnrollment {
	t.Helper()
	e := models.ProgramClassEnrollment{CohortID: cohort.ID, UserID: userID, EnrollmentStatus: status, EnrolledAt: enrolledAt}
	require.NoError(t, db.Create(&e).Error)
	return e
}

// Activating classes makes them Active and gives their Enrolled enrollments
// with no enrolled date the activation time, all as the batch user. Other
// classes and enrollments are left alone.
func TestActivateCohorts(t *testing.T) {
	db := testutil.NewDB(t)
	store := database.NewDB(db)
	due := createCohort(t, db, "due", models.Scheduled)
	other := createCohort(t, db, "other", models.Scheduled)
	earlier := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	undated := createEnrollment(t, db, due, 1, models.Enrolled, nil)
	dated := createEnrollment(t, db, due, 2, models.Enrolled, &earlier)
	dropped := createEnrollment(t, db, due, 3, models.EnrollmentIncompleteDropped, nil)
	elsewhere := createEnrollment(t, db, other, 4, models.Enrolled, nil)
	const batchUserID = 99
	at := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	ctx := identity.NewContext(context.Background(), &identity.Claims{UserID: batchUserID})
	require.NoError(t, store.ActivateCohorts(ctx, []int{int(due.ID)}, batchUserID, at))

	var got models.ProgramClassCohort
	require.NoError(t, db.First(&got, due.ID).Error)
	assert.Equal(t, models.Active, got.Status)
	require.NotNil(t, got.UpdateUserID)
	assert.Equal(t, uint(batchUserID), *got.UpdateUserID)
	var untouched models.ProgramClassCohort
	require.NoError(t, db.First(&untouched, other.ID).Error)
	assert.Equal(t, models.Scheduled, untouched.Status)

	enrolledAt := func(e models.ProgramClassEnrollment) *time.Time {
		var row models.ProgramClassEnrollment
		require.NoError(t, db.First(&row, e.ID).Error)
		return row.EnrolledAt
	}
	require.NotNil(t, enrolledAt(undated))
	assert.True(t, enrolledAt(undated).Equal(at))
	assert.True(t, enrolledAt(dated).Equal(earlier))
	assert.Nil(t, enrolledAt(dropped))
	assert.Nil(t, enrolledAt(elsewhere))
	var row models.ProgramClassEnrollment
	require.NoError(t, db.First(&row, undated.ID).Error)
	require.NotNil(t, row.UpdateUserID)
	assert.Equal(t, uint(batchUserID), *row.UpdateUserID)
}

// Without the system_batch user the job stops before changing anything.
func TestActivateScheduledClassesNoBatchUser(t *testing.T) {
	db := testutil.NewDB(t)
	cohort := createCohort(t, db, "scheduled", models.Scheduled)

	err := activateScheduledClasses(context.Background(), database.NewDB(db))

	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
	var got models.ProgramClassCohort
	require.NoError(t, db.First(&got, cohort.ID).Error)
	assert.Equal(t, models.Scheduled, got.Status)
}

func TestSystemBatchUserID(t *testing.T) {
	db := testutil.NewDB(t)
	require.NoError(t, db.Create(&models.User{Username: "someone", Email: "a@x"}).Error)
	batch := models.User{Username: "system_batch", Email: "b@x"}
	require.NoError(t, db.Create(&batch).Error)

	id, err := database.NewDB(db).SystemBatchUserID(context.Background())
	require.NoError(t, err)
	assert.Equal(t, batch.ID, id)
}
