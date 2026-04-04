package model

import "time"

// ReaderStatus is a lookup value for reader account state.
type ReaderStatus struct {
	Code             string `json:"code"`
	Description      string `json:"description"`
	AllowsBorrowing  bool   `json:"allows_borrowing"`
	AllowsEnrollment bool   `json:"allows_enrollment"`
}

// Reader is a library patron profile.
//
// Sensitive field encryption:
//   - NationalIDEnc, ContactEmailEnc, ContactPhoneEnc, DateOfBirthEnc are
//     AES-256-GCM ciphertext (base64-encoded) stored in the database.
//   - The application layer encrypts before write and decrypts after read.
//   - API responses mask these fields as "••••••" unless the caller has
//     the 'readers:reveal_sensitive' permission AND passes a step-up check.
//   - The plain-text counterparts (NationalID etc.) are populated only after
//     a successful decrypt; they are never stored in the database.
type Reader struct {
	ID            string    `json:"id"`
	BranchID      string    `json:"branch_id"`
	ReaderNumber  string    `json:"reader_number"`
	StatusCode    string    `json:"status_code"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	PreferredName *string   `json:"preferred_name,omitempty"`
	Notes         *string   `json:"notes,omitempty"`
	RegisteredAt  time.Time `json:"registered_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CreatedBy     *string   `json:"created_by,omitempty"`

	// Encrypted storage columns (AES-256-GCM ciphertext).
	// These are populated from the database row but never sent in API responses.
	NationalIDEnc    *string `json:"-"`
	ContactEmailEnc  *string `json:"-"`
	ContactPhoneEnc  *string `json:"-"`
	DateOfBirthEnc   *string `json:"-"`
}

// SensitiveFields carries the decrypted plaintext values for a reader.
// Populated only after a successful step-up authentication.
// Never stored in the database; only used for in-memory transport.
type SensitiveFields struct {
	NationalID   *string `json:"national_id,omitempty"`
	ContactEmail *string `json:"contact_email,omitempty"`
	ContactPhone *string `json:"contact_phone,omitempty"`
	DateOfBirth  *string `json:"date_of_birth,omitempty"`
}

// MaskedSensitiveFields returns a SensitiveFields with all values masked.
// Used in normal API responses for callers without reveal permission.
func MaskedSensitiveFields(r *Reader) *SensitiveFields {
	mask := func(enc *string) *string {
		if enc == nil {
			return nil
		}
		s := "••••••"
		return &s
	}
	return &SensitiveFields{
		NationalID:   mask(r.NationalIDEnc),
		ContactEmail: mask(r.ContactEmailEnc),
		ContactPhone: mask(r.ContactPhoneEnc),
		DateOfBirth:  mask(r.DateOfBirthEnc),
	}
}
