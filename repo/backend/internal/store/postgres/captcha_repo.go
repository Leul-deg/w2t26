package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/model"
)

// CaptchaRepo implements users.CaptchaRepository against the lms schema.
type CaptchaRepo struct {
	pool *pgxpool.Pool
}

// NewCaptchaRepo creates a new CaptchaRepo backed by the given connection pool.
func NewCaptchaRepo(pool *pgxpool.Pool) *CaptchaRepo {
	return &CaptchaRepo{pool: pool}
}

// Create stores a new CAPTCHA challenge.
func (r *CaptchaRepo) Create(ctx context.Context, c *model.CaptchaChallenge) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO lms.captcha_challenges
		 (challenge_key, answer_hash, username, ip_address, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id::text, created_at`,
		c.ChallengeKey, c.AnswerHash, c.Username, c.IPAddress, c.ExpiresAt,
	).Scan(&c.ID, &c.CreatedAt)
	return err
}

// GetByKey returns the challenge for the given key if not used and not expired.
// Returns apperr.NotFound if no matching challenge exists.
func (r *CaptchaRepo) GetByKey(ctx context.Context, key string) (*model.CaptchaChallenge, error) {
	c := &model.CaptchaChallenge{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, challenge_key, answer_hash, username, ip_address::text, created_at, expires_at, used
		 FROM lms.captcha_challenges
		 WHERE challenge_key = $1 AND used = false AND expires_at > NOW()`,
		key,
	).Scan(
		&c.ID, &c.ChallengeKey, &c.AnswerHash, &c.Username, &c.IPAddress,
		&c.CreatedAt, &c.ExpiresAt, &c.Used,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "captcha_challenge"}
		}
		return nil, err
	}
	return c, nil
}

// MarkUsed marks the challenge as used. Must be called even on wrong answers
// to prevent brute-force replay attacks.
func (r *CaptchaRepo) MarkUsed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lms.captcha_challenges SET used = true WHERE id = $1`,
		id,
	)
	return err
}

// PruneExpired deletes expired and already-used challenges.
// Returns the number of rows deleted.
func (r *CaptchaRepo) PruneExpired(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM lms.captcha_challenges
		 WHERE used = true OR expires_at < NOW()`,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
