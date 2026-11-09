package database

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/UnlockedLabs/unlocked-contracts/identity"
	"github.com/UnlockedLabs/unlocked-contracts/users"
	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	log "github.com/sirupsen/logrus"
	"github.com/teambition/rrule-go"
	"gorm.io/gorm"
)

// SeedDev fills a local database with sample Programs data for core's
// facilities and residents: rooms, 7 programs offered at every facility, their
// classes and weekly sessions, and each resident's enrollments, completions
// and attendance. It moved from core's dev seeder (`make seed`), which runs it
// after adding core's facilities and users. A failed run adds nothing.
func (db *DB) SeedDev(ctx context.Context) error {
	if err := seedDev(ctx, db.DB, time.Now()); err != nil {
		return err
	}
	log.Info("Seeded the sample Programs data")
	return nil
}

// sampleProgram is a program the sample data can offer.
type sampleProgram struct {
	name        string
	programType models.ProgType
	description string
}

var samplePrograms = []sampleProgram{
	{"Anger Management", models.Therapeutic, "Techniques to control and express anger constructively."},
	{"Substance Abuse Treatment", models.MentalHealth, "Support and strategies for overcoming addiction."},
	{"AA/NA", models.MentalHealth, "Peer support groups for alcohol and drug recovery."},
	{"Thinking for a Change", models.LifeSkills, "Cognitive-based program for decision-making and behavior change."},
	{"A New Freedom", models.LifeSkills, "Life skills and personal growth program for better choices."},
	{"Dog Training", models.Vocational, "Hands-on program teaching dog training and care skills."},
	{"A New Path", models.Therapeutic, "Therapeutic program focused on personal healing and growth."},
	{"GED/Hi-SET", models.Educational, "Education program to earn a high school equivalency diploma."},
	{"Parenting", models.LifeSkills, "Guidance and skills for effective parenting strategies."},
	{"Employment", models.Vocational, "Job readiness, resume building, and interview preparation."},
	{"Life Skills", models.LifeSkills, "Essential skills for daily living and self-sufficiency."},
	{"Health and Wellness", models.MentalHealth, "Education on physical and mental well-being practices."},
	{"Financial Literacy", models.Educational, "Budgeting, saving, and money management fundamentals."},
	{"Computer Skills", models.Educational, "Basic to advanced computer literacy and digital skills."},
}

var (
	sampleRoomNames     = []string{"Room A", "Room B", "Classroom 1", "Classroom 2", "Computer Lab", "Library", "Auditorium", "Conference Room"}
	sampleProgramOwners = []string{"Maria Lopez", "James Carter", "Aisha Brown", "Daniel Kim", "Grace Okafor", "Tom Nguyen"}
	sampleFundingTypes  = []models.FundingType{models.EduGrants, models.FederalGrants, models.InmateWelfare, models.NonProfitOrgs, models.Other, models.StateGrants}
	sampleCreditTypes   = []models.CreditType{models.Completion, models.EarnedTime, models.Education, models.Participation}
	sampleCreditHours   = []int64{5, 10, 15, 20}
	sampleCapacities    = []int64{15, 25, 30, 35, 40, 45}
	// An enrollment that isn't Enrolled has one of these.
	sampleEndedStatuses = []models.ProgramEnrollmentStatus{
		models.EnrollmentCancelled,
		models.EnrollmentCompleted,
		models.EnrollmentIncompleteWithdrawn,
		models.EnrollmentIncompleteDropped,
		models.EnrollmentIncompleteFailedToComplete,
		models.EnrollmentIncompleteTransferred,
	}
	sampleAttendance = []models.Attendance{models.Present, models.Absent_Excused, models.Absent_Unexcused}
)

// devUser is a core user the sample data is for.
type devUser struct {
	ID         uint
	FacilityID uint
}

