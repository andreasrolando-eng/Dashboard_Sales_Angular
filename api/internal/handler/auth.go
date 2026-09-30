package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/Operations-ESB/dashboard-sales/api/internal/auth"
)

// maxAuthBody keeps a login request from being used to push large bodies.
const maxAuthBody = 16 << 10

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBody)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("body tidak valid"))
		return false
	}
	return true
}

// clientIP is the caller's address (chi's RealIP middleware has already
// folded X-Forwarded-For from the reverse proxy into RemoteAddr).
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login handles POST /api/auth/login. On success it sets the session cookie
// and returns the user. Every failure to authenticate answers the same
// generic message, so the response never says whether an email exists.
func Login(svc *auth.Service, cookies auth.CookieConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if !decodeBody(w, r, &req) {
			return
		}
		if req.Email == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("email dan password wajib diisi"))
			return
		}

		token, user, err := svc.Login(req.Email, req.Password, r.UserAgent(), clientIP(r))
		var limited *auth.RateLimitedError
		switch {
		case errors.As(err, &limited):
			w.Header().Set("Retry-After", strconv.Itoa(int(limited.RetryAfter.Seconds())+1))
			writeError(w, http.StatusTooManyRequests, limited)
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, auth.ErrInvalidCredentials)
		case err != nil:
			writeError(w, http.StatusInternalServerError, fmt.Errorf("gagal login"))
		default:
			cookies.SetCookie(w, token)
			writeJSON(w, http.StatusOK, user)
		}
	}
}

// Logout handles POST /api/auth/logout: ends the current session.
func Logout(svc *auth.Service, cookies auth.CookieConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Logout(auth.TokenFrom(r.Context())); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("gagal logout"))
			return
		}
		cookies.ClearCookie(w)
		writeJSON(w, http.StatusNoContent, nil)
	}
}

// Me handles GET /api/me: who is signed in.
func Me() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := auth.UserFrom(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("belum login"))
			return
		}
		writeJSON(w, http.StatusOK, u)
	}
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangeOwnPassword handles POST /api/auth/password.
func ChangeOwnPassword(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := auth.UserFrom(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("belum login"))
			return
		}
		var req changePasswordRequest
		if !decodeBody(w, r, &req) {
			return
		}
		err := svc.ChangePassword(u.ID, auth.TokenFrom(r.Context()), req.CurrentPassword, req.NewPassword)
		switch {
		case errors.Is(err, auth.ErrWrongCurrentPassword), errors.Is(err, auth.ErrWeakPassword):
			writeError(w, http.StatusBadRequest, err)
		case err != nil:
			writeError(w, http.StatusInternalServerError, fmt.Errorf("gagal mengganti password"))
		default:
			writeJSON(w, http.StatusNoContent, nil)
		}
	}
}
