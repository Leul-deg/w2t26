package users

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"lms/internal/apperr"
	auditpkg "lms/internal/audit"
	"lms/internal/model"
)

// Lockout and CAPTCHA thresholds.
const (
	// CaptchaThreshold is the number of failed attempts before CAPTCHA is required.
	CaptchaThreshold = 3
	// LockoutThreshold is the number of failed attempts before the account is locked.
	LockoutThreshold = 5
	// LockoutDuration is how long an account is locked after reaching LockoutThreshold.
	LockoutDuration = 15 * time.Minute
	// CaptchaTTL is how long a CAPTCHA challenge remains valid.
	CaptchaTTL = 10 * time.Minute
)

// LoginResult is returned on successful authentication.
type LoginResult struct {
	SessionToken string          // raw token — must be set as cookie, never logged
	Session      *model.Session
	User         *model.UserWithRoles
}

// Service provides authentication operations for staff users.
type Service struct {
	userRepo    Repository
	sessionRepo SessionRepository
	captchaRepo CaptchaRepository
	auditLogger *auditpkg.Logger
	inactivitySeconds int
}

// NewService creates a new auth Service with all required dependencies.
func NewService(
	userRepo Repository,
	sessionRepo SessionRepository,
	captchaRepo CaptchaRepository,
	auditLogger *auditpkg.Logger,
	inactivitySeconds int,
) *Service {
	return &Service{
		userRepo:          userRepo,
		sessionRepo:       sessionRepo,
		captchaRepo:       captchaRepo,
		auditLogger:       auditLogger,
		inactivitySeconds: inactivitySeconds,
	}
}

