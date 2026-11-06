package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/UnlockedLabs/unlocked-contracts/identity"
	"github.com/UnlockedLabs/unlocked-programs/internal/database"
	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"github.com/sirupsen/logrus"
)

// The response, error, paging and request helpers Programs' handlers use.
// They are copies of core's, so every response is the same JSON as in core.

type resource[T any] struct {
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type paginatedResource[T any] struct {
	Message string                `json:"message"`
	Data    []T                   `json:"data"`
	Meta    models.PaginationMeta `json:"meta,omitempty"`
}

func writePaginatedResponse[T any](w http.ResponseWriter, status int, data []T, meta models.PaginationMeta) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := paginatedResource[T]{Message: "", Data: data, Meta: meta}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return newResponseServiceError(err)
	}
	return nil
}

func writeJsonResponse[T any](w http.ResponseWriter, status int, data T) error {
	if any(data) == nil {
		w.WriteHeader(status)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if str, ok := any(data).(string); ok {
		resp := resource[struct{}]{Message: str, Data: struct{}{}}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			return newResponseServiceError(err)
		}
		return nil
	}
	resp := resource[T]{Message: "Data fetched successfully", Data: data}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return newResponseServiceError(err)
	}
	return nil
}

func errorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resource[any]{Message: message}); err != nil {
		logrus.Error("error writing error response: ", err)
		http.Error(w, message, status)
	}
}

// handlerFunc is a route handler that returns its error for handle to write.
type handlerFunc func(w http.ResponseWriter, r *http.Request, log sLog) error

// handle turns h into an http.Handler that writes h's error as core's
// handleError does. An admin's write also gets core's admin audit line, with
// the same fields, after h runs.
func handle(h handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := sLog{f: logrus.Fields{"module": "programs", "method": r.Method, "path": r.URL.Path}}
		claims := claimsFrom(r)
		audit := claims.IsAdmin() && r.Method != http.MethodGet
		if audit {
			log.add("admin_id", claims.UserID)
			log.add("username", claims.Username)
			log.add("role", claims.Role)
			log.add("session_id", claims.SessionID)
			log.add("facility_id", claims.FacilityID)
			log.add("facility_name", claims.FacilityName)
			log.add("ip_address", clientIP(r))
		}
		if err := h(w, r, log); err != nil {
			var svcErr serviceError
			if errors.As(err, &svcErr) {
				errorResponse(w, svcErr.Status, svcErr.Message)
				log.error("Error occurred is ", svcErr.Err)
			} else {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				log.error("Error occurred is ", err.Error())
			}
		}
		if audit {
			log.f["audit"] = "admin_action"
			logrus.WithFields(log.f).Println()
		}
	})
}

// clientIP is the caller's address: the first X-Forwarded-For entry, which
// the ingress sets, else the connection's.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip, _, _ := strings.Cut(fwd, ",")
		return ip
	}
	return r.RemoteAddr
}

// sLog adds fields to a request's log lines, like core's sLog.
type sLog struct {
	f logrus.Fields
}

// add adds a field to the request's log lines, including the admin audit
// line.
func (slog sLog) add(key string, value any) {
	slog.f[key] = value
}

func (slog sLog) info(args ...any) {
	logrus.WithFields(slog.f).Info(args...)
}

func (slog sLog) warn(args ...any) {
	logrus.WithFields(slog.f).Warn(args...)
}

func (slog sLog) warnf(format string, args ...any) {
	logrus.WithFields(slog.f).Warnf(format, args...)
}

func (slog sLog) error(args ...any) {
	logrus.WithFields(slog.f).Error(args...)
}

func (slog sLog) errorf(format string, args ...any) {
	logrus.WithFields(slog.f).Errorf(format, args...)
}

// serviceError is an error with the status and message a handler answers
// with.
type serviceError struct {
	Status  int
	Message string
	Err     error
}

func (svcErr serviceError) Error() string {
	return fmt.Sprintf("service error: %s", svcErr.Err.Error())
}

