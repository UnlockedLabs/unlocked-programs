package handlers

import (
	"context"
	"time"

	"github.com/UnlockedLabs/unlocked-contracts/coreapi"
	api "github.com/UnlockedLabs/unlocked-contracts/programs"
	"github.com/UnlockedLabs/unlocked-programs/internal/database"
	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"gorm.io/gorm"
)

// programsAPI answers core's questions (api.Programs): the queries from the
// database, and the live provider data from the Canvas and Essential
// Education code.
type programsAPI struct {
	store *database.DB
	liveProviders
}

// newAPI is Programs' answers to core, from srv's database and providers.
func newAPI(srv *server) api.Programs {
	return programsAPI{store: srv.store, liveProviders: liveProviders{srv}}
}

func (a programsAPI) EnrolledUserIDs(ctx context.Context, classID uint) ([]uint, error) {
	ids, err := a.store.GetEnrolledResidentIDsForClass(ctx, int(classID))
	return ids, err
}

func (a programsAPI) EligibleUserIDs(ctx context.Context, classID uint) ([]uint, error) {
	ids, err := a.store.GetEligibleResidentIDsForClass(ctx, int(classID))
	return ids, err
}

func (a programsAPI) TransferConflicts(ctx context.Context, userID, toFacilityID uint) ([]api.ResidentTransferProgramConflicts, error) {
	return a.store.GetTransferProgramConflicts(ctx, userID, int(toFacilityID))
}

func (a programsAPI) EndEnrollmentsForTransfer(ctx context.Context, userID, fromFacilityID uint) error {
	return a.store.EndEnrollmentsForTransfer(ctx, userID, fromFacilityID)
}

func (a programsAPI) WithdrawEnrollments(ctx context.Context, userID uint) error {
	return a.store.WithdrawEnrollments(ctx, userID)
}

func (a programsAPI) ResidentPrograms(ctx context.Context, userID uint) ([]api.ResidentProgramClassInfo, error) {
	// The usage report has always listed every enrollment, latest class start
	// first.
	args := models.QueryContext{Ctx: ctx, All: true, OrderBy: "start_dt", Order: "DESC"}
	rows, err := a.store.GetUserProgramInfo(&args, int(userID))
	if err != nil {
		return nil, err
	}
	out := make([]api.ResidentProgramClassInfo, len(rows))
	for i := range rows {
		rows[i].CalculateAttendancePercentage()
		out[i] = toAPIResidentProgram(rows[i])
	}
	return out, nil
}

func toAPIResidentProgram(p models.ResidentProgramClassInfo) api.ResidentProgramClassInfo {
	return api.ResidentProgramClassInfo{
		ProgramName:          p.ProgramName,
		ClassName:            p.ClassName,
		Status:               string(p.Status),
		EnrollmentStatus:     string(p.EnrollmentStatus),
		StartDate:            p.StartDate,
		EndDate:              p.EndDate,
		AttendancePercentage: p.AttendancePercentage,
		CohortID:             p.CohortID,
		ProgramID:            p.ProgramID,
		EnrollmentID:         p.EnrollmentID,
		PresentAttendance:    p.PresentAttendance,
		AbsentAttendance:     p.AbsentAttendance,
		CreditTypes:          p.CreditTypes,
		UpdatedAt:            p.UpdatedAt,
		ChangeReason:         p.ChangeReason,
		Schedule:             p.Schedule,
		IsCanvas:             p.IsCanvas,
		Source:               p.Source,
	}
}

