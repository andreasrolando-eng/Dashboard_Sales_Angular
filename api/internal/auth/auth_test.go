package auth_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/auth"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func newUser(t *testing.T, db *gorm.DB, email, password string, admin bool) model.User {
	t.Helper()
	u := model.User{Email: email, IsAdmin: admin}
	if password != "" {
		h, err := auth.HashPassword(password)
		if err != nil {
			t.Fatal(err)
		}
		u.PasswordHash = &h
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	return u
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name string
		pw   string
		ok   bool
	}{
		{"ok", "abcd1234", true},
		{"passphrase with spaces", "kuda hijau makan rumput", true},
		{"too short", "abc123", false},
		{"seven chars", "abcdefg", false},
		{"whitespace only", "          ", false},
		{"exactly 72 bytes", strings.Repeat("a", 72), true},
		{"73 bytes (bcrypt would truncate)", strings.Repeat("a", 73), false},
	}
	for _, c := range cases {
		err := auth.ValidatePassword(c.pw)
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
		if err != nil && !errors.Is(err, auth.ErrWeakPassword) {
			t.Errorf("%s: error should wrap ErrWeakPassword, got %v", c.name, err)
		}
	}
}

func TestLogin_SuccessStartsASessionThatLookupAndLogoutHonour(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := auth.NewService(db, time.Hour)
	newUser(t, db, "andi@esb.co.id", "rahasia-123", true)

	token, u, err := svc.Login("  ANDI@esb.co.id ", "rahasia-123", "TestAgent", "10.0.0.1")
	if err != nil || token == "" || u.Email != "andi@esb.co.id" {
		t.Fatalf("Login = %q, %+v, %v", token, u, err)
	}

	got, err := svc.Lookup(token)
	if err != nil || got == nil || got.Email != "andi@esb.co.id" || !got.IsAdmin || !got.HasPassword {
		t.Fatalf("Lookup = %+v, %v", got, err)
	}
	var stored model.User
	db.First(&stored, u.ID)
	if stored.LastLoginAt == nil {
		t.Error("last_login_at should be set after a login")
	}

	if err := svc.Logout(token); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.Lookup(token); got != nil {
		t.Error("Lookup after Logout should find nothing")
	}
}

func TestLogin_TokenIsStoredOnlyAsAHash(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := auth.NewService(db, time.Hour)
	newUser(t, db, "a@esb.co.id", "rahasia-123", false)

	token, _, err := svc.Login("a@esb.co.id", "rahasia-123", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var sess model.Session
	db.First(&sess)
	if sess.TokenHash == token || strings.Contains(sess.TokenHash, token) {
		t.Errorf("the raw token must never be stored; token_hash = %q", sess.TokenHash)
	}
	if len(token) < 40 {
		t.Errorf("token %q looks too short to be 256 bits of randomness", token)
	}
}

func TestLogin_FailuresAreIndistinguishable(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := auth.NewService(db, time.Hour)
	newUser(t, db, "ada@esb.co.id", "rahasia-123", false)
	newUser(t, db, "tanpa-password@esb.co.id", "", false)

	for name, c := range map[string][2]string{
		"wrong password":       {"ada@esb.co.id", "salah-banget"},
		"unknown email":        {"tidak-ada@esb.co.id", "rahasia-123"},
		"account with no pass": {"tanpa-password@esb.co.id", "apa-saja-123"},
		"empty password":       {"ada@esb.co.id", ""},
	} {
		token, _, err := svc.Login(c[0], c[1], "", "")
		if token != "" || !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Errorf("%s: token=%q err=%v, want ErrInvalidCredentials", name, token, err)
		}
	}
	var n int64
	db.Model(&model.Session{}).Count(&n)
	if n != 0 {
		t.Errorf("%d sessions created by failed logins, want 0", n)
	}
}

func TestSession_ExpiresAfterTTL(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := auth.NewService(db, 2*time.Hour)
	now := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })
	newUser(t, db, "a@esb.co.id", "rahasia-123", false)

	token, _, _ := svc.Login("a@esb.co.id", "rahasia-123", "", "")
	now = now.Add(2*time.Hour - time.Minute)
	if u, _ := svc.Lookup(token); u == nil {
		t.Error("session should still be valid just before the TTL")
	}
	now = now.Add(2 * time.Minute)
	if u, _ := svc.Lookup(token); u != nil {
		t.Error("session should be rejected after the TTL")
	}
}

