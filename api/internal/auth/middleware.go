package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
)

// CookieName is the session cookie. HttpOnly, so scripts can't read it.
const CookieName = "ds_session"

type ctxKey int

const (
	userKey ctxKey = iota
	tokenKey
)

// UserFrom returns the signed-in user of a request that passed Require.
func UserFrom(ctx context.Context) (*model.User, bool) {
	u, ok := ctx.Value(userKey).(*model.User)
	return u, ok && u != nil
}

// TokenFrom returns the raw session token of a request that passed Require.
func TokenFrom(ctx context.Context) string {
	t, _ := ctx.Value(tokenKey).(string)
	return t
}

// CookieConfig controls how the session cookie is set.
type CookieConfig struct {
	Secure bool // true in production (HTTPS)
	TTL    time.Duration
}

// SetCookie stores the session token in the browser. SameSite=Lax means the
// browser won't send it on cross-site POST/DELETE requests, which is what
// protects the JSON API against CSRF.
func (c CookieConfig) SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: token, Path: "/", HttpOnly: true, Secure: c.Secure,
		SameSite: http.SameSiteLaxMode, MaxAge: int(c.TTL.Seconds()),
	})
}

func (c CookieConfig) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: "", Path: "/", HttpOnly: true, Secure: c.Secure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

func deny(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Require rejects requests without a live session (401) and otherwise puts
// the user and token in the request context.
func Require(s *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(CookieName)
			if err != nil {
				deny(w, http.StatusUnauthorized, "belum login")
				return
			}
			u, err := s.Lookup(c.Value)
			if err != nil {
				deny(w, http.StatusInternalServerError, "gagal memeriksa sesi")
				return
			}
			if u == nil {
				deny(w, http.StatusUnauthorized, "sesi tidak valid atau sudah berakhir")
				return
			}
			ctx := context.WithValue(r.Context(), userKey, u)
			ctx = context.WithValue(ctx, tokenKey, c.Value)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin must run after Require; non-admins get 403.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, ok := UserFrom(r.Context()); !ok || !u.IsAdmin {
			deny(w, http.StatusForbidden, "hanya admin yang boleh mengakses ini")
			return
		}
		next.ServeHTTP(w, r)
	})
}
