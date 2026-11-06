package handlers

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/evertonvps/go-jasper"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// The PDF reports, rendered by JasperStarter from the templates the image
// copies to /templates. The code is core's, unchanged.

// logo is the Unlocked logo the reports show in their header.
//
//go:embed assets/ul-symbol-k.png
var logo []byte

// JasperReports renders the reports' PDFs with JasperStarter.
type JasperReports struct{}

// PDF renders report with the named template in JASPER_TEMPLATE_DIR
// (/templates by default), writing its working files to JASPER_TEMP_DIR.
func (JasperReports) PDF(templateName string, config pdfReport) ([]byte, error) {
	type ReportData struct {
		Rows    [][]string `json:"rows"`
		SubRows [][]string `json:"subrows"`
	}

	rows := config.Rows
	if len(rows) == 0 {
		rows = [][]string{}
	}

	subRows := config.SubRows
	if subRows == nil {
		subRows = [][]string{}
	}

	reportData := ReportData{
		Rows:    rows,
		SubRows: subRows,
	}

	jsonData, err := json.Marshal(reportData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal report data: %w", err)
	}

	// IMPORTANT: The go-jasper library wraps string parameter values in quotes for command-line safety.
	// See: https://github.com/evertonvps/go-jasper/blob/main/go-jasper.go#L88
	//
	// This causes JasperReports to receive parameter values like "\"Attendance\"" instead of "Attendance".
	// We CANNOT strip quotes here because the backend values don't have quotes yet!
	// The quotes are added by go-jasper when generating command-line args.
	//
	// Solution: JRXML templates must use .replaceAll("\"", "") to strip quotes at display time.
	// Future developers: ALL command-line parameters (ReportTitle, GeneratedDate, FilterLabelN, FilterValueN, etc.)
	// MUST include .replaceAll("\"", "") in their JRXML template fields.
	// Table data from JSON does NOT need quote stripping (not passed as command-line args).
	title := config.Title
	if title == "" {
		title = "Report"
	}

	// Build individual filter label/value parameters for proper spacing in JRXML.
	// Each filter line is passed as FilterLabelN and FilterValueN pairs.
	filterCount := len(config.Filters)
	maxFilters := 6

	params := []jasper.Parameter{
		{Key: "ReportTitle", Value: title},
		{Key: "GeneratedDate", Value: time.Now().Format("January 2, 2006 at 3:04 PM")},
		{Key: "LogoImage", Value: base64.StdEncoding.EncodeToString(logo)},
		{Key: "FilterCount", Value: fmt.Sprintf("%d", filterCount)},
	}

	// Only pass SubRowCount when the report actually has a sub-table. JasperStarter
	// errors on any parameter a template doesn't declare, and only templates with a
	// sub-table (e.g. program_outcomes breakdown) declare SubRowCount.
	if len(config.SubRows) > 0 {
		params = append(params, jasper.Parameter{Key: "SubRowCount", Value: fmt.Sprintf("%d", len(config.SubRows))})
	}

	// Report-specific template parameters (e.g. class-roster column visibility
	// flags). Only set by reports whose template declares them.
	for k, v := range config.Params {
		params = append(params, jasper.Parameter{Key: k, Value: v})
	}

	for i := 0; i < filterCount && i < maxFilters; i++ {
		params = append(params,
			jasper.Parameter{Key: fmt.Sprintf("FilterLabel%d", i+1), Value: config.Filters[i].Label},
			jasper.Parameter{Key: fmt.Sprintf("FilterValue%d", i+1), Value: config.Filters[i].Value},
		)
	}

	tempDir := os.Getenv("JASPER_TEMP_DIR")
	if tempDir == "" {
		tempDir = filepath.Join(os.TempDir(), "jasper-reports")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create temp directory: %w", err)
		}
		logrus.WithField("temp_dir", tempDir).Info("JASPER_TEMP_DIR not set, using fallback directory")
	}

	outputFileName := fmt.Sprintf("%s_%s", templateName, uuid.New().String())
	outputFilePath := filepath.Join(tempDir, outputFileName)
	jsonFileName := fmt.Sprintf("%s_%s.json", templateName, uuid.New().String())
	jsonFilePath := filepath.Join(tempDir, jsonFileName)

	if err := os.WriteFile(jsonFilePath, jsonData, 0600); err != nil {
		return nil, fmt.Errorf("failed to write temporary data file: %w", err)
	}

	defer func() {
		if err := os.Remove(jsonFilePath); err != nil {
			logrus.WithFields(logrus.Fields{
				"json_file": jsonFilePath,
				"error":     err,
			}).Warn("Failed to remove temporary JSON file")
		}

		if err := os.Remove(outputFilePath + ".pdf"); err != nil {
			logrus.WithFields(logrus.Fields{
				"pdf_file": outputFilePath + ".pdf",
				"error":    err,
			}).Warn("Failed to remove temporary PDF file")
		}
	}()

	gj := jasper.NewGoJasperJsonData(jsonFilePath, "", params, "pdf", outputFilePath)
	gj.Output = outputFilePath

	if path, err := exec.LookPath("jasperstarter"); err == nil {
		gj.Executable = path
	} else {
		logrus.Info("jasperstarter not found in PATH, falling back to default path")
		gj.Executable = "/opt/jasperstarter/bin/jasperstarter"
	}

	templateDir := os.Getenv("JASPER_TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "/templates"
	}
	compiledTemplatePath := filepath.Join(templateDir, templateName+".jasper")
	jrxmlTemplatePath := filepath.Join(templateDir, templateName+".jrxml")

	pdfBytes, err := gj.Process(compiledTemplatePath)
	if err != nil {
		// Check if this is a Jaspersoft Studio compilation error (NoClassDefFoundError with UUID suffix)
		// If so, recompile from .jrxml source and retry
		errStr := err.Error()
		if strings.Contains(errStr, "NoClassDefFoundError") && strings.Contains(errStr, "_") {
			logrus.WithFields(logrus.Fields{
				"template":   templateName,
				"error":      err,
				"jrxml_path": jrxmlTemplatePath,
			}).Warn("Detected Jaspersoft Studio-compiled .jasper file (UUID suffix), recompiling with JasperStarter")

			if recompileErr := recompileTemplate(templateDir, templateName); recompileErr != nil {
				return nil, fmt.Errorf("failed to recover from Studio compilation: %w (original error: %w)", recompileErr, err)
			}

			pdfBytes, err = gj.Process(compiledTemplatePath)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"template_path": compiledTemplatePath,
					"template_dir":  templateDir,
					"error":         err,
				}).Error("Failed to process compiled template after recompilation")
				return nil, fmt.Errorf("failed to process template after recompilation: %w", err)
			}

			logrus.WithFields(logrus.Fields{
				"template": templateName,
			}).Info("Successfully recovered from Studio compilation and generated PDF")
		} else {
			logrus.WithFields(logrus.Fields{
				"template_path": compiledTemplatePath,
				"template_dir":  templateDir,
				"error":         err,
			}).Error("Failed to process compiled template")
			return nil, fmt.Errorf("failed to process template: %w", err)
		}
	}

	return pdfBytes, nil
}

// recompileTemplate compiles a .jrxml file using JasperStarter
// This is called when a .jasper file is missing or corrupt (e.g., Studio-compiled with UUID suffix)
func recompileTemplate(templateDir, baseTemplateName string) error {
	jrxmlPath := filepath.Join(templateDir, baseTemplateName+".jrxml")

	logrus.WithFields(logrus.Fields{
		"template": baseTemplateName,
	}).Info("Recompiling template from .jrxml source with JasperStarter")

	cmd := exec.Command("/opt/jasperstarter/bin/jasperstarter", "compile", jrxmlPath)
	cmd.Dir = templateDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"template": baseTemplateName,
			"error":    err,
			"output":   string(output),
		}).Error("Failed to recompile template")
		return fmt.Errorf("failed to recompile template %s: %w", baseTemplateName, err)
	}

	logrus.WithFields(logrus.Fields{
		"template": baseTemplateName,
		"output":   string(output),
	}).Info("Successfully recompiled template from .jrxml source")

	return nil
}
