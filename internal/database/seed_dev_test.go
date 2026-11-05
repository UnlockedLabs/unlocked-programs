package database

import (
	"testing"
	"time"

	"github.com/UnlockedLabs/unlocked-contracts/users"
	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"github.com/UnlockedLabs/unlocked-programs/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The sample data is for core's residents only, each at their own facility,
// and every row points at rows that exist.
func TestSeedDev(t *testing.T) {
	db := testutil.NewDB(t)
	// The sample sessions have no instructor, which Postgres allows: the
	// model's not-null tag would make SQLite refuse them, so the table is
	// Postgres's.
	require.NoError(t, db.Migrator().DropTable(&models.ProgramClassEvent{}))
	require.NoError(t, db.Exec(`CREATE TABLE program_class_events (
		id INTEGER PRIMARY KEY, cohort_id INTEGER NOT NULL, duration TEXT NOT NULL, recurrence_rule TEXT NOT NULL,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME, create_user_id INTEGER, update_user_id INTEGER,
		room_id INTEGER NOT NULL, instructor_id INTEGER, reason TEXT, is_cancelled BOOLEAN NOT NULL DEFAULT false)`).Error)
	count := func(model any) int64 {
		t.Helper()
		var n int64
		require.NoError(t, db.Model(model).Count(&n).Error)
		return n
	}

	require.ErrorContains(t, seedDev(t.Context(), db, time.Now()), "run core's seeder first")

	facilities := []models.Facility{{Name: "BCF", Timezone: "America/New_York"}, {Name: "Potosi", Timezone: "America/Chicago"}}
	require.NoError(t, db.Create(&facilities).Error)
	people := []models.User{
		{Username: "admin", Email: "admin@x", Role: users.SystemAdmin, FacilityID: facilities[0].ID},
		{Username: "jdoe", Email: "jdoe@x", Role: users.Student, FacilityID: facilities[0].ID},
		{Username: "asmith", Email: "asmith@x", Role: users.Student, FacilityID: facilities[1].ID},
		{Username: "gone", Email: "gone@x", Role: users.Student, FacilityID: facilities[1].ID},
		{Username: "system_batch", Email: "batch@x", Role: users.SystemAdmin, FacilityID: facilities[0].ID},
	}
	require.NoError(t, db.Create(&people).Error)
	require.NoError(t, db.Delete(&people[3]).Error)
	residentFacility := map[uint]uint{people[1].ID: facilities[0].ID, people[2].ID: facilities[1].ID}
	batchUserID := people[4].ID

	// A failure partway through adds nothing.
	require.NoError(t, db.Migrator().RenameTable("program_class_events", "events_away"))
	require.Error(t, seedDev(t.Context(), db, time.Now()))
	assert.Zero(t, count(&models.Room{}), "a failed run adds nothing")
	assert.Zero(t, count(&models.Program{}), "a failed run adds nothing")
	require.NoError(t, db.Migrator().RenameTable("events_away", "program_class_events"))

	now := time.Now()
	require.NoError(t, seedDev(t.Context(), db, now))

	assert.Equal(t, int64(16), count(&models.Room{}), "8 rooms per facility")
	var programs []models.Program
	require.NoError(t, db.Find(&programs).Error)
	assert.Len(t, programs, 7)
	names := map[string]bool{}
	for _, p := range programs {
		assert.False(t, names[p.Name], "program %q once", p.Name)
		names[p.Name] = true
		require.NotNil(t, p.CreateUserID)
		assert.Equal(t, batchUserID, *p.CreateUserID, "made by system_batch")
	}
	assert.Equal(t, int64(14), count(&models.FacilitiesPrograms{}), "every program at every facility")

	var cohorts []models.ProgramClassCohort
	require.NoError(t, db.Find(&cohorts).Error)
	cohortsByID := make(map[uint]models.ProgramClassCohort, len(cohorts))
	for _, c := range cohorts {
		cohortsByID[c.ID] = c
		assert.Equal(t, models.Scheduled, c.Status)
		require.NotNil(t, c.EndDt)
		assert.True(t, c.EndDt.After(c.StartDt), "cohort %d ends after it starts", c.ID)
		var class models.ProgramClass
		require.NoError(t, db.First(&class, c.ClassID).Error)
		assert.Equal(t, c.FacilityID, class.FacilityID, "cohort %d is at its class's facility", c.ID)
	}

	var enrollments []models.ProgramClassEnrollment
	require.NoError(t, db.Find(&enrollments).Error)
	classesPerUser := map[uint]map[uint]bool{}
	completed := 0
	for _, e := range enrollments {
		facilityID, isResident := residentFacility[e.UserID]
		require.True(t, isResident, "enrollment %d is for a resident", e.ID)
		cohort := cohortsByID[e.CohortID]
		assert.Equal(t, facilityID, cohort.FacilityID, "enrollment %d is at the resident's facility", e.ID)
		assert.Equal(t, cohort.ClassID, e.ClassID, "enrollment %d has its cohort's class", e.ID)
		if classesPerUser[e.UserID] == nil {
			classesPerUser[e.UserID] = map[uint]bool{}
		}
		assert.False(t, classesPerUser[e.UserID][e.ClassID], "user %d is in class %d once", e.UserID, e.ClassID)
		classesPerUser[e.UserID][e.ClassID] = true
		if e.EnrollmentStatus == models.EnrollmentCompleted {
			completed++
		}
	}
	for userID, classes := range classesPerUser {
		assert.LessOrEqual(t, len(classes), 3, "user %d has at most 3 enrollments", userID)
	}

	var completions []models.ClassCompletion
	require.NoError(t, db.Find(&completions).Error)
	assert.Len(t, completions, completed, "one completion per completed enrollment")
	facilityNames := map[uint]string{facilities[0].ID: "BCF", facilities[1].ID: "Potosi"}
	for _, c := range completions {
		assert.Equal(t, "admin@x", c.AdminEmail)
		require.NotNil(t, c.CohortID)
		assert.Equal(t, cohortsByID[*c.CohortID].ClassID, *c.ClassID)
		assert.Equal(t, facilityNames[residentFacility[c.UserID]], c.FacilityName)
		assert.NotEmpty(t, c.ProgramName)
		// The class's own credit type, else its program's.
		var own, inherited []string
		require.NoError(t, db.Model(&models.ProgramClassCreditType{}).Where("class_id = ?", *c.ClassID).Pluck("credit_type", &own).Error)
		require.NoError(t, db.Model(&models.ProgramCreditType{}).Where("program_id = ?", *c.ProgramID).Pluck("credit_type", &inherited).Error)
		want := append(own, inherited...)[0]
		assert.Equal(t, want, c.CreditType)
	}

	var attendance []models.ProgramClassEventAttendance
	require.NoError(t, db.Find(&attendance).Error)
	for _, a := range attendance {
		_, isResident := residentFacility[a.UserID]
		assert.True(t, isResident, "attendance %d is for a resident", a.ID)
	}

}
