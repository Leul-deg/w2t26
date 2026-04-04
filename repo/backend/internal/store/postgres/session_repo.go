package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/model"
)

// SessionRepo implements users.SessionRepository against the lms schema.
type SessionRepo struct {
	pool *pgxpool.Pool
}

// NewSessionRepo creates a new SessionRepo backed by the given connection pool.
func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{pool: pool}
}

// Create stores a new session record.
func (r *SessionRepo) Create(ctx context.Context, s *model.Session) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO lms.sessions
		 (token_hash, user_id, workstation_id, ip_address, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id::text, created_at, last_active_at`,
		s.TokenHash, s.UserID, s.WorkstationID, s.IPAddress, s.ExpiresAt,
	).Scan(&s.ID, &s.CreatedAt, &s.LastActiveAt)
	return err
}

// GetByTokenHash returns the session matching the token hash if valid and not expired.
// Returns apperr.NotFound if no matching active, unexpired session exists.
func (r *SessionRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error) {
	s := &model.Session{}
	err := r.pool.QueryRow(ctx,
		`SELECT id::text, token_hash, user_id::text, workstation_id, ip_address::text,
		        created_at, last_active_at, expires_at, is_valid, stepup_at
		 FROM lms.sessions
		 WHERE token_hash = $1 AND is_valid = true AND expires_at > NOW()`,
		tokenHash,
	).Scan(
		&s.ID, &s.TokenHash, &s.UserID, &s.WorkstationID, &s.IPAddress,
		&s.CreatedAt, &s.LastActiveAt, &s.ExpiresAt, &s.IsValid, &s.StepUpAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "session"}
		}
		return nil, err
	}
	return s, nil
}

// SetStepUp records the current UTC time as the step-up timestamp on the session.
func (r *SessionRepo) SetStepUp(ctx context.Context, sessionID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lms.sessions SET stepup_at = NOW() WHERE id = $1`,
		sessionID,
	)
	return err
}

// Touch updates last_active_at to now and sets a new expires_at.
func (r *SessionRepo) Touch(ctx context.Context, id string, newExpiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lms.sessions
		 SET last_active_at = NOW(), expires_at = $1
		 WHERE id = $2`,
		newExpiresAt, id,
	)
	return err
}

// Invalidate marks a session as is_valid = false (logout or forced expiry).
func (r *SessionRepo) Invalidate(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lms.sessions SET is_valid = false WHERE id = $1`,
		id,
	)
	return err
}

// InvalidateAll invalidates all active sessions for the given user.
func (r *SessionRepo) InvalidateAll(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lms.sessions SET is_valid = false WHERE user_id = $1 AND is_valid = true`,
		userID,
	)
	return err
}

// PruneExpired deletes sessions that are invalid or have been expired for more than 1 day.
// Returns the number of rows deleted.
func (r *SessionRepo) PruneExpired(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM lms.sessions
		 WHERE is_valid = false OR expires_at < NOW() - interval '1 day'`,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
