package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
)

var (
	// ErrInvalidCredentials is deliberately generic: it is returned for an
	// unknown email, a user without a password, and a wrong password alike.
	ErrInvalidCredentials = errors.New("email atau password salah")
	// ErrWrongCurrentPassword is for the change-password form only (the caller
	// is already signed in, so there is nothing to hide).
	ErrWrongCurrentPassword = errors.New("password saat ini salah")
)

// RateLimitedError says how long the caller must wait before trying again.
type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string {
	return "terlalu banyak percobaan gagal, coba lagi beberapa menit lagi"
}

// DefaultSessionTTL is one working day.
const DefaultSessionTTL = 12 * time.Hour

// Service owns sign-in, sessions and password changes.
type Service struct {
	DB  *gorm.DB
	TTL time.Duration
	// Now is injectable for tests.
	Now func() time.Time

	limiter *limiter
}

func NewService(db *gorm.DB, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &Service{DB: db, TTL: ttl, Now: time.Now, limiter: newLimiter()}
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Login checks the credentials and, on success, starts a session and returns
// its (raw) token -- the only time the raw token exists server-side -- and
// the user. ip is used for brute-force limiting only.
func (s *Service) Login(email, password, userAgent, ip string) (token string, user model.User, err error) {
	email = normalizeEmail(email)
	if wait := s.limiter.blockedFor(email, ip); wait > 0 {
		return "", model.User{}, &RateLimitedError{RetryAfter: wait}
	}

	var u model.User
	found := s.DB.Where("email = ?", email).First(&u).Error == nil
	hash := string(dummyHash)
	if found && u.PasswordHash != nil {
		hash = *u.PasswordHash
	}
	// Always run one bcrypt comparison, so timing doesn't reveal whether the
	// email exists or has a password.
	ok := checkPassword(hash, password)
	if !found || u.PasswordHash == nil || !ok {
		s.limiter.fail(email, ip)
		return "", model.User{}, ErrInvalidCredentials
	}
	s.limiter.succeed(email)

	token, err = newToken()
	if err != nil {
		return "", model.User{}, err
	}
	now := s.Now()
	sess := model.Session{TokenHash: hashToken(token), UserID: u.ID, ExpiresAt: now.Add(s.TTL)}
	if userAgent != "" {
		sess.UserAgent = &userAgent
	}
	if ip != "" {
		sess.IP = &ip
	}
	if err := s.DB.Create(&sess).Error; err != nil {
		return "", model.User{}, err
	}
	s.DB.Model(&model.User{}).Where("id = ?", u.ID).Update("last_login_at", now)
	// Housekeeping: expired sessions are useless; drop them as we go.
	s.DB.Where("expires_at < ?", now).Delete(&model.Session{})
	return token, u, nil
}

// Lookup returns the user behind a live session token, or (nil, nil) when the
// token is unknown or expired.
func (s *Service) Lookup(token string) (*model.User, error) {
	if token == "" {
		return nil, nil
	}
	var sess model.Session
	err := s.DB.Where("token_hash = ? AND expires_at > ?", hashToken(token), s.Now()).First(&sess).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var u model.User
	if err := s.DB.First(&u, sess.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// Logout ends one session.
func (s *Service) Logout(token string) error {
	if token == "" {
		return nil
	}
	return s.DB.Where("token_hash = ?", hashToken(token)).Delete(&model.Session{}).Error
}

// ChangePassword lets a signed-in user change their own password. Every OTHER
// session of that user is ended (the current one stays), so a stolen session
// doesn't survive a password change.
func (s *Service) ChangePassword(userID int64, currentToken, currentPassword, newPassword string) error {
	var u model.User
	if err := s.DB.First(&u, userID).Error; err != nil {
		return err
	}
	if u.PasswordHash == nil || !checkPassword(*u.PasswordHash, currentPassword) {
		return ErrWrongCurrentPassword
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("password_hash", hash).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ? AND token_hash <> ?", userID, hashToken(currentToken)).Delete(&model.Session{}).Error
	})
}

// SetPassword sets a user's password without knowing the old one (admin
// action / CLI) and ends ALL of that user's sessions.
func (s *Service) SetPassword(email, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	email = normalizeEmail(email)
	return s.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.User{}).Where("email = ?", email).Update("password_hash", hash)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Where("user_id IN (?)", tx.Model(&model.User{}).Select("id").Where("email = ?", email)).Delete(&model.Session{}).Error
	})
}
