// Package testutil is what the tests of more than one package share.
package testutil

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// NewDB returns an empty in-memory database with Programs' tables and the
// rows of core's tables that Programs reads and writes, closed when the test
// ends.
func NewDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(tables()...); err != nil {
		t.Fatal(err)
	}
	return db
}

// tables is core's tables Programs uses, then Programs' own. A class comes
// before its cohorts, so the cohort's foreign key has a table to point at.
func tables() []any {
	return []any{
		&models.Facility{},
		&models.User{},
		&models.UserAccountHistory{},
		&models.Program{},
		&models.ProgramCreditType{},
		&models.ProgramType{},
		&models.ProgramClass{},
		&models.ProgramClassCreditType{},
		&models.ProgramClassCohort{},
		&models.FacilitiesPrograms{},
		&models.ProgramClassEvent{},
		&models.ProgramClassEnrollment{},
		&models.ClassCompletion{},
		&models.ProgramClassEventOverride{},
		&models.ProgramClassEventAttendance{},
		&models.ChangeLogEntry{},
		&models.ProgramClassesHistory{},
		&models.Room{},
	}
}

// SeedCore adds what core's test database started with: its SuperAdmin (user
// 1) at its Default facility (1). Core's seed ran with no system_batch user,
// so the SuperAdmin's creator is the 99999 it falls back to.
func SeedCore(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now()
	if err := db.Exec(`INSERT INTO users (id, username, name_first, name_last, email, role, facility_id, create_user_id, created_at, updated_at)
		VALUES (1, 'SuperAdmin', 'Super', 'Admin', 'admin@unlocked.v2', 'system_admin', 1, 99999, ?, ?)`, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO facilities (id, name, timezone, create_user_id, created_at, updated_at)
		VALUES (1, 'Default', 'America/Chicago', 1, ?, ?)`, now, now).Error; err != nil {
		t.Fatal(err)
	}
}
