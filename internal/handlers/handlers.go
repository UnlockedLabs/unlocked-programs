package handlers

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/UnlockedLabs/unlocked-contracts/features"
	"github.com/UnlockedLabs/unlocked-contracts/identity"
	"github.com/UnlockedLabs/unlocked-contracts/users"
	"github.com/UnlockedLabs/unlocked-programs/internal/models"
)

// routes is every Programs route, with the access checks and features it had
// in core. The five routes that shared a path with core's own routes have
// their own paths here, under /api/program-users and /api/program-facilities,
// so the ingress can send every Programs path to this service.
func (srv *server) routes() []route {
	prog := []features.Access{features.ProgramManagement}
	cohort := srv.inFacility(models.TableNameCohort, "cohort_id")
	class := srv.inFacility(models.TableNameClass, "id")
	return []route{
		// Programs
		{Pattern: "GET /api/programs", Handler: handle(srv.handleIndexPrograms), Features: prog},
		{Pattern: "GET /api/programs/filters", Handler: handle(srv.handleGetProgramFilters), Features: prog},
		{Pattern: "GET /api/programs/{id}", Handler: handle(srv.handleShowProgram), Features: prog},
		{Pattern: "GET /api/programs/detailed-list", Handler: handle(srv.handleIndexProgramsOverviewTable), Access: admin, Features: prog},
		{Pattern: "GET /api/programs/stats", Handler: handle(srv.handleIndexProgramsFacilitiesStats), Access: admin, Features: prog},
		{Pattern: "GET /api/programs/{id}/history", Handler: handle(srv.handleGetProgramHistory), Access: admin, Features: prog},
		{Pattern: "GET /api/programs/{id}/archive-check", Handler: handle(srv.handleGetProgramArchiveCheck), Access: admin, Features: prog},
		{Pattern: "GET /api/programs/{id}/delete-check", Handler: handle(srv.handleGetProgramDeleteCheck), Access: admin, Features: prog},
		{Pattern: "POST /api/programs", Handler: handle(srv.handleCreateProgram), Access: admin, Features: prog},
		{Pattern: "DELETE /api/programs/{id}", Handler: handle(srv.handleDeleteProgram), Access: admin, Features: prog},
		{Pattern: "PATCH /api/programs/{id}/status", Handler: handle(srv.handleUpdateProgramStatus), Access: admin, Features: prog},
		{Pattern: "PATCH /api/programs/{id}", Handler: handle(srv.handleUpdateProgram), Access: admin, Features: prog},
		{Pattern: "GET /api/programs/csv", Handler: handle(srv.handleExportProgramCSV), Access: admin, Allow: deptAdminForAll, Features: prog},

		// Classes
		{Pattern: "GET /api/programs/{id}/classes", Handler: handle(srv.handleGetClassesForProgram), Features: prog},
		{Pattern: "GET /api/program-classes", Handler: handle(srv.handleIndexClassesForFacility), Features: prog},
		{Pattern: "POST /api/programs/{program_id}/classes", Handler: handle(srv.handleCreateClass), Access: admin, Allow: srv.programInFacility("is_active = true AND archived_at IS NULL AND"), Features: prog},
		{Pattern: "GET /api/instructors/{id}/classes", Handler: handle(srv.handleGetClassesByInstructor), Access: admin, UserParam: "id", Features: prog},
		{Pattern: "POST /api/program-classes/bulk-cancel", Handler: handle(srv.handleBulkCancelSessions), Access: admin, Features: prog},
		{Pattern: "GET /api/program-classes/{cohort_id}", Handler: handle(srv.handleGetClass), Allow: srv.classInFacility, Features: prog},
		{Pattern: "GET /api/programs/{program_id}/classes/outcomes", Handler: handle(srv.handleGetProgramClassOutcomes), Access: admin, Allow: srv.programInFacility(""), Features: prog},
		{Pattern: "GET /api/program-classes/{cohort_id}/attendance-flags", Handler: handle(srv.handleGetAttendanceFlagsForClass), Access: admin, Allow: srv.classInFacility, Features: prog},
		{Pattern: "GET /api/program-classes/missing-attendance", Handler: handle(srv.handleGetMissingAttendanceForFacility), Access: admin, Features: prog},
		{Pattern: "GET /api/program-classes/{cohort_id}/missing-attendance", Handler: handle(srv.handleGetMissingAttendance), Access: admin, Allow: srv.classInFacility, Features: prog},
		{Pattern: "GET /api/program-classes/{cohort_id}/attendance-rate", Handler: handle(srv.handleGetCumulativeAttendanceRate), Access: admin, Allow: srv.classInFacility, Features: prog},
		{Pattern: "GET /api/program-classes/{cohort_id}/history", Handler: handle(srv.handleGetClassHistory), Access: admin, Allow: srv.classInFacility, Features: prog},
		{Pattern: "PATCH /api/program-classes", Handler: handle(srv.handleUpdateClasses), Access: admin, Allow: srv.classesInFacility, Features: prog},
		{Pattern: "PATCH /api/programs/{id}/classes/{cohort_id}", Handler: handle(srv.handleUpdateClass), Access: admin, Allow: srv.classInFacility, Features: prog},
		{Pattern: "DELETE /api/program-classes/{cohort_id}", Handler: handle(srv.handleDeleteClass), Access: admin, Allow: srv.classInFacility, Features: prog},
		{Pattern: "GET /api/program-classes/{cohort_id}/delete-check", Handler: handle(srv.handleGetClassDeleteCheck), Access: admin, Allow: srv.classInFacility, Features: prog},

		// The class tier
		{Pattern: "GET /api/classes", Handler: handle(srv.handleIndexProgramClasses), Features: prog},
		{Pattern: "GET /api/classes/{id}", Handler: handle(srv.handleGetProgramClass), Allow: class, Features: prog},
		{Pattern: "POST /api/classes", Handler: handle(srv.handleCreateProgramClass), Access: admin, Features: prog},
		{Pattern: "PATCH /api/classes/{id}", Handler: handle(srv.handleUpdateProgramClass), Access: admin, Allow: class, Features: prog},

		// Class events
		{Pattern: "GET /api/student-calendar", Handler: handle(srv.handleGetStudentCalendar), Allow: residentPrograms, Features: prog},
		{Pattern: "GET /api/program-classes/{cohort_id}/events", Handler: handle(srv.handleGetProgramClassEvents), Features: prog},
		{Pattern: "GET /api/program-classes/{cohort_id}/canvas-schedule", Handler: handle(srv.handleGetCanvasClassSchedule), Access: admin, Features: prog},
		{Pattern: "GET /api/admin-calendar", Handler: handle(srv.handleGetAdminCalendar), Access: admin, Features: prog},
		{Pattern: "GET /api/program-classes/todays-schedule", Handler: handle(srv.handleGetTodaysSchedule), Access: admin, Features: prog},
		{Pattern: "PUT /api/program-classes/{cohort_id}/events/{event_id}", Handler: handle(srv.handleEventOverrides), Access: admin, Allow: cohort, Features: prog},
		{Pattern: "PATCH /api/program-classes/{cohort_id}/events/{event_id}", Handler: handle(srv.handlePatchEventOverride), Access: admin, Allow: cohort, Features: prog},
		{Pattern: "DELETE /api/program-classes/{cohort_id}/events/{event_override_id}", Handler: handle(srv.handleDeleteEventOverride), Access: admin, Allow: cohort, Features: prog},
		{Pattern: "POST /api/program-classes/{cohort_id}/events/{event_override_id}/uncancel", Handler: handle(srv.handleUncancelOverride), Access: admin, Allow: cohort, Features: prog},
		{Pattern: "POST /api/program-classes/{cohort_id}/events/{event_id}/uncancel-series", Handler: handle(srv.handleUncancelSeries), Access: admin, Allow: cohort, Features: prog},
		{Pattern: "POST /api/program-classes/{cohort_id}/events", Handler: handle(srv.handleCreateEvent), Access: admin, Allow: cohort, Features: prog},
		{Pattern: "PUT /api/program-classes/{cohort_id}/events", Handler: handle(srv.handleRescheduleEventSeries), Access: admin, Allow: cohort, Features: prog},

		// Enrollments
		{Pattern: "GET /api/program-classes/{cohort_id}/enrollments", Handler: handle(srv.handleGetEnrollmentsForProgram), Access: admin, Allow: srv.classInFacility, Features: prog},
		{Pattern: "POST /api/program-classes/{cohort_id}/enrollment-conflicts", Handler: handle(srv.handleCheckEnrollmentConflicts), Access: admin, Allow: srv.classInFacility, Features: prog},
		{Pattern: "POST /api/program-classes/{cohort_id}/enrollments", Handler: handle(srv.handleEnrollUsersInClass), Access: admin, Allow: cohort, Features: prog},
		{Pattern: "PATCH /api/program-classes/{cohort_id}/enrollments", Handler: handle(srv.handleUpdateProgramClassEnrollments), Access: admin, Allow: cohort, Features: prog},
		{Pattern: "PATCH /api/program-classes/{cohort_id}/enrollments/{enrollment_id}/date", Handler: handle(srv.handleUpdateEnrollmentDate), Access: admin, Allow: cohort, Features: prog},
		{Pattern: "GET /api/programs/{id}/classes/{cohort_id}/enrollments/{enrollment_id}/attendance", Handler: handle(srv.handleGetProgramClassEnrollmentsAttendance), Access: admin, Allow: cohort, Features: prog},
		{Pattern: "GET /api/program-users/{id}/program-completions", Handler: handle(srv.handleGetUserProgramCompletions), UserParam: "id", Features: prog},

		// Attendance
		{Pattern: "GET /api/program-classes/{cohort_id}/events/{event_id}/attendance", Handler: handle(srv.handleGetEventAttendance), Access: admin, Allow: srv.attendanceInFacility, Features: prog},
		{Pattern: "GET /api/program-classes/{cohort_id}/events/{event_id}/attendance-rate", Handler: handle(srv.handleGetAttendanceRateForEvent), Access: admin, Allow: srv.attendanceInFacility, Features: prog},
		{Pattern: "GET /api/program-classes/{cohort_id}/historical-enrollment-batch", Handler: handle(srv.handleGetHistoricalEnrollmentBatch), Access: admin, Allow: srv.attendanceInFacility, Features: prog},
		{Pattern: "POST /api/program-classes/{cohort_id}/events/{event_id}/attendance", Handler: handle(srv.handleAddAttendanceForEvent), Access: admin, Allow: srv.attendanceInFacility, Features: prog},
		{Pattern: "DELETE /api/program-classes/{cohort_id}/events/{event_id}/attendance/{user_id}", Handler: handle(srv.handleDeleteAttendee), Access: admin, Allow: srv.attendanceInFacility, Features: prog},

		// Reports
		{Pattern: "POST /api/reports/generate", Handler: handle(srv.handleGenerateReport), Access: admin, Features: prog},

		// Rooms and instructors
		{Pattern: "GET /api/rooms", Handler: handle(srv.handleGetRooms), Access: admin, Features: prog},
		{Pattern: "POST /api/rooms", Handler: handle(srv.handleCreateRoom), Access: admin, Features: prog},
		{Pattern: "GET /api/program-facilities/{facilityId}/instructors", Handler: handle(srv.handleGetFacilityInstructors), Access: admin, Allow: facilityInPath("facilityId"), Features: prog},

		// A user's programs and attendance
		{Pattern: "GET /api/program-users/{id}/programs", Handler: handle(srv.handleGetUserPrograms), UserParam: "id", Allow: residentPrograms},
		{Pattern: "GET /api/program-users/{id}/attendance-trend", Handler: handle(srv.handleGetUserAttendanceTrend), UserParam: "id"},
		{Pattern: "GET /api/program-users/{id}/attendance-export", Handler: handle(srv.handleExportResidentAttendanceCSV), Access: admin, Allow: srv.userInPath("id")},
	}
}

