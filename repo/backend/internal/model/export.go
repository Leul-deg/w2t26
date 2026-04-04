package model

import "time"

// ExportJob is the audited record of a data export operation.
// Every export generates one ExportJob row, regardless of size.
// The row is created before file generation; RowCount and FileName are
// populated after the file is written.
type ExportJob struct {
	ID             string    `json:"id"`
	BranchID       string    `json:"branch_id"`
	ExportType     string    `json:"export_type"` // readers, holdings, copies, circulation, programs, enrollments, audit_events, report
	FiltersApplied any       `json:"filters_applied,omitempty"`
	RowCount       *int      `json:"row_count,omitempty"`
	FileName       *string   `json:"file_name,omitempty"`
	ExportedBy     string    `json:"exported_by"`
	ExportedAt     time.Time `json:"exported_at"`
	WorkstationID  *string   `json:"workstation_id,omitempty"`
}
