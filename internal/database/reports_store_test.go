package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapRosterStatusesTransferred(t *testing.T) {
	assert.Equal(t, []string{"Incomplete: Transferred"}, mapRosterStatuses([]string{"Transferred"}))
}
