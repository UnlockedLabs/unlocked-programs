package models

import (
	"context"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/UnlockedLabs/unlocked-contracts/features"
)

type PaginationMeta struct {
	CurrentPage int   `json:"current_page"`
	LastPage    int   `json:"last_page"`
	PerPage     int   `json:"per_page"`
	Total       int64 `json:"total"`
}

func NewPaginationInfo(currentPage, perPage int, total int64) PaginationMeta {
	lastPage := int(total) / perPage
	if int(total)%perPage > 0 {
		lastPage++
	}
	return PaginationMeta{
		CurrentPage: currentPage,
		LastPage:    lastPage,
		PerPage:     perPage,
		Total:       total,
	}
}

// QueryContext is a list request's paging, filters and caller.
type QueryContext struct {
	Params             url.Values
	Page               int
	PerPage            int
	FacilityID         uint
	UserID             uint
	OrderBy            string
	Order              string
	IsAdmin            bool
	CanSwitchFacility  bool
	Search             string
	Tags               []string
	Total              int64
	All                bool
	Ctx                context.Context
	Timezone           string
	IncludeDeactivated bool
	// FeatureAccess is the caller's effective (per-facility) feature set.
	FeatureAccess []features.Access
}

func (q *QueryContext) HasFeature(feature features.Access) bool {
	return slices.Contains(q.FeatureAccess, feature)
}

func (q *QueryContext) MaybeID(key string) *int {
	var idptr *int
	id, err := strconv.Atoi(q.Params.Get(key))
	if err == nil {
		idptr = &id
	}
	return idptr
}

func (q QueryContext) IntoMeta() PaginationMeta {
	return NewPaginationInfo(q.Page, q.PerPage, q.Total)
}

func (q QueryContext) CalcOffset() int {
	return int(math.Abs(float64((q.Page - 1) * q.PerPage)))
}

// OrderClause is the request's order when order_by is one of allowed and the
// direction is asc or desc (asc when absent), otherwise fallback. allowed is
// the exact set of order_by values the query can sort by.
func (q QueryContext) OrderClause(fallback string, allowed ...string) string {
	if !slices.Contains(allowed, q.OrderBy) {
		return fallback
	}
	switch dir := strings.ToLower(q.Order); dir {
	case "":
		return q.OrderBy + " asc"
	case "asc", "desc":
		return q.OrderBy + " " + dir
	}
	return fallback
}

func (q QueryContext) SearchQuery() string {
	if q.Search == "" {
		return ""
	}
	return "%" + q.Search + "%"
}
