package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/UnlockedLabs/unlocked-programs/internal/models"
	"github.com/stretchr/testify/require"
)

// Every report renders with its template, through JasperStarter. The test
// needs jasperstarter, so it runs in the golang-jasper image and is skipped
// anywhere else:
//
//	docker run --rm -v "$PWD":/src -w /src -e GOTOOLCHAIN=auto ghcr.io/unlockedlabs/golang-jasper:latest go test ./internal/handlers -run TestJasperReports
func TestJasperReports(t *testing.T) {
	if _, err := exec.LookPath("jasperstarter"); err != nil {
		t.Skip("jasperstarter isn't installed")
	}
	templates, err := filepath.Abs("../../templates")
	require.NoError(t, err)
	t.Setenv("JASPER_TEMPLATE_DIR", templates)
	t.Setenv("JASPER_TEMP_DIR", t.TempDir())

	srv := newTestServer(t)
	srv.reports = JasperReports{}
	day := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	status := "Active"
	for _, tc := range []struct {
		typ    models.ReportType
		report reportExporter
	}{
		{models.AttendanceReport, AttendanceReportData{Data: []models.AttendanceReportRow{{
			FacilityName: "North", ProgramName: "GED", ClassName: "Math", Date: day,
			StudentLastName: "Doe", StudentFirstName: "Jane", DocID: "1001", AttendanceStatus: models.Present,
		}}}},
		{models.ProgramOutcomesReport, ProgramOutcomesReportData{
			Data:          []models.ProgramOutcomesReportRow{{ProgramName: "GED", IsActive: true, TotalClasses: 1}},
			Classes:       []models.ProgramClassBreakdownRow{{ClassName: "Math", Status: "Active", Capacity: 10, ActiveEnrollments: 4}},
			IncludeStatus: true,
		}},
		{models.FacilityComparisonReport, FacilityComparisonReportData{Data: []models.FacilityComparisonReportRow{{FacilityName: "North"}}}},
		{models.ClassRosterReport, ClassRosterReportData{
			Data:                    []models.ClassRosterReportRow{{NameLast: "Doe", NameFirst: "Jane", DocID: "1001", EnrollmentStatus: "Enrolled"}},
			IncludeIncompleteReason: true, IncludeAttendanceRate: true, IncludeEnrollmentDates: true,
		}},
		{models.ResidentProfileReport, ResidentProfileReportData{Data: []models.ResidentProfileReportRow{{NameLast: "Doe", NameFirst: "Jane", FacilityName: "North", ProgramName: "GED", ClassName: "Math"}}}},
	} {
		t.Run(string(tc.typ), func(t *testing.T) {
			req := &models.ReportGenerateRequest{Type: tc.typ, Format: models.FormatPDF, StartDate: day, EndDate: day.AddDate(0, 1, 0), ClassStatus: &status}
			w := httptest.NewRecorder()
			require.NoError(t, srv.exportReport(w, tc.report, models.FormatPDF, req, "North", "Doe, Jane"))
			require.Equal(t, http.StatusOK, w.Code)
			require.True(t, bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF-")), "not a PDF: %.40q", w.Body.String())
		})
	}
}