// Login authenticates a user and creates a new session.
// It enforces lockout, CAPTCHA, and failed-attempt tracking.
// Generic error messages are used throughout to prevent user enumeration.
func (s *Service) Login(
	ctx context.Context,
	username, password, captchaKey, captchaAnswer,
	workstationID, ipAddr string,
) (*LoginResult, error) {
	// 1. Look up the user. On not-found, return generic error to prevent enumeration.
	u, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		// Do not reveal that the username does not exist.
		s.auditLogger.LogLoginFailed(ctx, username, workstationID, ipAddr, "user_not_found")
		return nil, &apperr.Validation{Message: "invalid credentials"}
	}

	// 2. If account is inactive, return generic error.
	if !u.IsActive {
		s.auditLogger.LogLoginFailed(ctx, username, workstationID, ipAddr, "account_inactive")
		return nil, &apperr.Validation{Message: "invalid credentials"}
	}

	// 3. If account is locked, return AccountLocked with remaining seconds.
	now := time.Now()
	if u.IsLocked(now) {
		remaining := int(u.LockedUntil.Sub(now).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		return nil, &apperr.AccountLocked{SecondsRemaining: remaining}
	}

	// 4. Check if CAPTCHA is required (>= 3 failed attempts and no captchaKey provided).
	if u.FailedAttempts >= CaptchaThreshold && captchaKey == "" {
		challenge, err := s.issueCaptcha(ctx, username, ipAddr)
		if err != nil {
			return nil, &apperr.Internal{Cause: err}
		}
		return nil, &apperr.CaptchaRequired{ChallengeKey: challenge.ChallengeKey}
	}

	// 5. If captchaKey is provided, validate it before checking the password.
	if captchaKey != "" {
		if err := s.validateCaptcha(ctx, captchaKey, captchaAnswer); err != nil {
			// Wrong CAPTCHA answer: increment failed attempts but don't return a
			// new CAPTCHA — let the caller retry from the beginning.
			newCount, _ := s.userRepo.IncrementFailedAttempts(ctx, u.ID)
			s.auditLogger.LogLoginFailed(ctx, username, workstationID, ipAddr, "captcha_wrong")
			if newCount >= LockoutThreshold {
				lockUntil := time.Now().Add(LockoutDuration)
				_ = s.userRepo.SetLockedUntil(ctx, u.ID, lockUntil)
				s.auditLogger.LogLockout(ctx, u.ID, u.Username)
				return nil, &apperr.AccountLocked{SecondsRemaining: int(LockoutDuration.Seconds())}
			}
			return nil, &apperr.Validation{Message: "invalid captcha answer"}
		}
	}

	// 6. Verify bcrypt password.
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		// Wrong password — increment failed attempts.
		newCount, incErr := s.userRepo.IncrementFailedAttempts(ctx, u.ID)
		if incErr != nil {
			newCount = u.FailedAttempts + 1
		}

		s.auditLogger.LogLoginFailed(ctx, username, workstationID, ipAddr, "wrong_password")

		if newCount >= LockoutThreshold {
			lockUntil := time.Now().Add(LockoutDuration)
			_ = s.userRepo.SetLockedUntil(ctx, u.ID, lockUntil)
			s.auditLogger.LogLockout(ctx, u.ID, u.Username)
			return nil, &apperr.AccountLocked{SecondsRemaining: int(LockoutDuration.Seconds())}
		}

		if newCount >= CaptchaThreshold {
			// Issue a CAPTCHA challenge for the next attempt.
			challenge, cErr := s.issueCaptcha(ctx, username, ipAddr)
			if cErr != nil {
				return nil, &apperr.Validation{Message: "invalid credentials"}
			}
			return nil, &apperr.CaptchaRequired{ChallengeKey: challenge.ChallengeKey}
		}

		return nil, &apperr.Validation{Message: "invalid credentials"}
	}

	// 7. Password correct — reset failed attempts and update last_login.
	_ = s.userRepo.ResetFailedAttempts(ctx, u.ID)
	_ = s.userRepo.SetLastLogin(ctx, u.ID)

	// 8. Load roles and permissions.
	roles, err := s.userRepo.GetRoles(ctx, u.ID)
	if err != nil {
		return nil, &apperr.Internal{Cause: err}
	}
	roleNames := make([]string, len(roles))
	for i, r := range roles {
		roleNames[i] = r.Name
	}
	perms, err := s.userRepo.GetPermissions(ctx, u.ID)
	if err != nil {
		return nil, &apperr.Internal{Cause: err}
	}

	// 9. Generate a 32-byte cryptographically random session token.
	rawToken, err := generateToken()
	if err != nil {
		return nil, &apperr.Internal{Cause: err}
	}

	// 10. Hash the token for storage. Only the hash is persisted.
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	// 11. Create the session record.
	session := &model.Session{
		TokenHash:     tokenHash,
		UserID:        u.ID,
		WorkstationID: strPtrOrNil(workstationID),
		IPAddress:     strPtrOrNil(ipAddr),
		ExpiresAt:     time.Now().Add(time.Duration(s.inactivitySeconds) * time.Second),
		IsValid:       true,
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, &apperr.Internal{Cause: err}
	}

	// 12. Log the successful login.
	s.auditLogger.LogLogin(ctx, u.ID, u.Username, workstationID, ipAddr)

	userWithRoles := &model.UserWithRoles{
		User:        u,
		Roles:       roleNames,
		Permissions: perms,
	}

	return &LoginResult{
		SessionToken: rawToken,
		Session:      session,
		User:         userWithRoles,
	}, nil
}

// Logout invalidates the given session and logs the event.
func (s *Service) Logout(ctx context.Context, sessionID, userID, username, workstationID string) error {
	if err := s.sessionRepo.Invalidate(ctx, sessionID); err != nil {
		return &apperr.Internal{Cause: err}
	}
	s.auditLogger.LogLogout(ctx, userID, username, sessionID)
	return nil
}

// StepUp verifies the user's current password and records the step-up timestamp
// on the session. Returns nil on success. The sessionID is required to record
// the timestamp; if it is empty the password check still runs but nothing is persisted.
func (s *Service) StepUp(ctx context.Context, userID, sessionID, password string) error {
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return &apperr.Unauthorized{}
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return &apperr.Unauthorized{}
	}
	if sessionID != "" {
		// Non-fatal: if the update fails the user must retry step-up.
		_ = s.sessionRepo.SetStepUp(ctx, sessionID)
	}
	return nil
}