// The access checks below are core's resolvers for these routes, unchanged. A
// check that fails answers 401, as core's did.

// inFacility passes admins who can switch facility, and anyone else whose
// facility is the facility_id of the table's row whose id is the path
// parameter.
func (srv *server) inFacility(table, param string) func(*http.Request, *identity.Claims) bool {
	return func(r *http.Request, claims *identity.Claims) bool {
		if claims.CanSwitchFacility() {
			return true
		}
		var facID uint
		err := srv.store.Table(table).
			Select("facility_id").
			Where("id = ?", r.PathValue(param)).
			Limit(1).Scan(&facID).Error
		return err == nil && claims.FacilityID == facID
	}
}

// facilityInPath passes admins who can switch facility, and anyone else whose
// facility is the path parameter.
func facilityInPath(param string) func(*http.Request, *identity.Claims) bool {
	return func(r *http.Request, claims *identity.Claims) bool {
		if claims.CanSwitchFacility() {
			return true
		}
		facID, err := strconv.ParseUint(r.PathValue(param), 10, 64)
		return err == nil && claims.FacilityID == uint(facID)
	}
}

// userInPath passes admins who can switch facility, the user in the path
// parameter, and admins at that user's facility.
func (srv *server) userInPath(param string) func(*http.Request, *identity.Claims) bool {
	return func(r *http.Request, claims *identity.Claims) bool {
		if claims.CanSwitchFacility() {
			return true
		}
		id, err := strconv.Atoi(r.PathValue(param))
		if err != nil {
			return false
		}
		if claims.UserID == uint(id) {
			return true
		}
		if !slices.Contains(users.AdminRoles, claims.Role) {
			return false
		}
		user, err := srv.store.GetUserByID(uint(id))
		return err == nil && user.FacilityID == claims.FacilityID
	}
}

