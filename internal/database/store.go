package database

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"github.com/go-playground/validator/v10"
	log "github.com/sirupsen/logrus"
)

// Validate is the validator for Programs' models and requests.
var Validate = sync.OnceValue(func() *validator.Validate {
	return validator.New(validator.WithRequiredStructEnabled())
})

// The reads of core's users and facilities tables that Programs makes, copied
// from core's database package.

func (db *DB) GetUserByID(id uint) (*models.User, error) {
	user := models.User{}
	if err := db.First(&user, id).Error; err != nil {
		return nil, newNotFoundDBError(err, "users")
	}
	return &user, nil
}

func (db *DB) DeactivatedUsersPresent(userIDs []int) (bool, error) {
	if len(userIDs) == 0 {
		return false, nil
	}
	var count int64
	if err := db.Model(&models.User{}).Where("id IN (?) AND deactivated_at IS NOT NULL", userIDs).Count(&count).Error; err != nil {
		log.Errorf("Error checking deactivated users: %v", err)
		return false, newGetRecordsDBError(err, "users")
	}
	return count > 0, nil
}

func (db *DB) GetAllFacilitiesOrdered() ([]models.Facility, error) {
	var facilities []models.Facility
	if err := db.Order("name").Find(&facilities).Error; err != nil {
		return nil, newGetRecordsDBError(err, "facilities")
	}
	return facilities, nil
}

func (db *DB) GetFacilityByID(id int) (*models.Facility, error) {
	var facility models.Facility
	if err := db.Where("id = ?", fmt.Sprintf("%d", id)).First(&facility).Error; err != nil {
		return nil, newNotFoundDBError(err, "facilities")
	}
	return &facility, nil
}

func (db *DB) GetFacilityNamesByIDs(ids []uint) ([]string, error) {
	var names []string
	if len(ids) == 0 {
		return names, nil
	}
	if err := db.Model(&models.Facility{}).Where("id IN ?", ids).Order("name").Pluck("name", &names).Error; err != nil {
		return nil, newGetRecordsDBError(err, "facilities")
	}
	return names, nil
}

func adjustUserOrderBy(arg string) string {
	if strings.Contains(arg, "name_last") {
		return arg + ", name_first"
	}
	return arg
}

func StringPtr(s string) *string {
	return &s
}

// NilIfBlank returns nil for empty or whitespace-only strings so they are stored as NULL.
func NilIfBlank(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func UintPtr(u uint) *uint {
	return &u
}

func DerefUint(ptr *uint) uint {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func calcOffset(page, perPage int) int {
	return (page - 1) * perPage
}

// updateStruct copies src's non-zero fields, and every bool, onto dst.
func updateStruct(dst, src any) {
	srcVal := reflect.ValueOf(src).Elem()
	dstVal := reflect.ValueOf(dst).Elem()

	if srcVal.Kind() == reflect.Pointer {
		srcVal = srcVal.Elem()
	}

	if dstVal.Kind() == reflect.Pointer {
		dstVal = dstVal.Elem()
	}

	for i := 0; i < srcVal.NumField(); i++ {
		srcField := srcVal.Field(i)
		dstField := dstVal.Field(i)

		if srcField.Kind() == reflect.Bool { //always set booleans
			dstField.Set(srcField)
			continue
		}

		if !reflect.DeepEqual(srcField.Interface(), reflect.Zero(srcField.Type()).Interface()) {
			dstField.Set(srcField)
		}
	}
}
