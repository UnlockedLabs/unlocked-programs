package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/UnlockedLabs/unlocked-contracts/features"
	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"github.com/stretchr/testify/assert"
)

// fakeChecker answers from on, with err.
type fakeChecker struct {
	on    map[uint]bool
	err   error
	asked map[uint]int
}

func (f *fakeChecker) Enabled(_ context.Context, facilityID uint, feature features.Access) (bool, error) {
	if feature != features.ProviderPlatforms {
		panic("asked about " + feature)
	}
	f.asked[facilityID]++
	return f.on[facilityID], f.err
}

// Canvas classes at a facility with Provider Platforms off are left out. When
// core can't answer, the checker's last answer decides. Each facility is
// asked about once.
func TestFilterClassesByProviderAccess(t *testing.T) {
	classes := []models.ProgramClassCohort{{FacilityID: 1}, {FacilityID: 2}, {FacilityID: 1}, {FacilityID: 3}}
	for _, tc := range []struct {
		name string
		on   map[uint]bool
		err  error
		want []uint
	}{
		{"on at 1 and 3", map[uint]bool{1: true, 3: true}, nil, []uint{1, 1, 3}},
		{"off everywhere", nil, nil, []uint{}},
		{"core can't answer", map[uint]bool{2: true}, errors.New("core is down"), []uint{2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checker := &fakeChecker{on: tc.on, err: tc.err, asked: map[uint]int{}}
			srv := &server{facilityFeatures: checker}
			got := []uint{}
			for _, c := range srv.filterClassesByProviderAccess(t.Context(), classes) {
				got = append(got, c.FacilityID)
			}
			assert.Equal(t, tc.want, got)
			assert.Equal(t, map[uint]int{1: 1, 2: 1, 3: 1}, checker.asked)
		})
	}
}
