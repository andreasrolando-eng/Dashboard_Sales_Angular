// Package auth implements local email + password sign-in with server-side
// sessions. The session layer (cookie -> sessions row -> user) is independent
// of how a session was created, so operations-sso can later create sessions
// through the same tables without touching the rest of the API.
package auth

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	MinPasswordLen = 8
	// bcrypt silently ignores everything after 72 bytes, so longer passwords
	// would be "accepted" but only partly checked. Reject them instead.
	maxPasswordBytes = 72
)

// bcryptCost is a var so tests can lower it; 12 is a sane 2026 default for a
// login form (~250 ms).
var bcryptCost = 12

var ErrWeakPassword = errors.New("password tidak memenuhi syarat")

// ValidatePassword enforces the password policy: at least 8 characters, at
// most 72 bytes (bcrypt's limit), and not blank/whitespace only.
func ValidatePassword(pw string) error {
	if utf8.RuneCountInString(pw) < MinPasswordLen {
		return fmt.Errorf("%w: minimal %d karakter", ErrWeakPassword, MinPasswordLen)
	}
	if len(pw) > maxPasswordBytes {
		return fmt.Errorf("%w: maksimal %d karakter", ErrWeakPassword, maxPasswordBytes)
	}
	blank := true
	for _, r := range pw {
		if r != ' ' && r != '\t' {
			blank = false
			break
		}
	}
	if blank {
		return fmt.Errorf("%w: tidak boleh kosong", ErrWeakPassword)
	}
	return nil
}

// HashPassword returns a bcrypt hash after validating the policy.
func HashPassword(pw string) (string, error) {
	if err := ValidatePassword(pw); err != nil {
		return "", err
	}
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// dummyHash is compared against when the email is unknown (or has no
// password) so a failed login takes about as long as one for a real account
// and does not reveal which emails exist.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcryptCost)
