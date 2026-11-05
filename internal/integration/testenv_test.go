// Package integration has the tests that run Programs' routes the way core's
// tests/integration ran them through core's server. They moved here with the
// routes, with only the changes running them here needs.
package integration

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/UnlockedLabs/unlocked-contracts/programs"
	"github.com/UnlockedLabs/unlocked-contracts/users"
	"github.com/UnlockedLabs/unlocked-programs/internal/database"
	"github.com/UnlockedLabs/unlocked-programs/internal/handlers"
	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"github.com/UnlockedLabs/unlocked-programs/internal/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

var env *TestEnv

func TestMain(m *testing.M) {
	logrus.SetOutput(io.Discard)
	// Run in UTC, as CI does, so results that depend on the server's local
	// time zone are the same on every machine.
	time.Local = time.UTC
	os.Exit(m.Run())
}

// TestEnv is Programs' routes on an empty test database, with what core's
// test database started with: its SuperAdmin (user 1) at its Default facility
// (1).
type TestEnv struct {
	TestServer *httptest.Server
	DB         *database.DB
	// Programs is the same database, for the tests that called core's
	// Programs module's queries.
	Programs *database.DB
	Client   *Client
	// Core asks Programs' internal API, as core does.
	Core    *programs.Client
	Context context.Context
	Cancel  context.CancelFunc
	T       *testing.T
}

// coreToken is the secret core asks Programs' questions with in the tests.
const coreToken = "core-token"

func SetupTestEnv(t *testing.T) *TestEnv {
	ctx, cancel := context.WithCancel(context.Background())
	db := database.NewDB(testutil.NewDB(t))
	testutil.SeedCore(t, db.DB)

	app, err := handlers.NewHandler(handlers.Deps{
		DB:                db,
		ProviderPlatforms: testutil.StartFakeProviderPlatforms(t, db.DB),
		Features:          testutil.FeaturesOn{},
		Token:             coreToken,
	}, testutil.Login)
	require.NoError(t, err)
	ts := httptest.NewServer(app)

	return &TestEnv{
		TestServer: ts,
		DB:         db,
		Programs:   db,
		Client:     NewClient(ts.URL),
		Core:       programs.NewClient(ts.URL, coreToken, nil),
		Context:    ctx,
		Cancel:     cancel,
		T:          t,
	}
}

func (env *TestEnv) CleanupTestEnv() {
	env.TestServer.Close()
	env.Cancel()
}

