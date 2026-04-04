package readers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	auditpkg "lms/internal/audit"
	"lms/internal/apperr"
	"lms/internal/crypto"
	"lms/internal/model"
)

// Service implements reader management business logic including
// field-level encryption of sensitive reader attributes.
type Service struct {
	repo          Repository
	encryptionKey []byte // nil = encryption disabled (dev/test only)
	auditLogger   *auditpkg.Logger
}

// NewService creates a new readers Service.
// encryptionKey may be nil when CRYPTO_KEY_FILE is not configured;
// in that case sensitive fields are stored as plaintext (dev only).
func NewService(repo Repository, encryptionKey []byte, auditLogger *auditpkg.Logger) *Service {
	return &Service{
		repo:          repo,
		encryptionKey: encryptionKey,
		auditLogger:   auditLogger,
	}
}

// CreateRequest carries the input for creating a new reader.
type CreateRequest struct {
	BranchID      string
	ReaderNumber  string  // optional — generated if empty
	FirstName     string
	LastName      string
	PreferredName *string
	Notes         *string
	// Sensitive — will be encrypted before storage
	NationalID   *string
	ContactEmail *string
	ContactPhone *string
	DateOfBirth  *string
	// Actor is the staff member performing the operation (for audit / created_by)
	ActorUserID string
}

// UpdateRequest carries fields that may be changed on an existing reader.
type UpdateRequest struct {
	FirstName     string
	LastName      string
	PreferredName *string
	Notes         *string
	// Sensitive — will be encrypted if non-nil; nil means "leave unchanged"
	NationalID   *string
	ContactEmail *string
	ContactPhone *string
	DateOfBirth  *string
}

