package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/UnlockedLabs/unlocked-contracts/identity"
	"github.com/UnlockedLabs/unlocked-contracts/users"
	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TimePtr(t time.Time) *time.Time {
	return &t
}

func TestHandleEnrollUsersInClass_Validation(t *testing.T) {
	srv := newTestServer(t)

	// Setup Common Data
	facility := models.Facility{Name: "Test Facility"}
	if err := srv.store.Create(&facility).Error; err != nil {
		t.Fatal(err)
	}

	program := models.Program{Name: "Test Program"}
	if err := srv.store.Create(&program).Error; err != nil {
		t.Fatal(err)
	}

	activeUser := models.User{Username: "active_user", Email: "active_user@example.com", NameFirst: "Active", NameLast: "User", FacilityID: facility.ID}
	if err := srv.store.Create(&activeUser).Error; err != nil {
		t.Fatal(err)
	}

	activeUser2 := models.User{Username: "active_user_2", Email: "active_user_2@example.com", NameFirst: "Active", NameLast: "User2", FacilityID: facility.ID}
	if err := srv.store.Create(&activeUser2).Error; err != nil {
		t.Fatal(err)
	}

	deactivatedUser := models.User{Username: "deactivated_user", Email: "deactivated_user@example.com", NameFirst: "Deactivated", NameLast: "User", FacilityID: facility.ID, DeactivatedAt: TimePtr(time.Now())}
	if err := srv.store.Create(&deactivatedUser).Error; err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		classStatus    models.ClassStatus
		isArchived     bool
		targetUsers    []models.User
		capacity       int64
		setupClass     func(*models.ProgramClassCohort) // Optional extra setup
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "Enroll in Active Class (Success)",
			classStatus:    models.Active,
			targetUsers:    []models.User{activeUser},
			capacity:       10,
			expectedStatus: http.StatusCreated,
			expectedMsg:    "users enrolled",
		},
		{
			name:           "Enroll in Scheduled Class (Success)",
			classStatus:    models.Scheduled,
			targetUsers:    []models.User{activeUser},
			capacity:       10,
			expectedStatus: http.StatusCreated,
			expectedMsg:    "users enrolled",
		},
		{
			name:           "Enroll in Completed Class (Failure)",
			classStatus:    models.Completed,
			targetUsers:    []models.User{activeUser},
			capacity:       10,
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "cannot perform action on class that is completed cancelled or archived",
		},
		{
			name:           "Enroll in Cancelled Class (Failure)",
			classStatus:    models.Cancelled,
			targetUsers:    []models.User{activeUser},
			capacity:       10,
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "cannot perform action on class that is completed cancelled or archived",
		},
		{
			name:           "Enroll in Archived Class (Failure)",
			classStatus:    models.Active,
			isArchived:     true,
			targetUsers:    []models.User{activeUser},
			capacity:       10,
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "cannot perform action on class that is completed cancelled or archived",
		},
		{
			name:           "Enroll Deactivated User (Failure)",
			classStatus:    models.Active,
			targetUsers:    []models.User{deactivatedUser},
			capacity:       10,
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "deactivated user",
		},
		{
			name:        "Enroll in Full Class (Partial/Skipped)",
			classStatus: models.Active,
			targetUsers: []models.User{activeUser, activeUser2},
			capacity:    1, // Capacity 1, trying to enroll 2 users
			setupClass: func(c *models.ProgramClassCohort) {
			},
			expectedStatus: http.StatusCreated,
			expectedMsg:    "1 users were enrolled, 1 were not added because capacity is full.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			class := models.ProgramClassCohort{
				ProgramID:   program.ID,
				FacilityID:  facility.ID,
				Status:      tt.classStatus,
				Description: "Description",
				Capacity:    tt.capacity,
				StartDt:     time.Now(),
			}
			if tt.isArchived {
				class.ArchivedAt = TimePtr(time.Now())
			}
			if err := srv.store.Create(&class).Error; err != nil {
				t.Fatal(err)
			}

			if tt.setupClass != nil {
				tt.setupClass(&class)
			}

			userIDs := make([]int, len(tt.targetUsers))
			for i, u := range tt.targetUsers {
				userIDs[i] = int(u.ID)
			}

			enrollmentReq := struct {
				UserIDs []int `json:"user_ids"`
			}{
				UserIDs: userIDs,
			}
			body, _ := json.Marshal(enrollmentReq)

			req := httptest.NewRequest(http.MethodPost, "/api/program-classes/"+strconv.Itoa(int(class.ID))+"/enrollments", bytes.NewReader(body))
			req.SetPathValue("cohort_id", strconv.Itoa(int(class.ID)))

			w := httptest.NewRecorder()
			log := sLog{f: make(map[string]interface{})}

			err := srv.handleEnrollUsersInClass(w, req, log)

			if tt.expectedStatus >= 400 {
				assert.Error(t, err)
				svcErr, ok := err.(serviceError)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedStatus, svcErr.Status)
				assert.Contains(t, svcErr.Message, tt.expectedMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, w.Code)
				// For 201, the response body is a JSON object with a message
				var resp resource[any]
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Contains(t, resp.Message, tt.expectedMsg)
			}
		})
	}
}