// issueCaptcha generates and stores a new math CAPTCHA challenge.
// The challenge text is "What is X + Y?" where X, Y are 1-9 integers.
func (s *Service) issueCaptcha(ctx context.Context, username, ipAddr string) (*model.CaptchaChallenge, error) {
	x, err := randInt(1, 9)
	if err != nil {
		return nil, fmt.Errorf("issueCaptcha: generate X: %w", err)
	}
	y, err := randInt(1, 9)
	if err != nil {
		return nil, fmt.Errorf("issueCaptcha: generate Y: %w", err)
	}
	answer := fmt.Sprintf("%d", x+y)

	// Store only the hash of the answer.
	answerHash := hashAnswer(answer)

	// Generate an opaque challenge key (32 random bytes, hex-encoded).
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("issueCaptcha: generate key: %w", err)
	}
	challengeKey := hex.EncodeToString(keyBytes)

	challenge := &model.CaptchaChallenge{
		ChallengeKey: challengeKey,
		AnswerHash:   answerHash,
		Username:     strPtrOrNil(username),
		IPAddress:    strPtrOrNil(ipAddr),
		ExpiresAt:    time.Now().Add(CaptchaTTL),
	}

	// Store the challenge text as metadata in the ChallengeKey so the handler
	// can return it. We embed the question text in the challenge key using a
	// separator; the repo only stores challenge_key verbatim.
	// We keep the challenge text out of the DB and return it from here.
	// The handler will return both the key and the display text.

	// Store separately — we embed the question in a way the handler can retrieve it.
	// Since CaptchaChallenge.ChallengeKey is the opaque client token, we'll pass
	// the question text via a separate mechanism (returned from issueCaptcha and
	// embedded in the error — see CaptchaRequired extension in the handler).

	// For simplicity and DB compatibility: encode question into the ChallengeKey
	// as "<key>:<question>" — the client only uses the key part, and the handler
	// extracts the question from the full string before splitting on ':'.
	// This avoids changing the DB schema.
	displayQuestion := fmt.Sprintf("What is %d + %d?", x, y)
	challenge.ChallengeKey = challengeKey + ":" + displayQuestion

	if err := s.captchaRepo.Create(ctx, challenge); err != nil {
		return nil, fmt.Errorf("issueCaptcha: store: %w", err)
	}

	return challenge, nil
}

// validateCaptcha checks a CAPTCHA answer. Marks the challenge used even on failure.
func (s *Service) validateCaptcha(ctx context.Context, captchaKey, answer string) error {
	// Extract the opaque key portion (before the ':' separator if present).
	key := extractKey(captchaKey)

	challenge, err := s.captchaRepo.GetByKey(ctx, captchaKey)
	if err != nil {
		// Try with just the key portion (client may have stripped the question text).
		challenge, err = s.captchaRepo.GetByKey(ctx, key)
		if err != nil {
			return fmt.Errorf("captcha not found or expired")
		}
	}

	// Always mark used to prevent replay.
	_ = s.captchaRepo.MarkUsed(ctx, challenge.ID)

	// Compare the hash of the submitted answer (case-insensitive).
	submittedHash := hashAnswer(strings.TrimSpace(strings.ToLower(answer)))
	if submittedHash != challenge.AnswerHash {
		return fmt.Errorf("wrong captcha answer")
	}
	return nil
}

// hashAnswer returns the hex-encoded SHA-256 hash of the normalized answer.
func hashAnswer(answer string) string {
	normalised := strings.TrimSpace(strings.ToLower(answer))
	hash := sha256.Sum256([]byte(normalised))
	return hex.EncodeToString(hash[:])
}

// generateToken generates a 32-byte cryptographically random token,
// returned as a 64-character hex string.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generateToken: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// randInt returns a random integer in [min, max] using crypto/rand.
func randInt(min, max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()) + min, nil
}

// strPtrOrNil returns a pointer to s if non-empty, otherwise nil.
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// extractKey returns the portion of a challenge key before the first ':'.
// If no ':' is present, returns the full key.
func extractKey(key string) string {
	if idx := strings.Index(key, ":"); idx >= 0 {
		return key[:idx]
	}
	return key
}
