package model

import "time"

// ImportJob tracks a bulk import from upload through preview to commit or rollback.
// The service layer guarantees full rollback if any row-level error occurs.
type ImportJob struct {
	ID                           string     `json:"id"`
	BranchID                     string     `json:"branch_id"`
	ImportType                   string     `json:"import_type"` // readers, holdings, copies, programs, enrollments
	Status                       string     `json:"status"`      // uploaded, previewing, preview_ready, committed, rolled_back, failed
	FileName                     string     `json:"file_name"`
	FilePath                     *string    `json:"-"` // internal path; not exposed in API responses
	RowCount                     *int       `json:"row_count,omitempty"`
	ErrorCount                   int        `json:"error_count"`
	ErrorSummary                 any        `json:"error_summary,omitempty"` // []{ row, field, message }
	ValidRowCount                int        `json:"valid_row_count"`
	InvalidRowCount              int        `json:"invalid_row_count"`
	CompletenessPercent          float64    `json:"completeness_percent"`
	CompletenessThresholdPercent float64    `json:"completeness_threshold_percent"`
	MeetsCompletenessThreshold   bool       `json:"meets_completeness_threshold"`
	UploadedBy                   string     `json:"uploaded_by"`
	UploadedAt                   time.Time  `json:"uploaded_at"`
	CommittedAt                  *time.Time `json:"committed_at,omitempty"`
	RolledBackAt                 *time.Time `json:"rolled_back_at,omitempty"`
	WorkstationID                *string    `json:"workstation_id,omitempty"`
}

// ImportRow is a staging record for one row of an import file.
type ImportRow struct {
	ID           string    `json:"id"`
	JobID        string    `json:"job_id"`
	RowNumber    int       `json:"row_number"`
	RawData      any       `json:"raw_data"`              // original parsed values
	ParsedData   any       `json:"parsed_data,omitempty"` // validated/normalised values
	Status       string    `json:"status"`                // pending, valid, invalid, committed, rolled_back
	ErrorDetails *string   `json:"error_details,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