func TestLogin_LocksOutAfterRepeatedFailuresEvenWithTheRightPassword(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := auth.NewService(db, time.Hour)
	now := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })
	newUser(t, db, "korban@esb.co.id", "rahasia-123", false)

	for i := 0; i < 5; i++ {
		guess := fmt.Sprintf("tebakan-%d", i)
		if _, _, err := svc.Login("korban@esb.co.id", guess, "", "1.2.3.4"); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: err = %v", i, err)
		}
	}
	_, _, err := svc.Login("korban@esb.co.id", "rahasia-123", "", "1.2.3.4") // correct, but locked
	var rl *auth.RateLimitedError
	if !errors.As(err, &rl) || rl.RetryAfter <= 0 {
		t.Fatalf("err = %v, want RateLimitedError with a positive RetryAfter", err)
	}
	// A different email from the same IP is not locked (only the IP counter rose).
	newUser(t, db, "lain@esb.co.id", "rahasia-123", false)
	if _, _, err := svc.Login("lain@esb.co.id", "rahasia-123", "", "1.2.3.4"); err != nil {
		t.Errorf("other account should still be able to log in, got %v", err)
	}
	// After the window the lock lifts.
	now = now.Add(16 * time.Minute)
	if _, _, err := svc.Login("korban@esb.co.id", "rahasia-123", "", "1.2.3.4"); err != nil {
		t.Errorf("lockout should lift after the window, got %v", err)
	}
}

func TestChangePassword_KeepsCurrentSessionEndsOthers(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := auth.NewService(db, time.Hour)
	u := newUser(t, db, "a@esb.co.id", "lama-12345", false)
	current, _, _ := svc.Login("a@esb.co.id", "lama-12345", "", "")
	other, _, _ := svc.Login("a@esb.co.id", "lama-12345", "", "")

	if err := svc.ChangePassword(u.ID, current, "bukan-yang-lama", "baru-12345"); !errors.Is(err, auth.ErrWrongCurrentPassword) {
		t.Errorf("wrong current password: err = %v", err)
	}
	if err := svc.ChangePassword(u.ID, current, "lama-12345", "pendek"); !errors.Is(err, auth.ErrWeakPassword) {
		t.Errorf("weak new password: err = %v", err)
	}
	if err := svc.ChangePassword(u.ID, current, "lama-12345", "baru-12345"); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.Lookup(current); got == nil {
		t.Error("the session that changed the password must stay signed in")
	}
	if got, _ := svc.Lookup(other); got != nil {
		t.Error("other sessions must end when the password changes")
	}
	if _, _, err := svc.Login("a@esb.co.id", "lama-12345", "", ""); err == nil {
		t.Error("old password must stop working")
	}
	if _, _, err := svc.Login("a@esb.co.id", "baru-12345", "", ""); err != nil {
		t.Errorf("new password should work: %v", err)
	}
}

func TestSetPassword_EndsAllSessionsAndFailsForUnknownUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := auth.NewService(db, time.Hour)
	newUser(t, db, "a@esb.co.id", "lama-12345", false)
	tok, _, _ := svc.Login("a@esb.co.id", "lama-12345", "", "")

	if err := svc.SetPassword("A@esb.co.id", "reset-12345"); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.Lookup(tok); got != nil {
		t.Error("an admin reset must end every session of that user")
	}
	if _, _, err := svc.Login("a@esb.co.id", "reset-12345", "", ""); err != nil {
		t.Errorf("reset password should work: %v", err)
	}
	if err := svc.SetPassword("tidak-ada@esb.co.id", "reset-12345"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("unknown user: err = %v, want ErrRecordNotFound", err)
	}
	if err := svc.SetPassword("a@esb.co.id", "x"); !errors.Is(err, auth.ErrWeakPassword) {
		t.Errorf("weak password: err = %v", err)
	}
}

func TestDeletingAUserEndsTheirSessions(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := auth.NewService(db, time.Hour)
	u := newUser(t, db, "a@esb.co.id", "rahasia-123", false)
	tok, _, _ := svc.Login("a@esb.co.id", "rahasia-123", "", "")
	db.Delete(&u)
	if got, _ := svc.Lookup(tok); got != nil {
		t.Error("session of a deleted user must not work")
	}
}

func TestMiddleware_RequireAndRequireAdmin(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := auth.NewService(db, time.Hour)
	newUser(t, db, "admin@esb.co.id", "rahasia-123", true)
	newUser(t, db, "biasa@esb.co.id", "rahasia-123", false)
	adminTok, _, _ := svc.Login("admin@esb.co.id", "rahasia-123", "", "")
	userTok, _, _ := svc.Login("biasa@esb.co.id", "rahasia-123", "", "")

	var seen string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.UserFrom(r.Context())
		seen = u.Email
		w.WriteHeader(http.StatusOK)
	})
	protected := auth.Require(svc)(inner)
	adminOnly := auth.Require(svc)(auth.RequireAdmin(inner))

	call := func(h http.Handler, token string) int {
		seen = ""
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if c := call(protected, ""); c != http.StatusUnauthorized {
		t.Errorf("no cookie: %d, want 401", c)
	}
	if c := call(protected, "token-palsu"); c != http.StatusUnauthorized {
		t.Errorf("bogus token: %d, want 401", c)
	}
	if c := call(protected, userTok); c != http.StatusOK || seen != "biasa@esb.co.id" {
		t.Errorf("valid session: %d (%q), want 200 with the user in context", c, seen)
	}
	if c := call(adminOnly, userTok); c != http.StatusForbidden || seen != "" {
		t.Errorf("non-admin on admin route: %d, want 403 without running the handler", c)
	}
	if c := call(adminOnly, adminTok); c != http.StatusOK {
		t.Errorf("admin on admin route: %d, want 200", c)
	}
}
