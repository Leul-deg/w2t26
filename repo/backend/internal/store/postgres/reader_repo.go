package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/readers"
	"lms/internal/model"
)

// ReaderRepo implements readers.Repository against the lms schema.
type ReaderRepo struct {
	pool *pgxpool.Pool
}

// NewReaderRepo creates a new ReaderRepo backed by the given connection pool.
func NewReaderRepo(pool *pgxpool.Pool) *ReaderRepo {
	return &ReaderRepo{pool: pool}
}

// Create inserts a new reader. Returns apperr.Conflict if reader_number is taken.
func (r *ReaderRepo) Create(ctx context.Context, reader *model.Reader) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO lms.readers
		     (branch_id, reader_number, status_code, first_name, last_name,
		      preferred_name, national_id_enc, contact_email_enc, contact_phone_enc,
		      date_of_birth_enc, notes, registered_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id::text, registered_at, created_at, updated_at`,
		reader.BranchID, reader.ReaderNumber, reader.StatusCode,
		reader.FirstName, reader.LastName, reader.PreferredName,
		reader.NationalIDEnc, reader.ContactEmailEnc, reader.ContactPhoneEnc,
		reader.DateOfBirthEnc, reader.Notes, reader.RegisteredAt, reader.CreatedBy,
	).Scan(&reader.ID, &reader.RegisteredAt, &reader.CreatedAt, &reader.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return &apperr.Conflict{Resource: "reader", Message: "reader_number already in use"}
		}
		return err
	}
	return nil
}

// GetByID returns the reader with the given ID, optionally scoped to branchID.
func (r *ReaderRepo) GetByID(ctx context.Context, id, branchID string) (*model.Reader, error) {
	query := `
		SELECT id::text, branch_id::text, reader_number, status_code,
		       first_name, last_name, preferred_name,
		       national_id_enc, contact_email_enc, contact_phone_enc, date_of_birth_enc,
		       notes, registered_at, created_at, updated_at, created_by::text
		FROM lms.readers
		WHERE id = $1`

	var args []any
	args = append(args, id)

	if branchID != "" {
		query += " AND branch_id = $2"
		args = append(args, branchID)
	}

	reader := &model.Reader{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&reader.ID, &reader.BranchID, &reader.ReaderNumber, &reader.StatusCode,
		&reader.FirstName, &reader.LastName, &reader.PreferredName,
		&reader.NationalIDEnc, &reader.ContactEmailEnc, &reader.ContactPhoneEnc, &reader.DateOfBirthEnc,
		&reader.Notes, &reader.RegisteredAt, &reader.CreatedAt, &reader.UpdatedAt, &reader.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "reader", ID: id}
		}
		return nil, err
	}
	return reader, nil
}

// GetByReaderNumber returns the reader with the given card number, scoped to branchID.
func (r *ReaderRepo) GetByReaderNumber(ctx context.Context, number, branchID string) (*model.Reader, error) {
	query := `
		SELECT id::text, branch_id::text, reader_number, status_code,
		       first_name, last_name, preferred_name,
		       national_id_enc, contact_email_enc, contact_phone_enc, date_of_birth_enc,
		       notes, registered_at, created_at, updated_at, created_by::text
		FROM lms.readers
		WHERE reader_number = $1`

	var args []any
	args = append(args, number)

	if branchID != "" {
		query += " AND branch_id = $2"
		args = append(args, branchID)
	}

	reader := &model.Reader{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&reader.ID, &reader.BranchID, &reader.ReaderNumber, &reader.StatusCode,
		&reader.FirstName, &reader.LastName, &reader.PreferredName,
		&reader.NationalIDEnc, &reader.ContactEmailEnc, &reader.ContactPhoneEnc, &reader.DateOfBirthEnc,
		&reader.Notes, &reader.RegisteredAt, &reader.CreatedAt, &reader.UpdatedAt, &reader.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "reader", ID: number}
		}
		return nil, err
	}
	return reader, nil
}

// Update persists changes to an existing reader's fields.
func (r *ReaderRepo) Update(ctx context.Context, reader *model.Reader) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lms.readers
		 SET first_name=$1, last_name=$2, preferred_name=$3,
		     national_id_enc=$4, contact_email_enc=$5, contact_phone_enc=$6, date_of_birth_enc=$7,
		     notes=$8, updated_at=NOW()
		 WHERE id=$9`,
		reader.FirstName, reader.LastName, reader.PreferredName,
		reader.NationalIDEnc, reader.ContactEmailEnc, reader.ContactPhoneEnc, reader.DateOfBirthEnc,
		reader.Notes, reader.ID,
	)
	return err
}

