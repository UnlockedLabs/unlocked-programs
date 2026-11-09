package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/UnlockedLabs/unlocked-contracts/features"
	"github.com/UnlockedLabs/unlocked-contracts/identity"
	"github.com/UnlockedLabs/unlocked-contracts/users"
	"github.com/stretchr/testify/require"
)

// Direct navigation to the resident Programs page must not render, which means
// the endpoints behind it have to refuse the resident — not just the nav item
// disappearing client-side.
func TestResidentProgramsAccess_RouteGating(t *testing.T) {
	env = SetupTestEnv(t)
	defer env.CleanupTestEnv()

	facility, err := env.CreateTestFacility("Resident Programs Route Facility")
	require.NoError(t, err)
	resident, err := env.CreateTestUser("residentprograms", users.Student, facility.ID, "")
	require.NoError(t, err)

	withPage := []features.Access{features.ProgramManagement, features.ResidentPrograms}
	withoutPage := []features.Access{features.ProgramManagement}

	residentClaims := func(features []features.Access) *identity.Claims {
		return &identity.Claims{
			UserID:        resident.ID,
			Role:          users.Student,
			FacilityID:    facility.ID,
			FeatureAccess: features,
		}
	}
	programsURL := fmt.Sprintf("/api/program-users/%d/programs", resident.ID)
	calendarURL := "/api/student-calendar"

	// handleGetUserPrograms runs raw Postgres-only SQL, so it can't return 200
	// against the in-memory SQLite test DB. Asserting "not refused" is what this
	// test is actually about: the gate let the request through to the handler.
	expectNotRefused := func(t *testing.T, claims *identity.Claims, url string) {
		t.Helper()
		resp := NewRequest[any](env.Client, t, http.MethodGet, url, nil).
			WithTestClaims(claims).
			Do()
		require.NotEqual(t, http.StatusUnauthorized, resp.resp.StatusCode,
			"the resident_programs gate must not block this caller")
	}

	t.Run("resident with the page enabled reaches their programs", func(t *testing.T) {
		expectNotRefused(t, residentClaims(withPage), programsURL)
	})

	t.Run("resident with the page enabled reaches the class calendar", func(t *testing.T) {
		NewRequest[any](env.Client, t, http.MethodGet, calendarURL, nil).
			WithTestClaims(residentClaims(withPage)).
			Do().
			ExpectStatus(http.StatusOK)
	})

	t.Run("resident with the page hidden is refused their programs", func(t *testing.T) {
		NewRequest[any](env.Client, t, http.MethodGet, programsURL, nil).
			WithTestClaims(residentClaims(withoutPage)).
			Do().
			ExpectStatus(http.StatusUnauthorized)
	})

	t.Run("resident with the page hidden is refused the class calendar", func(t *testing.T) {
		NewRequest[any](env.Client, t, http.MethodGet, calendarURL, nil).
			WithTestClaims(residentClaims(withoutPage)).
			Do().
			ExpectStatus(http.StatusUnauthorized)
	})

	// Hiding the page from residents must not touch any admin-side view.
	t.Run("admin without the flag still sees the resident's programs", func(t *testing.T) {
		expectNotRefused(t, &identity.Claims{
			Role:          users.FacilityAdmin,
			FacilityID:    facility.ID,
			FeatureAccess: withoutPage,
		}, programsURL)
	})
}
