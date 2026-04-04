package model

import "time"

// Program is a scheduled library event with defined capacity and enrollment window.
// venue_type is the reporting analog of "room_type".
// enrollment_channel is the reporting analog of "channel".
type Program struct {
	ID                  string     `json:"id"`
	BranchID            string     `json:"branch_id"`
	Title               string     `json:"title"`
	Description         *string    `json:"description,omitempty"`
	Category            *string    `json:"category,omitempty"`
	VenueType           *string    `json:"venue_type,omitempty"`   // reporting alias: room_type
	VenueName           *string    `json:"venue_name,omitempty"`
	Capacity            int        `json:"capacity"`
	EnrollmentOpensAt   *time.Time `json:"enrollment_opens_at,omitempty"`
	EnrollmentClosesAt  *time.Time `json:"enrollment_closes_at,omitempty"`
	StartsAt            time.Time  `json:"starts_at"`
	EndsAt              time.Time  `json:"ends_at"`
	Status              string     `json:"status"` // draft, published, cancelled, completed
	EnrollmentChannel   string     `json:"enrollment_channel"` // reporting alias: channel
	CreatedBy           *string    `json:"created_by,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// ProgramPrerequisite requires a reader to have completed another program.
type ProgramPrerequisite struct {
	ID                string    `json:"id"`
	ProgramID         string    `json:"program_id"`
	RequiredProgramID string    `json:"required_program_id"`
	Description       *string   `json:"description,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// EnrollmentRule is a whitelist or blacklist rule for program enrollment.
type EnrollmentRule struct {
	ID         string    `json:"id"`
	ProgramID  string    `json:"program_id"`
	RuleType   string    `json:"rule_type"`   // whitelist, blacklist
	MatchField string    `json:"match_field"` // e.g. status_code, branch_id
	MatchValue string    `json:"match_value"`
	Reason     *string   `json:"reason,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Enrollment is a reader's registration in a program.
// The (program_id, reader_id) pair is unique.
type Enrollment struct {
	ID                string    `json:"id"`
	ProgramID         string    `json:"program_id"`
	ReaderID          string    `json:"reader_id"`
	BranchID          string    `json:"branch_id"`
	Status            string    `json:"status"` // pending, confirmed, waitlisted, cancelled, completed, no_show
	EnrollmentChannel *string   `json:"enrollment_channel,omitempty"` // reporting alias: channel
	WaitlistPosition  *int      `json:"waitlist_position,omitempty"`
	EnrolledAt        time.Time `json:"enrolled_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	EnrolledBy        *string   `json:"enrolled_by,omitempty"`
	// RemainingSeats is computed at enrollment time and included in the response
	// for immediate feedback. It is not persisted and will be nil on fetch.
	RemainingSeats    *int      `json:"remaining_seats,omitempty"`
}

// EnrollmentHistory records each status transition on an enrollment.
type EnrollmentHistory struct {
	ID             string    `json:"id"`
	EnrollmentID   string    `json:"enrollment_id"`
	PreviousStatus string    `json:"previous_status"`
	NewStatus      string    `json:"new_status"`
	ChangedBy      *string   `json:"changed_by,omitempty"`
	Reason         *string   `json:"reason,omitempty"`
	ChangedAt      time.Time `json:"changed_at"`
	WorkstationID  *string   `json:"workstation_id,omitempty"`
}