// UpdateStatus changes the reader's status_code.
func (r *ReaderRepo) UpdateStatus(ctx context.Context, id, branchID, statusCode string) error {
	var n int64
	if branchID != "" {
		tag, err := r.pool.Exec(ctx,
			`UPDATE lms.readers SET status_code=$1, updated_at=NOW() WHERE id=$2 AND branch_id=$3`,
			statusCode, id, branchID,
		)
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
	} else {
		tag, err := r.pool.Exec(ctx,
			`UPDATE lms.readers SET status_code=$1, updated_at=NOW() WHERE id=$2`,
			statusCode, id,
		)
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
	}
	if n == 0 {
		return &apperr.NotFound{Resource: "reader", ID: id}
	}
	return nil
}

// List returns a paginated, filtered list of readers.
func (r *ReaderRepo) List(ctx context.Context, branchID string, filter readers.ReaderFilter, p model.Pagination) (model.PageResult[*model.Reader], error) {
	var whereClauses []string
	var args []any
	argIdx := 1

	if branchID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("branch_id = $%d", argIdx))
		args = append(args, branchID)
		argIdx++
	}
	if filter.StatusCode != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("status_code = $%d", argIdx))
		args = append(args, *filter.StatusCode)
		argIdx++
	}
	if filter.Search != nil && *filter.Search != "" {
		pattern := "%" + strings.ToLower(*filter.Search) + "%"
		whereClauses = append(whereClauses,
			fmt.Sprintf("(LOWER(first_name) LIKE $%d OR LOWER(last_name) LIKE $%d OR LOWER(reader_number) LIKE $%d)",
				argIdx, argIdx, argIdx),
		)
		args = append(args, pattern)
		argIdx++
	}

	where := ""
	if len(whereClauses) > 0 {
		where = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM lms.readers " + where
	dataQuery := fmt.Sprintf(`
		SELECT id::text, branch_id::text, reader_number, status_code,
		       first_name, last_name, preferred_name,
		       national_id_enc, contact_email_enc, contact_phone_enc, date_of_birth_enc,
		       notes, registered_at, created_at, updated_at, created_by::text
		FROM lms.readers
		%s
		ORDER BY last_name, first_name, id
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	dataArgs := append(args, p.Limit(), p.Offset())

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return model.PageResult[*model.Reader]{}, err
	}

	rows, err := r.pool.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return model.PageResult[*model.Reader]{}, err
	}
	defer rows.Close()

	var items []*model.Reader
	for rows.Next() {
		rd := &model.Reader{}
		if err := rows.Scan(
			&rd.ID, &rd.BranchID, &rd.ReaderNumber, &rd.StatusCode,
			&rd.FirstName, &rd.LastName, &rd.PreferredName,
			&rd.NationalIDEnc, &rd.ContactEmailEnc, &rd.ContactPhoneEnc, &rd.DateOfBirthEnc,
			&rd.Notes, &rd.RegisteredAt, &rd.CreatedAt, &rd.UpdatedAt, &rd.CreatedBy,
		); err != nil {
			return model.PageResult[*model.Reader]{}, err
		}
		items = append(items, rd)
	}
	if err := rows.Err(); err != nil {
		return model.PageResult[*model.Reader]{}, err
	}

	return model.NewPageResult(items, total, p), nil
}

// ListStatuses returns all reader status lookup rows.
func (r *ReaderRepo) ListStatuses(ctx context.Context) ([]*model.ReaderStatus, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT code, description, allows_borrowing, allows_enrollment FROM lms.reader_statuses ORDER BY code`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []*model.ReaderStatus
	for rows.Next() {
		s := &model.ReaderStatus{}
		if err := rows.Scan(&s.Code, &s.Description, &s.AllowsBorrowing, &s.AllowsEnrollment); err != nil {
			return nil, err
		}
		statuses = append(statuses, s)
	}
	return statuses, rows.Err()
}

// GetLoanHistory returns paginated circulation events for the given reader,
// joined with copy barcode and holding title/author.
func (r *ReaderRepo) GetLoanHistory(ctx context.Context, readerID, branchID string, p model.Pagination) (model.PageResult[*readers.LoanHistoryItem], error) {
	var whereClauses []string
	var args []any
	argIdx := 1

	whereClauses = append(whereClauses, fmt.Sprintf("ce.reader_id = $%d::uuid", argIdx))
	args = append(args, readerID)
	argIdx++

	if branchID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("ce.branch_id = $%d::uuid", argIdx))
		args = append(args, branchID)
		argIdx++
	}

	where := "WHERE " + strings.Join(whereClauses, " AND ")

	countQuery := `SELECT COUNT(*) FROM lms.circulation_events ce ` + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return model.PageResult[*readers.LoanHistoryItem]{}, err
	}

	dataArgs := append(args, p.Limit(), p.Offset())
	dataQuery := fmt.Sprintf(`
		SELECT ce.id::text, ce.copy_id::text,
		       c.barcode, h.title, h.author,
		       ce.event_type, ce.due_date::text, ce.returned_at, ce.created_at
		FROM lms.circulation_events ce
		JOIN lms.copies c ON c.id = ce.copy_id
		JOIN lms.holdings h ON h.id = c.holding_id
		%s
		ORDER BY ce.created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	rows, err := r.pool.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return model.PageResult[*readers.LoanHistoryItem]{}, err
	}
	defer rows.Close()

	var items []*readers.LoanHistoryItem
	for rows.Next() {
		item := &readers.LoanHistoryItem{}
		var returnedAt *time.Time
		if err := rows.Scan(
			&item.EventID, &item.CopyID,
			&item.Barcode, &item.Title, &item.Author,
			&item.EventType, &item.DueDate, &returnedAt, &item.CreatedAt,
		); err != nil {
			return model.PageResult[*readers.LoanHistoryItem]{}, err
		}
		item.ReturnedAt = returnedAt
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return model.PageResult[*readers.LoanHistoryItem]{}, err
	}

	return model.NewPageResult(items, total, p), nil
}

// GetCurrentHoldings returns the reader's currently checked-out copies.
func (r *ReaderRepo) GetCurrentHoldings(ctx context.Context, readerID, branchID string) ([]*readers.LoanHistoryItem, error) {
	var whereClauses []string
	var args []any
	argIdx := 1

	whereClauses = append(whereClauses,
		fmt.Sprintf("ce.reader_id = $%d::uuid", argIdx),
		"ce.event_type = 'checkout'",
		"ce.returned_at IS NULL",
		"c.status_code = 'checked_out'",
	)
	args = append(args, readerID)
	argIdx++

	if branchID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("ce.branch_id = $%d::uuid", argIdx))
		args = append(args, branchID)
	}

	where := "WHERE " + strings.Join(whereClauses, " AND ")

	query := fmt.Sprintf(`
		SELECT ce.id::text, ce.copy_id::text,
		       c.barcode, h.title, h.author,
		       ce.event_type, ce.due_date::text, ce.returned_at, ce.created_at
		FROM lms.circulation_events ce
		JOIN lms.copies c ON c.id = ce.copy_id
		JOIN lms.holdings h ON h.id = c.holding_id
		%s
		ORDER BY ce.due_date ASC NULLS LAST`, where)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*readers.LoanHistoryItem
	for rows.Next() {
		item := &readers.LoanHistoryItem{}
		var returnedAt *time.Time
		if err := rows.Scan(
			&item.EventID, &item.CopyID,
			&item.Barcode, &item.Title, &item.Author,
			&item.EventType, &item.DueDate, &returnedAt, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.ReturnedAt = returnedAt
		items = append(items, item)
	}
	return items, rows.Err()
}