func (env *TestEnv) CreateTestUser(username string, role users.Role, facilityId uint, residentId string) (*models.User, error) {
	user := &models.User{
		Username:   username,
		NameFirst:  "Test",
		NameLast:   "User",
		Role:       role,
		Email:      username + "@example.com",
		DocID:      models.NilIfBlank(residentId),
		FacilityID: facilityId,
	}

	if err := env.DB.Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func (env *TestEnv) CreateTestInstructor(facilityID uint, usernamePrefix string) (*models.User, error) {
	// Use no separator to comply with alphanumunicode validation (no underscores or hyphens allowed)
	username := fmt.Sprintf("instructor%s", usernamePrefix)
	return env.CreateTestUser(username, users.FacilityAdmin, facilityID, "")
}

func (env *TestEnv) CreateTestFacility(name string) (*models.Facility, error) {

	facility := &models.Facility{
		Timezone: "America/Chicago",
		Name:     name,
	}

	if err := env.DB.Create(facility).Error; err != nil {
		return nil, err
	}

	return facility, nil
}

func (env *TestEnv) CreateTestProgram(name string, fundingType models.FundingType, programTypes []models.ProgramType, creditTypes []models.ProgramCreditType, isActive bool, archivedAt *time.Time) (*models.Program, error) {
	program := &models.Program{
		Name:               name,
		Description:        "This is a test program created for integration testing purposes.",
		FundingType:        fundingType,
		ProgramTypes:       programTypes,
		ProgramCreditTypes: creditTypes,
		IsActive:           isActive,
		ArchivedAt:         archivedAt,
	}

	if err := env.Programs.CreateProgram(program); err != nil {
		return nil, err
	}
	return program, nil
}

func (env *TestEnv) SetFacilitiesToProgram(programId uint, facilityIds []uint) error {
	if err := env.DB.Where("program_id = ?", programId).Delete(&models.FacilitiesPrograms{}).Error; err != nil {
		return err
	}

	facilityPrograms := models.IterMap(func(id uint) models.FacilitiesPrograms {
		return models.FacilitiesPrograms{
			FacilityID: id,
			ProgramID:  programId,
		}
	}, facilityIds)

	if len(facilityPrograms) > 0 {
		if err := env.DB.Model(&models.FacilitiesPrograms{}).Create(&facilityPrograms).Error; err != nil {
			return err
		}
	}

	return nil
}

func (env *TestEnv) CreateTestClass(program *models.Program, facility *models.Facility, status models.ClassStatus, instructorID *uint) (*models.ProgramClassCohort, error) {
	endDt := time.Now().Add(time.Hour * 24)
	creditHours := int64(2)

	class := &models.ProgramClassCohort{
		ProgramID:    program.ID,
		FacilityID:   facility.ID,
		Capacity:     10,
		InstructorID: instructorID,
		Description:  "This is a test class created for integration testing purposes.",
		StartDt:      time.Now(),
		EndDt:        &endDt,
		Status:       status,
		CreditHours:  &creditHours,
	}

	if err := env.DB.Create(class).Error; err != nil {
		return nil, err
	}

	return class, nil
}

func (env *TestEnv) NameCohortClass(cohortID uint, name string) error {
	var classID uint
	if err := env.DB.Model(&models.ProgramClassCohort{}).
		Select("class_id").
		Where("id = ?", cohortID).
		Scan(&classID).Error; err != nil {
		return err
	}
	return env.DB.Model(&models.ProgramClass{}).
		Where("id = ?", classID).
		Update("name", name).Error
}

func (env *TestEnv) CreateTestEnrollment(classID, userID uint, status models.ProgramEnrollmentStatus) (*models.ProgramClassEnrollment, error) {
	enrollment := &models.ProgramClassEnrollment{
		CohortID:         classID,
		UserID:           userID,
		EnrollmentStatus: status,
	}

	if err := env.DB.Create(enrollment).Error; err != nil {
		return nil, err
	}

	return enrollment, nil
}

func (env *TestEnv) GetEnrollmentTimestamps(enrollmentID uint) (enrolledAt, endedAt *time.Time, err error) {
	var enrollment models.ProgramClassEnrollment
	if err := env.DB.First(&enrollment, "id = ?", enrollmentID).Error; err != nil {
		return nil, nil, err
	}
	return enrollment.EnrolledAt, enrollment.EnrollmentEndedAt, nil
}

func (env *TestEnv) CreateTestEvent(classID uint, rrule string, instructorID uint) (*models.ProgramClassEvent, error) {
	if rrule == "" {
		startDate := time.Now().AddDate(0, 0, -2) // 2 days ago
		rrule = fmt.Sprintf("DTSTART:%s\nRRULE:FREQ=DAILY;COUNT=10", startDate.Format("20060102T090000Z"))
	}

	roomID, err := env.getOrCreateTestRoom(classID)
	if err != nil {
		return nil, err
	}
	event := &models.ProgramClassEvent{
		CohortID:       classID,
		Duration:       "2h",
		RecurrenceRule: rrule,
		RoomID:         &roomID,
		InstructorID:   &instructorID,
	}

	if err := env.DB.Create(event).Error; err != nil {
		return nil, err
	}

	return event, nil
}

func (env *TestEnv) GetEventRecurrenceRule(eventID uint) (string, error) {
	var event models.ProgramClassEvent
	if err := env.DB.First(&event, "id = ?", eventID).Error; err != nil {
		return "", err
	}
	return event.RecurrenceRule, nil
}

func (env *TestEnv) CreateTestEventWithRRule(classID uint, customRRule string, instructorID uint) (*models.ProgramClassEvent, error) {
	roomID, err := env.getOrCreateTestRoom(classID)
	if err != nil {
		return nil, err
	}
	event := &models.ProgramClassEvent{
		CohortID:       classID,
		Duration:       "2h",
		RecurrenceRule: customRRule,
		RoomID:         &roomID,
		InstructorID:   &instructorID,
	}
	if err := env.DB.Create(event).Error; err != nil {
		return nil, err
	}
	return event, nil
}

func (env *TestEnv) CreateTestEventOverride(eventID uint, date string, isCancelled bool, reason string) (*models.ProgramClassEventOverride, error) {
	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, err
	}

	rrule := fmt.Sprintf("DTSTART:%s\nRRULE:FREQ=DAILY;COUNT=1", parsedDate.Format("20060102T090000Z"))

	override := &models.ProgramClassEventOverride{
		EventID:       eventID,
		Duration:      "2h",
		OverrideRrule: rrule,
		IsCancelled:   isCancelled,
		Reason:        models.NilIfBlank(reason),
	}

	if err := env.DB.Create(override).Error; err != nil {
		return nil, err
	}

	return override, nil
}

func (env *TestEnv) CreateTestEnrollmentWithDates(classID, userID uint, status models.ProgramEnrollmentStatus, enrolledAt time.Time, endedAt *time.Time) (*models.ProgramClassEnrollment, error) {
	enrollment := &models.ProgramClassEnrollment{
		CohortID:          classID,
		UserID:            userID,
		EnrollmentStatus:  status,
		EnrolledAt:        &enrolledAt,
		EnrollmentEndedAt: endedAt,
	}

	if err := env.DB.Create(enrollment).Error; err != nil {
		return nil, err
	}

	return enrollment, nil
}

func (env *TestEnv) getOrCreateInstructorForClass(classID uint) (uint, error) {
	var class models.ProgramClassCohort
	if err := env.DB.First(&class, "id = ?", classID).Error; err != nil {
		return 0, err
	}
	var instructor models.User
	result := env.DB.Where("facility_id = ? AND role = ?", class.FacilityID, users.FacilityAdmin).
		Order("id ASC").First(&instructor)
	if result.Error == nil {
		return instructor.ID, nil
	}
	created, err := env.CreateTestInstructor(class.FacilityID, fmt.Sprintf("auto%d", time.Now().UnixNano()))
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (env *TestEnv) getOrCreateTestRoom(classID uint) (uint, error) {
	var class models.ProgramClassCohort
	if err := env.DB.First(&class, "id = ?", classID).Error; err != nil {
		return 0, err
	}
	var room models.Room
	result := env.DB.Where("facility_id = ? AND name = ?", class.FacilityID, "Test Room").First(&room)
	if result.Error == nil {
		return room.ID, nil
	}
	room = models.Room{
		FacilityID: class.FacilityID,
		Name:       "Test Room",
	}
	if err := env.DB.Create(&room).Error; err != nil {
		return 0, err
	}
	return room.ID, nil
}
