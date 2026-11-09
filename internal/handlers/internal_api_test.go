package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/UnlockedLabs/unlocked-contracts/coreapi"
	"github.com/UnlockedLabs/unlocked-contracts/programs"
	"github.com/UnlockedLabs/unlocked-contracts/providerplatforms"
	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Core asks over HTTP, with the contracts client, and gets the database's
// answers. Ending and withdrawing enrollments change nothing the second time,
// so core can retry them.
func TestInternalAPI(t *testing.T) {
	srv := newTestServer(t)
	const token = "s3cret"
	ts := httptest.NewServer(programs.NewHandler(newAPI(srv), token))
	t.Cleanup(ts.Close)
	core := programs.NewClient(ts.URL, token, nil)
	ctx := t.Context()

	facility := models.Facility{Name: "North", Timezone: "America/Chicago"}
	require.NoError(t, srv.store.Create(&facility).Error)
	resident := models.User{Username: "jdoe", Email: "jdoe@x", FacilityID: facility.ID}
	require.NoError(t, srv.store.Create(&resident).Error)
	program := models.Program{Name: "GED", IsActive: true}
	require.NoError(t, srv.store.Create(&program).Error)
	require.NoError(t, srv.store.Create(&models.FacilitiesPrograms{FacilityID: facility.ID, ProgramID: program.ID}).Error)
	class := models.ProgramClassCohort{ProgramID: program.ID, FacilityID: facility.ID, Status: models.Active, Description: "Morning", Capacity: 10, StartDt: time.Now()}
	require.NoError(t, srv.store.Create(&class).Error)
	enroll := func() {
		t.Helper()
		require.NoError(t, srv.store.Create(&models.ProgramClassEnrollment{CohortID: class.ID, UserID: resident.ID, EnrollmentStatus: models.Enrolled}).Error)
	}
	status := func() []models.ProgramEnrollmentStatus {
		t.Helper()
		var got []models.ProgramEnrollmentStatus
		require.NoError(t, srv.store.Model(&models.ProgramClassEnrollment{}).Order("id").Pluck("enrollment_status", &got).Error)
		return got
	}

	enroll()
	ids, err := core.EnrolledUserIDs(ctx, class.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint{resident.ID}, ids)

	counts, err := core.FacilityPrograms(ctx, []uint{facility.ID})
	require.NoError(t, err)
	// The cohort made its class, so the facility has one of each.
	assert.Equal(t, map[uint]programs.FacilityPrograms{facility.ID: {ActivePrograms: 1, ActiveClasses: 1, Classes: 1, Programs: 1}}, counts)

	for range 2 {
		require.NoError(t, core.EndEnrollmentsForTransfer(ctx, resident.ID, facility.ID))
	}
	assert.Equal(t, []models.ProgramEnrollmentStatus{models.EnrollmentIncompleteTransferred}, status())

	enroll()
	for range 2 {
		require.NoError(t, core.WithdrawEnrollments(ctx, resident.ID))
	}
	assert.Equal(t, []models.ProgramEnrollmentStatus{models.EnrollmentIncompleteTransferred, models.EnrollmentIncompleteWithdrawn}, status())

	// A wrong token gets nothing.
	_, err = programs.NewClient(ts.URL, "guess", nil).EnrolledUserIDs(ctx, class.ID)
	assert.ErrorContains(t, err, "401")
}

// fakeProviders is Provider Platforms' api answering Provider from a map.
type fakeProviders struct {
	providerplatforms.ProviderPlatforms
	providers map[uint]providerplatforms.Provider
}

func (f fakeProviders) Provider(_ context.Context, id uint) (providerplatforms.Provider, error) {
	return f.providers[id], nil
}

// Only a provider that lists its own users (Essential Ed) answers; the rest
// fall back to Provider Platforms' worker.
func TestLiveProvidersUsers(t *testing.T) {
	status := http.StatusOK
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(envelope([]map[string]interface{}{
			{"id": float64(4329946), "first_name": "Jane", "last_name": "Smith", "username": "jsmith", "email_address": "jane@example.com"},
		}, true))
	}))
	defer ts.Close()
	const essentialEd, kolibri = 1, 2
	live := liveProviders{&server{
		Client: ts.Client(),
		providerPlatforms: fakeProviders{providers: map[uint]providerplatforms.Provider{
			essentialEd: {ID: essentialEd, Type: providerplatforms.EssentialEd, Name: "Essential Ed", AccountID: "303", AccessKey: "token", BaseURL: ts.URL},
			kolibri:     {ID: kolibri, Type: providerplatforms.Kolibri, Name: "Kolibri", BaseURL: ts.URL},
		}},
	}}

	t.Run("essential ed lists its users", func(t *testing.T) {
		users, ok, err := live.LiveUsers(context.Background(), essentialEd)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, []coreapi.ProviderUser{{
			Username: "jsmith", NameFirst: "Jane", NameLast: "Smith", Email: "jane@example.com",
			ExternalUserID: "4329946", ExternalUsername: "jsmith",
		}}, users)
	})

	t.Run("kolibri goes through the worker", func(t *testing.T) {
		users, ok, err := live.LiveUsers(context.Background(), kolibri)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Nil(t, users)
	})

	t.Run("a failed listing is an error", func(t *testing.T) {
		status = http.StatusBadGateway
		_, ok, err := live.LiveUsers(context.Background(), essentialEd)
		assert.Error(t, err)
		assert.True(t, ok)
	})
}