// residentPrograms passes admins, and residents whose facility has resident
// programs turned on.
func residentPrograms(_ *http.Request, claims *identity.Claims) bool {
	if slices.Contains(users.AdminRoles, claims.Role) {
		return true
	}
	return claims.HasFeature(features.ResidentPrograms)
}

// deptAdminForAll passes admins, except that only admins who can switch
// facility may ask for ?all=true.
func deptAdminForAll(r *http.Request, claims *identity.Claims) bool {
	all := r.URL.Query().Get("all") == "true"
	if all && !claims.CanSwitchFacility() {
		return false
	}
	return claims.IsAdmin()
}

// classInFacility is inFacility for the class in cohort_id, which may also be
// a Canvas class: a facility-scoped Canvas ID carries its facility, and an
// old-style one is scoped by the handler.
func (srv *server) classInFacility(r *http.Request, claims *identity.Claims) bool {
	if claims.CanSwitchFacility() {
		return true
	}
	id, err := strconv.ParseUint(r.PathValue("cohort_id"), 10, 64)
	if err != nil {
		return false
	}
	// Facility-scoped canvas ID: assert the embedded facility matches the JWT.
	if uint(id) >= models.CanvasFacilityClassIDOffset {
		embeddedFacilityID, _, _ := decodeFacilityCanvasClassID(uint(id))
		return claims.FacilityID == embeddedFacilityID
	}
	// Old-style canvas ID: allow through; handlers scope data via JWT claims.
	if uint(id) >= models.CanvasClassIDOffset {
		return true
	}
	var facID uint
	return srv.store.Table("program_class_cohorts").
		Select("facility_id").
		Where("id = ?", id).
		Limit(1).Scan(&facID).Error == nil && claims.FacilityID == facID
}

