package report

import (
	"os"
	"path/filepath"
)

// ReportManager manages report storage
type ReportManager struct {
	reportDir string
}

// NewReportManager creates a new ReportManager
func NewReportManager(reportDir string) *ReportManager {
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		panic(err)
	}
	return &ReportManager{reportDir: reportDir}
}

// SaveReport saves a report to disk
func (rm *ReportManager) SaveReport(reportID, content string) error {
	filePath := filepath.Join(rm.reportDir, reportID+".json")
	return os.WriteFile(filePath, []byte(content), 0644)
}

// GetReportPath returns the file path for a report
func (rm *ReportManager) GetReportPath(reportID string) (string, bool) {
	filePath := filepath.Join(rm.reportDir, reportID+".json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", false
	}
	return filePath, true
}
