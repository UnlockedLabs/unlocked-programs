package models

import "time"

// The reports' request and the rows their queries return.

type ReportType string

const (
	AttendanceReport         ReportType = "attendance"
	ProgramOutcomesReport    ReportType = "program_outcomes"
	FacilityComparisonReport ReportType = "facility_comparison"
	ClassRosterReport        ReportType = "class_roster"
	ResidentProfileReport    ReportType = "resident_profile"
)

type ReportFormat string

const (
	FormatCSV   ReportFormat = "csv"
	FormatPDF   ReportFormat = "pdf"
	FormatExcel ReportFormat = "excel"
)

type ReportGenerateRequest struct {
	Type      ReportType   `json:"type" validate:"required,oneof=attendance program_outcomes facility_comparison class_roster resident_profile"`
	Format    ReportFormat `json:"format" validate:"required,oneof=csv pdf excel"`
	StartDate time.Time    `json:"start_date" validate:"required"`
	EndDate   time.Time    `json:"end_date" validate:"required,gtefield=StartDate"`

	FacilityID   *uint         `json:"facility_id"`
	FacilityIDs  []uint        `json:"facility_ids" validate:"omitempty,dive,min=1"`
	ProgramID    *uint         `json:"program_id"`
	ProgramIDs   []uint        `json:"program_ids" validate:"omitempty,dive,min=1"`
	CohortID     *uint         `json:"cohort_id"`
	UserID       *uint         `json:"user_id"`
	ClassStatus  *string       `json:"class_status"`
	ProgramTypes []ProgType    `json:"program_types"`
	FundingTypes []FundingType `json:"funding_types"`

	IncludeClassBreakdown   bool     `json:"include_class_breakdown"`
	IncludeInactive         bool     `json:"include_inactive"`
	EnrollmentStatuses      []string `json:"enrollment_statuses"`
	IncludeIncompleteReason bool     `json:"include_incomplete_reason"`
	IncludeAttendanceRate   bool     `json:"include_attendance_rate"`
	IncludeEnrollmentDates  bool     `json:"include_enrollment_dates"`
}

type AttendanceReportRow struct {
	FacilityName     string
	ProgramName      string
	ClassName        string
	Date             time.Time
	StudentLastName  string
	StudentFirstName string
	DocID            string
	AttendanceStatus Attendance
	SeatTimeMinutes  *int
	AbsenceReason    *string
	RecordedBy       *string
}

type ProgramOutcomesReportRow struct {
	ProgramName       string
	IsActive          bool
	ProgramType       string
	FacilitiesActive  int
	TotalClasses      int
	ActiveEnrollments int
	TotalEnrollments  int
	TotalCapacity     int
	Utilization       int
}

type ProgramClassBreakdownRow struct {
	ClassName         string
	Status            string
	CreditHours       *int64
	Capacity          int
	ActiveEnrollments int
	RangeEnrollments  int
}

type FacilityComparisonReportRow struct {
	FacilityName       string
	TotalPrograms      int
	ActivePrograms     int
	TotalEnrollments   int
	ActiveEnrollments  int
	CompletionRate     float64
	AttendanceRate     float64
	TopProgramType     string
	TotalCreditHours   float64
	CertificatesEarned int
	LastActivityDate   *time.Time
}

type ClassRosterReportRow struct {
	NameLast         string
	NameFirst        string
	DocID            string
	EnrollmentStatus string
	EnrolledAt       *time.Time
	EndedAt          *time.Time
	SessionsAttended int
	TotalSessions    int
}

type ResidentProfileReportRow struct {
	NameLast         string
	NameFirst        string
	DocID            string
	FacilityName     string
	ProgramName      string
	ClassName        string
	EnrollmentStatus string
	EnrolledAt       *time.Time
	EndedAt          *time.Time
	SessionsAttended int
	TotalSessions    int
}