// FacilityPrograms counts with the conditions core's facility list and
// delete check used before they asked Programs.
func (a programsAPI) FacilityPrograms(ctx context.Context, facilityIDs []uint) (map[uint]api.FacilityPrograms, error) {
	counts := make(map[uint]api.FacilityPrograms)
	if len(facilityIDs) == 0 {
		return counts, nil
	}
	db := a.store.WithContext(ctx)
	queries := []struct {
		query    *gorm.DB
		facility string
		set      func(*api.FacilityPrograms, int64)
	}{
		{db.Table("facilities_programs fp").
			Select("fp.facility_id, COUNT(DISTINCT p.id) AS count").
			Joins("JOIN programs p ON p.id = fp.program_id AND p.deleted_at IS NULL").
			Where("fp.deleted_at IS NULL AND p.is_active = true AND p.archived_at IS NULL"),
			"fp.facility_id", func(c *api.FacilityPrograms, n int64) { c.ActivePrograms = n }},
		{db.Table("program_class_cohorts").
			Select("facility_id, COUNT(*) AS count").
			Where("deleted_at IS NULL AND status = ? AND archived_at IS NULL", models.Active),
			"facility_id", func(c *api.FacilityPrograms, n int64) { c.ActiveClasses = n }},
		{db.Table("program_classes").
			Select("facility_id, COUNT(*) AS count").
			Where("deleted_at IS NULL"),
			"facility_id", func(c *api.FacilityPrograms, n int64) { c.Classes = n }},
		{db.Table("facilities_programs fp").
			Select("fp.facility_id, COUNT(*) AS count").
			Where("fp.deleted_at IS NULL"),
			"fp.facility_id", func(c *api.FacilityPrograms, n int64) { c.Programs = n }},
		{db.Table("rooms").
			Select("facility_id, COUNT(*) AS count").
			Where("deleted_at IS NULL"),
			"facility_id", func(c *api.FacilityPrograms, n int64) { c.Rooms = n }},
	}
	for _, q := range queries {
		var rows []struct {
			FacilityID uint
			Count      int64
		}
		if err := q.query.Where(q.facility+" IN ?", facilityIDs).
			Group(q.facility).
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			c := counts[row.FacilityID]
			q.set(&c, row.Count)
			counts[row.FacilityID] = c
		}
	}
	return counts, nil
}

func (a programsAPI) ClassHistoryCreatedAt(ctx context.Context, ids []uint) (map[uint]time.Time, error) {
	times := make(map[uint]time.Time)
	if len(ids) == 0 {
		return times, nil
	}
	var rows []struct {
		ID        uint
		CreatedAt time.Time
	}
	if err := a.store.WithContext(ctx).Table("program_classes_history").
		Select("id, created_at").
		Where("id IN ? AND created_at IS NOT NULL", ids).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		times[row.ID] = row.CreatedAt
	}
	return times, nil
}

func (a programsAPI) EngagementOverview(ctx context.Context, start, end *time.Time, facilityID *uint) (api.ProgramEngagementOverview, error) {
	overview, err := a.store.GetProgramEngagementOverview(&models.QueryContext{Ctx: ctx}, start, end, facilityID)
	return overview, err
}

func (a programsAPI) SecondProgramEnrollment(ctx context.Context, facilityID *uint) ([]api.SecondProgramEnrollmentRow, error) {
	rows, err := a.store.GetSecondProgramEnrollmentRates(&models.QueryContext{Ctx: ctx}, facilityID)
	return rows, err
}

func (a programsAPI) CompletionMatrix(ctx context.Context, facilityID *uint) ([]api.ProgramCompletionMatrixCell, error) {
	cells, err := a.store.GetProgramCompletionMatrix(&models.QueryContext{Ctx: ctx}, facilityID)
	return cells, err
}

func (a programsAPI) LoadDistribution(ctx context.Context, facilityID *uint) (api.ProgramLoadDistribution, error) {
	dist, err := a.store.GetProgramLoadDistribution(&models.QueryContext{Ctx: ctx}, facilityID)
	return dist, err
}

func (a programsAPI) EnrollmentByProgramType(ctx context.Context, facilityID *uint) ([]api.ProgramTypeEnrollment, error) {
	rows, err := a.store.GetEnrollmentByProgramType(&models.QueryContext{Ctx: ctx}, facilityID)
	return rows, err
}

func (a programsAPI) ClassMetrics(ctx context.Context, facilityID *uint) (api.ClassDashboardMetrics, error) {
	metrics, err := a.store.GetClassDashboardMetrics(&models.QueryContext{Ctx: ctx}, facilityID)
	return metrics, err
}

func (a programsAPI) FacilityHealth(ctx context.Context, facilityID *uint, days int, tz string) ([]api.FacilityHealthSummary, error) {
	summaries, err := NewClassesService(a.store).GetFacilityHealthSummaries(&models.QueryContext{Ctx: ctx, Timezone: tz}, facilityID, days)
	return summaries, err
}

// liveProviders answers Provider Platforms' questions about Essential
// Education's users and Canvas's cached programs.
type liveProviders struct{ srv *server }

func (l liveProviders) LiveUsers(ctx context.Context, providerID uint) ([]coreapi.ProviderUser, bool, error) {
	provider, err := l.srv.providerByID(ctx, providerID)
	if err != nil {
		return nil, false, err
	}
	lister, ok := l.srv.liveUserListerFor(provider)
	if !ok {
		return nil, false, nil
	}
	users, err := lister.ListUsers(ctx)
	if err != nil {
		return nil, true, err
	}
	return users, true, nil
}

func (l liveProviders) MappingsChanged(providerID, userID uint) {
	l.srv.invalidateCanvasProgramCache(providerID, int(userID))
}