// A completion records the email of the admin who graduated the resident,
// read from the users table as core's claims had it.
func TestGraduationRecordsAdminEmail(t *testing.T) {
	srv := newTestServer(t)
	facility := models.Facility{Name: "North", Timezone: "America/Chicago"}
	require.NoError(t, srv.store.Create(&facility).Error)
	admin := models.User{Username: "facadmin", Email: "facadmin@example.com", Role: users.FacilityAdmin, FacilityID: facility.ID}
	require.NoError(t, srv.store.Create(&admin).Error)
	resident := models.User{Username: "resident", Email: "resident@example.com", Role: users.Student, FacilityID: facility.ID}
	require.NoError(t, srv.store.Create(&resident).Error)
	program := models.Program{Name: "GED"}
	require.NoError(t, srv.store.Create(&program).Error)
	class := models.ProgramClassCohort{ProgramID: program.ID, FacilityID: facility.ID, Status: models.Active, Description: "Description", Capacity: 10, StartDt: time.Now()}
	require.NoError(t, srv.store.Create(&class).Error)
	require.NoError(t, srv.store.Create(&models.ProgramClassEnrollment{CohortID: class.ID, UserID: resident.ID, EnrollmentStatus: models.Enrolled}).Error)

	body, err := json.Marshal(map[string]any{"enrollment_status": "Completed", "user_ids": []uint{resident.ID}})
	require.NoError(t, err)
	cohortID := strconv.Itoa(int(class.ID))
	req := httptest.NewRequest(http.MethodPatch, "/api/program-classes/"+cohortID+"/enrollments", bytes.NewReader(body))
	req.SetPathValue("cohort_id", cohortID)
	req = req.WithContext(identity.NewContext(req.Context(), &identity.Claims{UserID: admin.ID, Role: admin.Role, FacilityID: facility.ID}))
	w := httptest.NewRecorder()
	require.NoError(t, srv.handleUpdateProgramClassEnrollments(w, req, sLog{f: map[string]any{}}))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var completion models.ClassCompletion
	require.NoError(t, srv.store.Where("user_id = ?", resident.ID).First(&completion).Error)
	assert.Equal(t, "facadmin@example.com", completion.AdminEmail)
}

func TestUpdateEnrollmentsRejectsUnknownStatus(t *testing.T) {
	srv := newTestServer(t)
	facility := models.Facility{Name: "North", Timezone: "America/Chicago"}
	require.NoError(t, srv.store.Create(&facility).Error)
	admin := models.User{Username: "facadmin", Email: "facadmin@example.com", Role: users.FacilityAdmin, FacilityID: facility.ID}
	require.NoError(t, srv.store.Create(&admin).Error)
	resident := models.User{Username: "resident", Email: "resident@example.com", Role: users.Student, FacilityID: facility.ID}
	require.NoError(t, srv.store.Create(&resident).Error)
	program := models.Program{Name: "GED"}
	require.NoError(t, srv.store.Create(&program).Error)
	class := models.ProgramClassCohort{ProgramID: program.ID, FacilityID: facility.ID, Status: models.Active, Description: "Description", Capacity: 10, StartDt: time.Now()}
	require.NoError(t, srv.store.Create(&class).Error)
	require.NoError(t, srv.store.Create(&models.ProgramClassEnrollment{CohortID: class.ID, UserID: resident.ID, EnrollmentStatus: models.Enrolled}).Error)
	cohortID := strconv.Itoa(int(class.ID))

	for _, status := range []string{"Incomplete: Transfered", "Bogus"} {
		t.Run(status, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"enrollment_status": status, "user_ids": []uint{resident.ID}})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPatch, "/api/program-classes/"+cohortID+"/enrollments", bytes.NewReader(body))
			req.SetPathValue("cohort_id", cohortID)
			req = req.WithContext(identity.NewContext(req.Context(), &identity.Claims{UserID: admin.ID, Role: admin.Role, FacilityID: facility.ID}))
			err = srv.handleUpdateProgramClassEnrollments(httptest.NewRecorder(), req, sLog{f: map[string]any{}})
			svcErr, ok := err.(serviceError)
			require.True(t, ok, "expected a serviceError, got %v", err)
			assert.Equal(t, http.StatusBadRequest, svcErr.Status)

			var stored models.ProgramClassEnrollment
			require.NoError(t, srv.store.Where("user_id = ?", resident.ID).First(&stored).Error)
			assert.Equal(t, models.Enrolled, stored.EnrollmentStatus)
		})
	}
}