// seedDev adds the sample data in one transaction, so a failed run adds
// nothing.
func seedDev(ctx context.Context, db *gorm.DB, now time.Time) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var facilities []models.Facility
		if err := tx.Order("id").Find(&facilities).Error; err != nil {
			return fmt.Errorf("find facilities: %w", err)
		}
		var residents []devUser
		if err := tx.Table("users").Select("id, facility_id").Where("role = ? AND deleted_at IS NULL", users.Student).Scan(&residents).Error; err != nil {
			return fmt.Errorf("find residents: %w", err)
		}
		if len(facilities) == 0 || len(residents) == 0 {
			return errors.New("no facilities or residents: run core's seeder first")
		}
		var batchUserID uint
		if err := tx.Table("users").Select("id").Where("username = ?", "system_batch").Limit(1).Scan(&batchUserID).Error; err != nil {
			return fmt.Errorf("find the system_batch user: %w", err)
		}
		if batchUserID != 0 {
			tx = tx.WithContext(identity.NewContext(ctx, &identity.Claims{UserID: batchUserID}))
		}
		// class_completions.admin_email is required and every certificate
		// screen shows it, so use a real admin's.
		adminEmail := "admin@unlocked.test"
		var firstAdminEmail string
		if err := tx.Table("users").Select("email").Where("role IN ? AND deleted_at IS NULL", users.AdminRoles).Order("id").Limit(1).Scan(&firstAdminEmail).Error; err != nil {
			return fmt.Errorf("find an admin: %w", err)
		}
		if firstAdminEmail != "" {
			adminEmail = firstAdminEmail
		}

		cohorts, err := seedClasses(tx, facilities, now)
		if err != nil {
			return err
		}
		var cohortIDs []uint
		for _, c := range cohorts {
			cohortIDs = append(cohortIDs, c.ID)
		}
		var events []models.ProgramClassEvent
		if err := tx.Where("cohort_id IN ?", cohortIDs).Find(&events).Error; err != nil {
			return fmt.Errorf("find sessions: %w", err)
		}
		facilityNames := make(map[uint]string, len(facilities))
		for _, f := range facilities {
			facilityNames[f.ID] = f.Name
		}
		for _, user := range residents {
			if err := seedEnrollments(tx, user, cohorts, facilityNames[user.FacilityID], adminEmail); err != nil {
				return err
			}
			if err := seedAttendance(tx, user, events, now); err != nil {
				return err
			}
		}
		return nil
	})
}

// seedClasses adds the rooms, then 7 different programs offered at every
// facility, each with a program type, a credit type and up to 3 classes per
// facility, 1 to 3 cohorts per class, and the cohorts' weekly sessions. Every
// cohort is Scheduled, starting two weeks or more from now and running 20 to
// 35 days.
func seedClasses(tx *gorm.DB, facilities []models.Facility, now time.Time) ([]models.ProgramClassCohort, error) {
	rooms := make(map[uint][]models.Room, len(facilities))
	for _, f := range facilities {
		for _, name := range sampleRoomNames {
			rooms[f.ID] = append(rooms[f.ID], models.Room{FacilityID: f.ID, Name: name})
		}
		if err := tx.Create(rooms[f.ID]).Error; err != nil {
			return nil, fmt.Errorf("create rooms for facility %d: %w", f.ID, err)
		}
	}
	var cohorts []models.ProgramClassCohort
	for _, i := range rand.Perm(len(samplePrograms))[:7] {
		sample := samplePrograms[i]
		program := models.Program{
			Name:        sample.name,
			Description: sample.description,
			FundingType: sampleFundingTypes[rand.Intn(len(sampleFundingTypes))],
			IsActive:    true,
		}
		if err := tx.Create(&program).Error; err != nil {
			return nil, fmt.Errorf("create program %s: %w", program.Name, err)
		}
		programType := models.ProgramType{ProgramType: sample.programType, ProgramID: program.ID}
		if err := tx.Create(&programType).Error; err != nil {
			return nil, fmt.Errorf("create program type for program %d: %w", program.ID, err)
		}
		creditType := models.ProgramCreditType{CreditType: sampleCreditTypes[rand.Intn(len(sampleCreditTypes))], ProgramID: program.ID}
		if err := tx.Create(&creditType).Error; err != nil {
			return nil, fmt.Errorf("create credit type for program %d: %w", program.ID, err)
		}
		for _, f := range facilities {
			offered := models.FacilitiesPrograms{
				ProgramID:    program.ID,
				FacilityID:   f.ID,
				ProgramOwner: models.StringPtr(sampleProgramOwners[rand.Intn(len(sampleProgramOwners))]),
			}
			if err := tx.Create(&offered).Error; err != nil {
				return nil, fmt.Errorf("offer program %d at facility %d: %w", program.ID, f.ID, err)
			}
			for range rand.Intn(4) {
				classCohorts, err := seedClass(tx, program, sample, f.ID, rooms[f.ID], now)
				if err != nil {
					return nil, err
				}
				cohorts = append(cohorts, classCohorts...)
			}
		}
	}
	return cohorts, nil
}