func newServiceError(err error, status int, msg string) serviceError {
	svcErr := serviceError{Status: status, Message: msg, Err: err}
	if dbErr, ok := err.(database.DBError); ok {
		svcErr.Err = errors.Join(errors.New(dbErr.Message), dbErr.InternalErr)
	}
	return svcErr
}

func newBadRequestServiceError(err error, msg string) serviceError {
	return newServiceError(err, http.StatusBadRequest, msg)
}

func newResponseServiceError(err error) serviceError {
	return newServiceError(err, http.StatusInternalServerError, "error writing response")
}

func newJSONReqBodyServiceError(err error) serviceError {
	return newServiceError(err, http.StatusBadRequest, "error reading json request")
}

func newInvalidIdServiceError(err error, idName string) serviceError {
	return newServiceError(err, http.StatusBadRequest, fmt.Sprintf("invalid %s", idName))
}

func newInvalidQueryParamServiceError(err error, paramName string) serviceError {
	return newServiceError(err, http.StatusBadRequest, fmt.Sprintf("invalid %s parameter", paramName))
}

func newUnauthorizedServiceError() serviceError {
	return newServiceError(errors.New("user unauthorized"), http.StatusUnauthorized, "Unauthorized")
}

func newForbiddenServiceError(err error, msg string) serviceError {
	return newServiceError(err, http.StatusForbidden, msg)
}

func newInternalServerServiceError(err error, msg string) serviceError {
	return newServiceError(err, http.StatusInternalServerError, msg)
}

// newDatabaseServiceError answers with a database.DBError's status and message, and
// with a 500 and the error's text for any other error.
func newDatabaseServiceError(err error) serviceError {
	if dbErr, ok := err.(database.DBError); ok {
		return serviceError{Status: dbErr.Status, Message: dbErr.Message, Err: dbErr.InternalErr}
	}
	return serviceError{Status: http.StatusInternalServerError, Message: err.Error(), Err: err}
}

// claimsFrom is the caller of a request. The router serves only requests the
// session check has passed, so the claims are always there.
func claimsFrom(r *http.Request) *identity.Claims {
	c, _ := identity.FromContext(r.Context())
	return c
}

// getQueryContext resolves the facility scope for a request: a switch-capable
// admin (sysadmin/dept admin) gets the ?facility_id they're viewing, or 0
// (statewide) when none is selected; a facility admin is always pinned to their
// own facility.
func getQueryContext(r *http.Request) models.QueryContext {
	var userID uint
	claims := claimsFrom(r)
	facilityID := requestFacilityID(r, claims)
	u, err := strconv.Atoi(r.URL.Query().Get("user_id"))
	if err != nil {
		userID = claims.UserID
	} else {
		userID = uint(u)
	}
	page, perPage := getPaginationInfo(r)
	orderBy := strings.ToLower(r.URL.Query().Get("order_by"))
	order := strings.ToLower(r.URL.Query().Get("order"))
	if orderBy != "" && order == "" {
		orderBySplit := strings.Fields(orderBy)
		if len(orderBySplit) > 1 {
			orderBy = orderBySplit[0]
			order = orderBySplit[1]
		}
	}
	return models.QueryContext{
		Params:             r.URL.Query(),
		Ctx:                r.Context(),
		Page:               page,
		PerPage:            perPage,
		FacilityID:         facilityID,
		UserID:             userID,
		OrderBy:            orderBy,
		Order:              order,
		IsAdmin:            claims.IsAdmin(),
		CanSwitchFacility:  claims.CanSwitchFacility(),
		Search:             strings.TrimSpace(strings.ToLower(r.URL.Query().Get("search"))),
		Tags:               r.URL.Query()["tags"],
		All:                r.URL.Query().Get("all") == "true",
		Timezone:           claims.TimeZone,
		IncludeDeactivated: r.URL.Query().Get("include_deactivated") == "true",
		FeatureAccess:      claims.FeatureAccess,
	}
}