// Create validates, encrypts sensitive fields, and inserts a new reader.
func (s *Service) Create(ctx context.Context, req CreateRequest) (*model.Reader, error) {
	if req.FirstName == "" {
		return nil, &apperr.Validation{Field: "first_name", Message: "first name is required"}
	}
	if req.LastName == "" {
		return nil, &apperr.Validation{Field: "last_name", Message: "last name is required"}
	}
	if req.BranchID == "" {
		return nil, &apperr.Validation{Field: "branch_id", Message: "branch is required"}
	}

	number := req.ReaderNumber
	if number == "" {
		var err error
		number, err = generateReaderNumber()
		if err != nil {
			return nil, &apperr.Internal{Cause: err}
		}
	}

	r := &model.Reader{
		BranchID:      req.BranchID,
		ReaderNumber:  number,
		StatusCode:    "active",
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		PreferredName: req.PreferredName,
		Notes:         req.Notes,
		RegisteredAt:  time.Now().UTC(),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if req.ActorUserID != "" {
		r.CreatedBy = &req.ActorUserID
	}

	if err := s.encryptSensitive(r, req.NationalID, req.ContactEmail, req.ContactPhone, req.DateOfBirth); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// GetByID fetches a reader by ID within the caller's branch scope.
func (s *Service) GetByID(ctx context.Context, id, branchID string) (*model.Reader, error) {
	return s.repo.GetByID(ctx, id, branchID)
}

// GetByReaderNumber fetches a reader by membership card number.
func (s *Service) GetByReaderNumber(ctx context.Context, number, branchID string) (*model.Reader, error) {
	return s.repo.GetByReaderNumber(ctx, number, branchID)
}

// Update applies changes to a reader's non-status fields.
// Sensitive fields that are non-nil in the request are re-encrypted and saved.
// Sensitive fields that are nil are left unchanged (preserving existing ciphertext).
func (s *Service) Update(ctx context.Context, id, branchID string, req UpdateRequest) (*model.Reader, error) {
	if req.FirstName == "" {
		return nil, &apperr.Validation{Field: "first_name", Message: "first name is required"}
	}
	if req.LastName == "" {
		return nil, &apperr.Validation{Field: "last_name", Message: "last name is required"}
	}

	existing, err := s.repo.GetByID(ctx, id, branchID)
	if err != nil {
		return nil, err
	}

	existing.FirstName = req.FirstName
	existing.LastName = req.LastName
	existing.PreferredName = req.PreferredName
	existing.Notes = req.Notes
	existing.UpdatedAt = time.Now().UTC()

	// Only re-encrypt fields that were explicitly provided in the request.
	// Use the existing ciphertext for fields where the caller passed nil.
	natID := req.NationalID
	email := req.ContactEmail
	phone := req.ContactPhone
	dob := req.DateOfBirth

	if natID != nil || email != nil || phone != nil || dob != nil {
		// For unchanged fields, keep existing ciphertext by passing nil to encryptSensitive.
		if err := s.encryptSensitivePartial(existing, natID, email, phone, dob); err != nil {
			return nil, err
		}
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

// UpdateStatus transitions the reader to a new status.
func (s *Service) UpdateStatus(ctx context.Context, id, branchID, actorID, statusCode string) error {
	validStatuses := map[string]bool{
		"active":               true,
		"frozen":               true,
		"blacklisted":          true,
		"pending_verification": true,
	}
	if !validStatuses[statusCode] {
		return &apperr.Validation{Field: "status_code", Message: "invalid status code"}
	}

	if err := s.repo.UpdateStatus(ctx, id, branchID, statusCode); err != nil {
		return err
	}
	return nil
}

// List returns a paginated, filtered list of readers for the branch.
func (s *Service) List(ctx context.Context, branchID string, filter ReaderFilter, p model.Pagination) (model.PageResult[*model.Reader], error) {
	return s.repo.List(ctx, branchID, filter, p)
}

// ListStatuses returns all reader status lookup values.
func (s *Service) ListStatuses(ctx context.Context) ([]*model.ReaderStatus, error) {
	return s.repo.ListStatuses(ctx)
}

// RevealSensitive decrypts and returns the reader's sensitive fields.
// Caller must have already verified step-up authentication.
// An audit event is logged regardless of whether decryption succeeds.
func (s *Service) RevealSensitive(ctx context.Context, readerID, branchID string) (*model.SensitiveFields, error) {
	r, err := s.repo.GetByID(ctx, readerID, branchID)
	if err != nil {
		return nil, err
	}

	if s.encryptionKey == nil {
		// Encryption disabled — return whatever was stored as plaintext.
		return &model.SensitiveFields{
			NationalID:   r.NationalIDEnc,
			ContactEmail: r.ContactEmailEnc,
			ContactPhone: r.ContactPhoneEnc,
			DateOfBirth:  r.DateOfBirthEnc,
		}, nil
	}

	sf := &model.SensitiveFields{}
	sf.NationalID, err = decryptField(s.encryptionKey, r.NationalIDEnc)
	if err != nil {
		return nil, &apperr.Internal{Cause: fmt.Errorf("decrypt national_id: %w", err)}
	}
	sf.ContactEmail, err = decryptField(s.encryptionKey, r.ContactEmailEnc)
	if err != nil {
		return nil, &apperr.Internal{Cause: fmt.Errorf("decrypt contact_email: %w", err)}
	}
	sf.ContactPhone, err = decryptField(s.encryptionKey, r.ContactPhoneEnc)
	if err != nil {
		return nil, &apperr.Internal{Cause: fmt.Errorf("decrypt contact_phone: %w", err)}
	}
	sf.DateOfBirth, err = decryptField(s.encryptionKey, r.DateOfBirthEnc)
	if err != nil {
		return nil, &apperr.Internal{Cause: fmt.Errorf("decrypt date_of_birth: %w", err)}
	}

	return sf, nil
}

// GetLoanHistory returns paginated loan history for the reader.
func (s *Service) GetLoanHistory(ctx context.Context, readerID, branchID string, p model.Pagination) (model.PageResult[*LoanHistoryItem], error) {
	return s.repo.GetLoanHistory(ctx, readerID, branchID, p)
}

// GetCurrentHoldings returns the reader's currently checked-out items.
func (s *Service) GetCurrentHoldings(ctx context.Context, readerID, branchID string) ([]*LoanHistoryItem, error) {
	return s.repo.GetCurrentHoldings(ctx, readerID, branchID)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// encryptSensitive encrypts each non-nil sensitive field and sets the
// corresponding *Enc field on the reader. Nil values produce nil ciphertext.
func (s *Service) encryptSensitive(r *model.Reader, natID, email, phone, dob *string) error {
	var err error
	r.NationalIDEnc, err = encryptField(s.encryptionKey, natID)
	if err != nil {
		return &apperr.Internal{Cause: fmt.Errorf("encrypt national_id: %w", err)}
	}
	r.ContactEmailEnc, err = encryptField(s.encryptionKey, email)
	if err != nil {
		return &apperr.Internal{Cause: fmt.Errorf("encrypt contact_email: %w", err)}
	}
	r.ContactPhoneEnc, err = encryptField(s.encryptionKey, phone)
	if err != nil {
		return &apperr.Internal{Cause: fmt.Errorf("encrypt contact_phone: %w", err)}
	}
	r.DateOfBirthEnc, err = encryptField(s.encryptionKey, dob)
	if err != nil {
		return &apperr.Internal{Cause: fmt.Errorf("encrypt date_of_birth: %w", err)}
	}
	return nil
}

// encryptSensitivePartial encrypts non-nil fields; nil means "leave unchanged".
func (s *Service) encryptSensitivePartial(r *model.Reader, natID, email, phone, dob *string) error {
	var err error
	if natID != nil {
		r.NationalIDEnc, err = encryptField(s.encryptionKey, natID)
		if err != nil {
			return &apperr.Internal{Cause: fmt.Errorf("encrypt national_id: %w", err)}
		}
	}
	if email != nil {
		r.ContactEmailEnc, err = encryptField(s.encryptionKey, email)
		if err != nil {
			return &apperr.Internal{Cause: fmt.Errorf("encrypt contact_email: %w", err)}
		}
	}
	if phone != nil {
		r.ContactPhoneEnc, err = encryptField(s.encryptionKey, phone)
		if err != nil {
			return &apperr.Internal{Cause: fmt.Errorf("encrypt contact_phone: %w", err)}
		}
	}
	if dob != nil {
		r.DateOfBirthEnc, err = encryptField(s.encryptionKey, dob)
		if err != nil {
			return &apperr.Internal{Cause: fmt.Errorf("encrypt date_of_birth: %w", err)}
		}
	}
	return nil
}

// encryptField encrypts a plaintext string using AES-256-GCM.
// Returns nil if the input is nil (field not provided).
// If the encryption key is nil, stores plaintext (dev/test only).
func encryptField(key []byte, plaintext *string) (*string, error) {
	if plaintext == nil {
		return nil, nil
	}
	if key == nil {
		// No encryption key — store as plaintext. Not for production.
		return plaintext, nil
	}
	ct, err := crypto.Encrypt(key, *plaintext)
	if err != nil {
		return nil, err
	}
	return &ct, nil
}

// decryptField decrypts a ciphertext string using AES-256-GCM.
// Returns nil if the input is nil.
func decryptField(key []byte, ciphertext *string) (*string, error) {
	if ciphertext == nil {
		return nil, nil
	}
	if key == nil {
		// No encryption key — value was stored as plaintext.
		return ciphertext, nil
	}
	pt, err := crypto.Decrypt(key, *ciphertext)
	if err != nil {
		return nil, err
	}
	return &pt, nil
}

// generateReaderNumber produces a unique reader card number in the format
// RN-YYYYMMDD-<6-hex-chars>. Randomness makes collisions negligible.
func generateReaderNumber() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("RN-%s-%s", time.Now().UTC().Format("20060102"), hex.EncodeToString(b)), nil
}