// seedClass adds one class of program at a facility, with 1 to 3 cohorts and
// their sessions. A third of the classes get their own credit type, so both
// the inherited and the overriding credit type show up.
func seedClass(tx *gorm.DB, program models.Program, sample sampleProgram, facilityID uint, rooms []models.Room, now time.Time) ([]models.ProgramClassCohort, error) {
	class := models.ProgramClass{
		Name:        program.Name,
		Description: models.NilIfBlank(sample.description),
		CreditHours: &sampleCreditHours[rand.Intn(len(sampleCreditHours))],
		FacilityID:  facilityID,
		ProgramID:   program.ID,
	}
	if err := tx.Create(&class).Error; err != nil {
		return nil, fmt.Errorf("create class for program %d: %w", program.ID, err)
	}
	if rand.Intn(3) == 0 {
		own := models.ProgramClassCreditType{ClassID: class.ID, CreditType: sampleCreditTypes[rand.Intn(len(sampleCreditTypes))]}
		if err := tx.Create(&own).Error; err != nil {
			return nil, fmt.Errorf("create credit type for class %d: %w", class.ID, err)
		}
	}
	var cohorts []models.ProgramClassCohort
	for c := range rand.Intn(3) + 1 {
		startDt := now.AddDate(0, 0, 14+7*c)
		endDt := startDt.AddDate(0, 0, 20+5*rand.Intn(4))
		cohort := models.ProgramClassCohort{
			ClassID:     class.ID,
			Capacity:    sampleCapacities[rand.Intn(len(sampleCapacities))],
			Description: sample.description,
			Status:      models.Scheduled,
			StartDt:     startDt,
			EndDt:       &endDt,
			FacilityID:  facilityID,
			ProgramID:   program.ID,
		}
		if err := tx.Create(&cohort).Error; err != nil {
			return nil, fmt.Errorf("create cohort for class %d: %w", class.ID, err)
		}
		// ClassName is read-only in the table; it carries the name to the
		// completions.
		cohort.ClassName = class.Name
		if err := seedSessions(tx, cohort, rooms); err != nil {
			return nil, err
		}
		cohorts = append(cohorts, cohort)
	}
	return cohorts, nil
}

// seedSessions adds a cohort's weekly sessions, on 1 to 3 different weekdays
// in one of the facility's rooms.
func seedSessions(tx *gorm.DB, cohort models.ProgramClassCohort, rooms []models.Room) error {
	days := []rrule.Weekday{rrule.MO, rrule.TU, rrule.WE, rrule.TH, rrule.FR, rrule.SA, rrule.SU}
	// Different weekdays: the same one twice would make the rule repeat each
	// session, and the attendance with it.
	rand.Shuffle(len(days), func(i, j int) { days[i], days[j] = days[j], days[i] })
	rule, err := rrule.NewRRule(rrule.ROption{
		Freq:      rrule.WEEKLY,
		Dtstart:   cohort.StartDt,
		Until:     *cohort.EndDt,
		Byweekday: days[:rand.Intn(3)+1],
	})
	if err != nil {
		return fmt.Errorf("make the schedule for cohort %d: %w", cohort.ID, err)
	}
	roomID := rooms[rand.Intn(len(rooms))].ID
	event := models.ProgramClassEvent{
		CohortID:       cohort.ID,
		RecurrenceRule: rule.String(),
		RoomID:         &roomID,
		Duration:       "1h0m0s",
	}
	if err := tx.Create(&event).Error; err != nil {
		return fmt.Errorf("create sessions for cohort %d: %w", cohort.ID, err)
	}
	return nil
}