// requestFacilityID is the facility a request is about, as getQueryContext
// resolves it.
func requestFacilityID(r *http.Request, claims *identity.Claims) uint {
	f, err := strconv.Atoi(r.URL.Query().Get("facility_id"))
	switch {
	case err == nil && claims.CanSwitchFacility():
		return uint(f)
	case claims.CanSwitchFacility():
		return 0
	default:
		return claims.FacilityID
	}
}

// facilityScopedQueryContext is for reads whose results are inherently
// per-facility. A statewide admin has no single facility (FacilityID == 0), so
// these fall back to the admin's home facility rather than matching no
// facility at all.
func facilityScopedQueryContext(r *http.Request) models.QueryContext {
	args := getQueryContext(r)
	if args.FacilityID == 0 {
		args.FacilityID = claimsFrom(r).FacilityID
	}
	return args
}

// requireFacilityID is the facility a request acts on. A statewide admin must
// pick one.
func requireFacilityID(r *http.Request) (uint, error) {
	facilityID := getQueryContext(r).FacilityID
	if facilityID == 0 {
		return 0, newBadRequestServiceError(nil, "facility selection is required")
	}
	return facilityID, nil
}

func getPaginationInfo(r *http.Request) (int, int) {
	page := r.URL.Query().Get("page")
	perPage := r.URL.Query().Get("per_page")
	if page == "" {
		page = "1"
	}
	if perPage == "" {
		perPage = "20"
	}
	intPage, err := strconv.Atoi(page)
	if err != nil || intPage < 1 {
		intPage = 1
	}
	intPerPage, err := strconv.Atoi(perPage)
	if err != nil || intPerPage < 1 {
		intPerPage = 20
	}
	return intPage, intPerPage
}

// getDateRange is the start_dt and end_dt of a request, in the caller's time
// zone, two weeks either side of now when they are missing.
func getDateRange(r *http.Request) (*models.DateRange, error) {
	tz, err := time.LoadLocation(claimsFrom(r).TimeZone)
	if err != nil {
		tz = time.UTC
	}

	parseDate := func(dateStr string) (time.Time, error) {
		formats := []string{"02-01-2006", "2006-01-02", "2006-01-02T15:04:05Z07:00"}
		for _, format := range formats {
			if t, err := time.Parse(format, dateStr); err == nil {
				return t, nil
			}
		}
		return time.Time{}, errors.New("invalid date format")
	}

	start, err := parseDate(r.URL.Query().Get("start_dt"))
	if err != nil {
		// if start is not provided, use two weeks ago
		start = time.Now().Add(-14 * 24 * time.Hour)
	}
	start = start.In(tz)
	end, err := parseDate(r.URL.Query().Get("end_dt"))
	if err != nil {
		// if end is not provided, use two weeks from now
		end = time.Now().Add(14 * 24 * time.Hour)
	}
	end = end.In(tz)
	if start.After(end) {
		return nil, errors.New("start date cannot be after end date")
	}
	return &models.DateRange{
		Start: start,
		End:   end,
		Tzone: tz,
	}, nil
}

func writeConflictResponse(w http.ResponseWriter, conflicts []models.RoomConflict) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	resp := resource[[]models.RoomConflict]{
		Message: conflictResponseMessage(conflicts),
		Data:    conflicts,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return newResponseServiceError(err)
	}
	return nil
}

func conflictResponseMessage(conflicts []models.RoomConflict) string {
	var hasRoom, hasInstructor bool
	for _, c := range conflicts {
		switch c.ConflictType {
		case models.ConflictTypeInstructor:
			hasInstructor = true
		default:
			hasRoom = true
		}
	}
	switch {
	case hasRoom && hasInstructor:
		return "the room and instructor are already booked during this time"
	case hasInstructor:
		return "the instructor is already teaching during this time"
	default:
		return "room is already booked during this time"
	}
}

func writeDeleteConflictResponse(w http.ResponseWriter, message string, blockers models.DeleteBlockingChildren) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	resp := resource[models.DeleteBlockingChildren]{
		Message: message,
		Data:    blockers,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return newResponseServiceError(err)
	}
	return nil
}
