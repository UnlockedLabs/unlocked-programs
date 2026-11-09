package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/UnlockedLabs/unlocked-contracts/features"
	"github.com/UnlockedLabs/unlocked-contracts/identity"
	"github.com/sirupsen/logrus"
)

// The router puts each route behind the same checks core ran on it before
// the move, with the same answers: the login session and CSRF token, then the
// route's own access checks, then the admin check, then the route's features.

// route is one HTTP route and who may call it.
type route struct {
	Pattern string
	Handler http.Handler
	// Access is who may call the route. The zero value is anyUser.
	Access accessLevel
	// UserParam names the path parameter that holds the user a route is about,
	// such as "id" in "GET /api/program-users/{id}/programs". Then only admins
	// who can see that user may call the route: department and system admins
	// see everyone, facility admins their own facility's users. With anyUser,
	// the user themself may too.
	UserParam string
	// Allow is the route's own access check, such as a class in the caller's
	// facility. It runs after UserParam's, and the caller needs to pass both.
	Allow func(r *http.Request, c *identity.Claims) bool
	// Features turns the route off unless every listed feature is enabled for
	// the caller.
	Features []features.Access
}

// accessLevel is who may call a route.
type accessLevel int

const (
	// anyUser is any logged-in user.
	anyUser accessLevel = iota
	// admin is any admin: facility, department or system.
	admin
)

// newRouter serves srv's routes, each behind requireLogin and its access and
// feature checks, and records each one's metrics. requireLogin checks the
// session and puts the caller's identity.Claims in the request's context, as
// identity.SessionChecker's Require does.
func (srv *server) newRouter(requireLogin func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()
	for _, rt := range srv.routes() {
		mux.Handle(rt.Pattern, observe(rt.Pattern, requireLogin(srv.guard(rt))))
	}
	return securityHeaders(mux)
}

// guard runs rt's access and feature checks before its handler.
func (srv *server) guard(rt route) http.Handler {
	h := requireFeatures(rt.Handler, rt.Features)
	switch rt.Access {
	case anyUser:
	case admin:
		h = requireAdmin(h)
	default:
		// Routes are built once at startup, so an unknown level stops the
		// server instead of leaving a route open.
		panic(fmt.Sprintf("route %q: unknown access level %d", rt.Pattern, rt.Access))
	}
	var checks []func(*http.Request, *identity.Claims) bool
	if rt.UserParam != "" {
		checks = append(checks, srv.userCheck(rt))
	}
	if rt.Allow != nil {
		checks = append(checks, rt.Allow)
	}
	if len(checks) == 0 {
		return h
	}
	return requireAll(h, checks)
}

// userCheck is the check for rt's UserParam, core's check for the user in a
// route's path. A misconfigured route stops startup instead of leaving the
// route open or closed by mistake.
func (srv *server) userCheck(rt route) func(*http.Request, *identity.Claims) bool {
	if !strings.Contains(rt.Pattern, "{"+rt.UserParam+"}") {
		panic(fmt.Sprintf("route %q: no path parameter %q", rt.Pattern, rt.UserParam))
	}
	if rt.Access == admin {
		return srv.inFacility("users", rt.UserParam)
	}
	return srv.userInPath(rt.UserParam)
}

// requireAll passes callers who pass every check. Its answer is core's route
// resolver's: plain text, not JSON.
func requireAll(next http.Handler, checks []func(*http.Request, *identity.Claims) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := claimsFrom(r)
		for _, allowed := range checks {
			if !allowed(r, claims) {
				logrus.WithFields(logrus.Fields{
					"handler":     "authMiddleware",
					"method":      r.Method,
					"path":        r.URL.Path,
					"user_id":     claims.UserID,
					"facility_id": claims.FacilityID,
				}).Warn("route resolver denied access to resource")
				http.Error(w, "User is not allowed to view this resource", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !claimsFrom(r).IsAdmin() {
			errorResponse(w, http.StatusUnauthorized, "Unauthorized - not admin")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requireFeatures(next http.Handler, required []features.Access) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := claimsFrom(r)
		if !claims.HasFeature(required...) {
			logrus.WithFields(logrus.Fields{
				"handler":           "checkFeatureAccessMiddleware",
				"method":            r.Method,
				"path":              r.URL.Path,
				"required_features": required,
				"user_id":           claims.UserID,
				"facility_id":       claims.FacilityID,
				"feature_access":    claims.FeatureAccess,
			}).Warn("feature not enabled for user")
			errorResponse(w, http.StatusUnauthorized, "Feature not enabled")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeaders sets core's security headers. No Programs page is framed.
func securityHeaders(next http.Handler) http.Handler {
	dev := os.Getenv("APP_ENV") == "dev"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !dev {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
