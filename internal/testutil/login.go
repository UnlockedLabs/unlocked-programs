package testutil

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/UnlockedLabs/unlocked-contracts/features"
	"github.com/UnlockedLabs/unlocked-contracts/identity"
)

// ClaimsHeader carries the caller's identity.Claims as JSON, in place of the
// login session tests don't have, as core's X-Test-Claims did.
const ClaimsHeader = "X-Test-Claims"

// Login passes the caller in ClaimsHeader, as identity.SessionChecker passes a
// logged-in caller. Claims without feature_access have every feature on, as
// core's test claims did.
func Login(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c identity.Claims
		if err := json.Unmarshal([]byte(r.Header.Get(ClaimsHeader)), &c); err != nil {
			http.Error(w, "test claims: "+err.Error(), http.StatusUnauthorized)
			return
		}
		if c.FeatureAccess == nil {
			c.FeatureAccess = features.All
		}
		next.ServeHTTP(w, r.WithContext(identity.NewContext(r.Context(), &c)))
	})
}

// FeaturesOn turns every feature on at every facility, as core's test
// database did.
type FeaturesOn struct{}

func (FeaturesOn) Enabled(context.Context, uint, features.Access) (bool, error) { return true, nil }