// seedEnrollments enrolls a resident in 1 to 3 cohorts of different classes
// at their facility. About half the enrollments have ended, and a completed
// one gets its completion record.
func seedEnrollments(tx *gorm.DB, user devUser, cohorts []models.ProgramClassCohort, facilityName, adminEmail string) error {
	var available []models.ProgramClassCohort
	for _, c := range cohorts {
		if c.FacilityID == user.FacilityID {
			available = append(available, c)
		}
	}
	rand.Shuffle(len(available), func(i, j int) { available[i], available[j] = available[j], available[i] })
	want := rand.Intn(3) + 1
	seenClasses := make(map[uint]bool)
	for _, cohort := range available {
		if len(seenClasses) == want {
			break
		}
		if seenClasses[cohort.ClassID] {
			continue
		}
		seenClasses[cohort.ClassID] = true

		status := models.Enrolled
		if rand.Intn(2) == 0 {
			status = sampleEndedStatuses[rand.Intn(len(sampleEndedStatuses))]
		}
		// The enrollment's create hook sets its class from the cohort.
		enrollment := models.ProgramClassEnrollment{UserID: user.ID, CohortID: cohort.ID, EnrollmentStatus: status}
		if err := tx.Create(&enrollment).Error; err != nil {
			return fmt.Errorf("enroll user %d in cohort %d: %w", user.ID, cohort.ID, err)
		}
		if status != models.EnrollmentCompleted {
			continue
		}
		creditType, err := classCreditType(tx, cohort.ClassID, cohort.ProgramID)
		if err != nil {
			return err
		}
		// The create hook dates only an Active cohort's enrollments, and these
		// are Scheduled, so the completion uses the cohort's start.
		enrolledOn := cohort.StartDt
		if enrollment.EnrolledAt != nil {
			enrolledOn = *enrollment.EnrolledAt
		}
		completion := models.ClassCompletion{
			UserID:        user.ID,
			CohortID:      &cohort.ID,
			ClassID:       &cohort.ClassID,
			ProgramID:     &cohort.ProgramID,
			ProgramName:   cohort.ClassName,
			FacilityName:  facilityName,
			ProgramOwner:  sampleProgramOwners[rand.Intn(len(sampleProgramOwners))],
			ClassName:     cohort.ClassName,
			CreditType:    string(creditType),
			AdminEmail:    adminEmail,
			CohortStartDt: cohort.StartDt,
			EnrolledOnDt:  enrolledOn,
		}
		if err := tx.Create(&completion).Error; err != nil {
			return fmt.Errorf("create completion for user %d in cohort %d: %w", user.ID, cohort.ID, err)
		}
	}
	return nil
}

// classCreditType is the class's own credit type, else its program's, else
// Completion: the rule a real completion follows.
func classCreditType(tx *gorm.DB, classID, programID uint) (models.CreditType, error) {
	var own []models.CreditType
	if err := tx.Model(&models.ProgramClassCreditType{}).Where("class_id = ?", classID).Limit(1).Pluck("credit_type", &own).Error; err != nil {
		return "", fmt.Errorf("find the credit type of class %d: %w", classID, err)
	}
	if len(own) > 0 {
		return own[0], nil
	}
	var inherited []models.CreditType
	if err := tx.Model(&models.ProgramCreditType{}).Where("program_id = ?", programID).Limit(1).Pluck("credit_type", &inherited).Error; err != nil {
		return "", fmt.Errorf("find the credit type of program %d: %w", programID, err)
	}
	if len(inherited) > 0 {
		return inherited[0], nil
	}
	return models.Completion, nil
}

// seedAttendance records a resident at about half the sessions of every
// event, from 90 days ago to 6 months from now, as core's seeder did.
func seedAttendance(tx *gorm.DB, user devUser, events []models.ProgramClassEvent, now time.Time) error {
	var attendance []models.ProgramClassEventAttendance
	for _, event := range events {
		rule, err := rrule.StrToRRule(event.RecurrenceRule)
		if err != nil {
			return fmt.Errorf("read the schedule of event %d: %w", event.ID, err)
		}
		for _, session := range rule.Between(now.AddDate(0, 0, -90), now.AddDate(0, 6, 0), true) {
			if rand.Intn(2) == 0 {
				continue
			}
			attendance = append(attendance, models.ProgramClassEventAttendance{
				EventID:          event.ID,
				UserID:           user.ID,
				Date:             session.Format("2006-01-02"),
				AttendanceStatus: sampleAttendance[rand.Intn(len(sampleAttendance))],
			})
		}
	}
	if len(attendance) == 0 {
		return nil
	}
	if err := tx.CreateInBatches(attendance, 500).Error; err != nil {
		return fmt.Errorf("record attendance for user %d: %w", user.ID, err)
	}
	return nil
}
