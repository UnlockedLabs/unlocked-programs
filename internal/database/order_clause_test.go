package database

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"github.com/UnlockedLabs/unlocked-programs/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Sort keys the frontend and core's internal caller send.
func TestOrderByListsKeepClientKeys(t *testing.T) {
	assert.Subset(t, programOverviewOrderBy, []string{"programs.name", "mr.total_active_enrollments", "completion_rate"})
	assert.Subset(t, programOrderBy, []string{"name"})
	assert.Subset(t, classDetailsOrderBy, []string{"ps.start_dt"})
	assert.Subset(t, enrollmentOrderBy, []string{"name_last", "doc_id", "start_dt"})
	assert.Subset(t, userProgramOrderBy, []string{"start_dt"})
}

// captureSQL is a store whose queries are built but not run, and the SQL each
// one produced. It lets the Postgres-only queries be checked on SQLite.
func captureSQL(t *testing.T) (*DB, *[]string) {
	t.Helper()
	db := testutil.NewDB(t)
	var sqls []string
	record := func(tx *gorm.DB) {
		sqls = append(sqls, tx.Statement.SQL.String())
		// gorm skips this reset in DryRun, so a following Find would reuse the Count's SQL.
		tx.Statement.SQL.Reset()
		tx.Statement.Vars = nil
	}
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("capture", record))
	require.NoError(t, db.Callback().Row().After("gorm:row").Register("capture_row", record))
	return NewDB(db.Session(&gorm.Session{DryRun: true})), &sqls
}

func hostile(orderBy, order string) *models.QueryContext {
	return &models.QueryContext{Ctx: context.Background(), PerPage: 10, Page: 1, OrderBy: orderBy, Order: order}
}

func TestStoreQueriesIgnoreHostileOrder(t *testing.T) {
	const payload = "(1/(select/**/0))"
	cases := []struct {
		name string
		run  func(s *DB, q *models.QueryContext)
		want string
	}{
		{"programs", func(s *DB, q *models.QueryContext) { _, _ = s.GetPrograms(q) }, "programs.created_at desc"},
		{"programs overview", func(s *DB, q *models.QueryContext) {
			_, _ = s.GetProgramsOverviewTable(q, 30, false, "", nil)
		}, "programs.created_at desc"},
		{"class details", func(s *DB, q *models.QueryContext) { _, _ = s.GetProgramClassDetailsByID(1, q) }, "ps.created_at desc"},
		{"class outcomes", func(s *DB, q *models.QueryContext) { _, _ = s.GetProgramClassOutcomes(1, q) }, "months.month asc"},
		{"class history", func(s *DB, q *models.QueryContext) { _, _ = s.GetProgramClassesHistory(1, "t", q) }, "created_at desc"},
		{"user programs", func(s *DB, q *models.QueryContext) { _, _ = s.GetUserProgramInfo(q, 1) }, "pce.created_at desc"},
		{"enrollments", func(s *DB, q *models.QueryContext) { _, _ = s.GetProgramClassEnrollmentsForProgram(q, 1, "all") }, "pse.created_at desc"},
		{"attendees", func(s *DB, q *models.QueryContext) { _, _ = s.GetAttendees(q, nil, 1) }, "program_class_event_attendance.created_at DESC"},
	}
	for _, c := range cases {
		for _, in := range [][2]string{{payload, ""}, {"name_last", "desc," + payload}} {
			t.Run(c.name+"/"+in[0]+" "+in[1], func(t *testing.T) {
				s, sqls := captureSQL(t)
				c.run(s, hostile(in[0], in[1]))
				require.NotEmpty(t, *sqls, "query was not built")
				for _, q := range *sqls {
					assert.NotContains(t, q, "select/**/0")
				}
				assert.True(t, slices.ContainsFunc(*sqls, func(q string) bool { return strings.Contains(q, "ORDER BY "+c.want) }),
					"no query ordered by %q", c.want)
			})
		}
	}
}

// The roster query builds its ORDER BY into a raw SQL string.
func TestRosterOrderIgnoresHostileOrder(t *testing.T) {
	const payload = "(1/(select/**/0))"
	cases := []struct{ orderBy, order, want string }{
		{"name_last", "asc", "name_last asc, name_first"},
		{"name_first", "desc", "name_first desc"},
		{"name_last", "desc," + payload, "e.created_at DESC"},
		{"name_last", "desc;" + payload, "e.created_at DESC"},
		{payload, "", "e.id ASC"},
		{"name_last, " + payload, "asc", "e.id ASC"},
	}
	for _, c := range cases {
		t.Run(c.orderBy+" "+c.order, func(t *testing.T) {
			assert.Equal(t, c.want, rosterOrder(hostile(c.orderBy, c.order)))
		})
	}
}
