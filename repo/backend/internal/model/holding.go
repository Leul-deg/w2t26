package model

import "time"

// Holding is a title-level bibliographic record.
// Each physical item is a separate Copy record.
type Holding struct {
	ID              string    `json:"id"`
	BranchID        string    `json:"branch_id"`
	Title           string    `json:"title"`
	Author          *string   `json:"author,omitempty"`
	ISBN            *string   `json:"isbn,omitempty"`
	Publisher       *string   `json:"publisher,omitempty"`
	PublicationYear *int      `json:"publication_year,omitempty"`
	Category        *string   `json:"category,omitempty"`
	Subcategory     *string   `json:"subcategory,omitempty"`
	Language        string    `json:"language"`
	Description     *string   `json:"description,omitempty"`
	CoverImagePath  *string   `json:"cover_image_path,omitempty"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedBy       *string   `json:"created_by,omitempty"`
}

// CopyStatus is a lookup value for the physical state of a copy.
type CopyStatus struct {
	Code         string `json:"code"`
	Description  string `json:"description"`
	IsBorrowable bool   `json:"is_borrowable"`
}

// Copy is a single physical item that belongs to a Holding.
// Each copy has a globally unique barcode.
type Copy struct {
	ID            string    `json:"id"`
	HoldingID     string    `json:"holding_id"`
	BranchID      string    `json:"branch_id"`
	Barcode       string    `json:"barcode"`
	StatusCode    string    `json:"status_code"`
	Condition     string    `json:"condition"` // new, good, fair, poor, damaged
	ShelfLocation *string   `json:"shelf_location,omitempty"`
	AcquiredAt    *string   `json:"acquired_at,omitempty"` // DATE stored as string (YYYY-MM-DD)
	WithdrawnAt   *string   `json:"withdrawn_at,omitempty"`
	PricePaid     *float64  `json:"price_paid,omitempty"`
	Notes         *string   `json:"notes,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