// attendanceInFacility is inFacility for the class in cohort_id, letting any
// Canvas class through.
func (srv *server) attendanceInFacility(r *http.Request, claims *identity.Claims) bool {
	if claims.CanSwitchFacility() {
		return true
	}
	id, err := strconv.ParseUint(r.PathValue("cohort_id"), 10, 64)
	if err != nil {
		return false
	}
	if uint(id) >= models.CanvasClassIDOffset {
		return true
	}
	var facID uint
	return srv.store.Table("program_class_cohorts").
		Select("facility_id").
		Where("id = ?", id).
		Limit(1).Scan(&facID).Error == nil && claims.FacilityID == facID
}

// classesInFacility passes when every class in the ?id= list is at the
// caller's facility, or any facility for department and system admins. It
// fails for any list classIDsFromQuery rejects.
func (srv *server) classesInFacility(r *http.Request, claims *identity.Claims) bool {
	classIDs, err := classIDsFromQuery(r)
	if err != nil {
		return false
	}
	if claims.CanSwitchFacility() {
		return true
	}
	slices.Sort(classIDs)
	classIDs = slices.Compact(classIDs)
	var count int64
	return srv.store.WithContext(r.Context()).
		Table("program_class_cohorts").
		Where("id IN (?) AND facility_id = ?", classIDs, claims.FacilityID).
		Count(&count).Error == nil && count == int64(len(classIDs))
}

// programInFacility passes when the program_id in the path is offered at the
// request's facility and matches check. A statewide admin (facility 0) passes
// only when there is no check, for facility-agnostic reads.
func (srv *server) programInFacility(check string) func(*http.Request, *identity.Claims) bool {
	return func(r *http.Request, claims *identity.Claims) bool {
		// Facility is the request's, as getQueryContext has it: the
		// ?facility_id= param for switch-capable admins, else their home
		// facility.
		facilityID := requestFacilityID(r, claims)
		if facilityID == 0 {
			return check == ""
		}
		var count int64
		return srv.store.WithContext(r.Context()).
			Table("programs").
			Where(fmt.Sprintf("id = ? AND %s id IN (SELECT program_id FROM facilities_programs WHERE facility_id = ?)", check),
				r.PathValue("program_id"), facilityID).
			Count(&count).Error == nil && count > 0
	}
}
