package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/auth"
	"github.com/Operations-ESB/dashboard-sales/api/internal/config"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

const testPassword = "rahasia-123"

type env struct {
	t      *testing.T
	db     *gorm.DB
	router http.Handler
}

func newEnv(t *testing.T, secureCookie bool) *env {
	t.Helper()
	db := testutil.SetupTestDB(t)
	cfg := config.Config{SessionTTLHours: 12, CookieSecure: secureCookie}
	return &env{t: t, db: db, router: newRouter(cfg, db, nil)}
}

func (e *env) user(email string, admin bool) {
	e.t.Helper()
	h, err := auth.HashPassword(testPassword)
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.db.Create(&model.User{Email: email, IsAdmin: admin, PasswordHash: &h}).Error; err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) do(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// login returns the session cookie for a user created with e.user.
func (e *env) login(email string) *http.Cookie {
	e.t.Helper()
	rec := e.do(http.MethodPost, "/api/auth/login", `{"email":"`+email+`","password":"`+testPassword+`"}`, nil)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("login %s = %d %s", email, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	e.t.Fatal("login set no session cookie")
	return nil
}

// Guard against the classic mistake: adding a route and forgetting to
// protect it. Walks EVERY route the router serves and requires a 401 without
// a session, except the two that must stay public.
func TestEveryRouteRequiresLoginExceptTheAllowList(t *testing.T) {
	e := newEnv(t, false)
	public := map[string]bool{"GET /healthz": true, "POST /api/auth/login": true}

	checked := 0
	err := chi.Walk(e.router.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		path := strings.NewReplacer("{email}", "x%40y.z", "{memberCode}", "M1").Replace(route)
		path = strings.ReplaceAll(strings.TrimSuffix(path, "/*"), "//", "/")
		if len(path) > 1 {
			path = strings.TrimSuffix(path, "/")
		}
		rec := e.do(method, path, "", nil)
		key := method + " " + path
		if public[key] {
			return nil
		}
		checked++
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s answered %d without a session, want 401 (route is not protected!)", key, rec.Code)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 25 {
		t.Errorf("only %d protected routes were checked; the walk is probably broken", checked)
	}
}

func TestAdminRoutesAreForbiddenToNonAdmins(t *testing.T) {
	e := newEnv(t, false)
	e.user("admin@esb.co.id", true)
	e.user("biasa@esb.co.id", false)
	admin, biasa := e.login("admin@esb.co.id"), e.login("biasa@esb.co.id")

	for _, r := range []struct{ method, path, body string }{
		{"GET", "/api/admin/users", ""},
		{"POST", "/api/admin/users", `{"email":"x@esb.co.id"}`},
		{"POST", "/api/admin/users/admin%40esb.co.id/password", `{"password":"baru-12345"}`},
		{"DELETE", "/api/admin/users/admin%40esb.co.id", ""},
		{"GET", "/api/admin/sync", ""},
		{"POST", "/api/admin/sync", `{}`},
		{"GET", "/api/admin/sync/logs", ""},
	} {
		if rec := e.do(r.method, r.path, r.body, biasa); rec.Code != http.StatusForbidden {
			t.Errorf("non-admin %s %s = %d, want 403", r.method, r.path, rec.Code)
		}
	}
	if rec := e.do("GET", "/api/admin/users", "", admin); rec.Code != http.StatusOK {
		t.Errorf("admin GET users = %d, want 200", rec.Code)
	}
	// Ordinary dashboard data stays open to any signed-in user.
	if rec := e.do("GET", "/api/meta/outlets", "", biasa); rec.Code != http.StatusOK {
		t.Errorf("non-admin GET outlets = %d, want 200", rec.Code)
	}
}

func TestLoginFlow_CookieMeAndLogout(t *testing.T) {
	e := newEnv(t, true) // production-style: Secure cookie
	e.user("andi@esb.co.id", true)

	if rec := e.do("POST", "/api/auth/login", `{"email":"andi@esb.co.id","password":"salah"}`, nil); rec.Code != http.StatusUnauthorized ||
		!strings.Contains(rec.Body.String(), "email atau password salah") {
		t.Errorf("bad password = %d %s, want 401 with the generic message", rec.Code, rec.Body.String())
	}
	if rec := e.do("POST", "/api/auth/login", `{"email":"tidak-ada@esb.co.id","password":"salah"}`, nil); rec.Code != http.StatusUnauthorized ||
		!strings.Contains(rec.Body.String(), "email atau password salah") {
		t.Errorf("unknown email must give the SAME response as a wrong password, got %d %s", rec.Code, rec.Body.String())
	}
	for _, body := range []string{`{}`, `{"email":"andi@esb.co.id"}`, `nope`} {
		if rec := e.do("POST", "/api/auth/login", body, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("login body %q = %d, want 400", body, rec.Code)
		}
	}

	rec := e.do("POST", "/api/auth/login", `{"email":"ANDI@esb.co.id","password":"`+testPassword+`"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login = %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "password_hash") || strings.Contains(rec.Body.String(), "$2a$") {
		t.Errorf("login response must not expose password data: %s", rec.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 12*3600 {
		t.Fatalf("session cookie = %+v, want HttpOnly + Secure + SameSite=Lax + 12h", cookie)
	}

	me := e.do("GET", "/api/me", "", cookie)
	if got := me.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("/api/me Cache-Control = %q, want no-store", got)
	}
	var got map[string]any
	_ = json.Unmarshal(me.Body.Bytes(), &got)
	if me.Code != http.StatusOK || got["email"] != "andi@esb.co.id" || got["is_admin"] != true {
		t.Fatalf("/api/me = %d %s", me.Code, me.Body.String())
	}
	if _, leaked := got["password_hash"]; leaked {
		t.Error("/api/me must not include password_hash")
	}

	out := e.do("POST", "/api/auth/logout", "", cookie)
	if out.Code != http.StatusNoContent {
		t.Fatalf("logout = %d", out.Code)
	}
	cleared := false
	for _, c := range out.Result().Cookies() {
		if c.Name == auth.CookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("logout should clear the cookie in the browser")
	}
	if rec := e.do("GET", "/api/me", "", cookie); rec.Code != http.StatusUnauthorized {
		t.Errorf("/api/me with a logged-out token = %d, want 401 (session must be revoked server-side)", rec.Code)
	}
}

func TestLogin_IsThrottledAfterRepeatedFailures(t *testing.T) {
	e := newEnv(t, false)
	e.user("korban@esb.co.id", false)
	for i := 0; i < 5; i++ {
		e.do("POST", "/api/auth/login", `{"email":"korban@esb.co.id","password":"tebak-`+string(rune('a'+i))+`"}`, nil)
	}
	rec := e.do("POST", "/api/auth/login", `{"email":"korban@esb.co.id","password":"`+testPassword+`"}`, nil)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Errorf("6th attempt = %d (Retry-After %q), want 429 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}
}

func TestAdminUserManagement_PasswordsAndSelfRemoval(t *testing.T) {
	e := newEnv(t, false)
	e.user("admin@esb.co.id", true)
	admin := e.login("admin@esb.co.id")

	// A weak password is refused BEFORE the account is created.
	if rec := e.do("POST", "/api/admin/users", `{"email":"baru@esb.co.id","password":"pendek"}`, admin); rec.Code != http.StatusBadRequest {
		t.Errorf("weak password = %d, want 400", rec.Code)
	}
	var n int64
	e.db.Model(&model.User{}).Where("email = ?", "baru@esb.co.id").Count(&n)
	if n != 0 {
		t.Error("a rejected password must not leave a half-created user behind")
	}

	// Creating a user with a password lets them sign in straight away.
	if rec := e.do("POST", "/api/admin/users", `{"email":"Baru@esb.co.id","password":"`+testPassword+`"}`, admin); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"has_password":true`) || strings.Contains(rec.Body.String(), "$2a$") {
		t.Fatalf("add user = %d %s", rec.Code, rec.Body.String())
	}
	baru := e.login("baru@esb.co.id")

	// Without a password the account exists but cannot sign in.
	e.do("POST", "/api/admin/users", `{"email":"tanpa@esb.co.id"}`, admin)
	if rec := e.do("POST", "/api/auth/login", `{"email":"tanpa@esb.co.id","password":"apa-saja-123"}`, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("account without a password logged in: %d", rec.Code)
	}

	// Admin reset ends that user's sessions and installs the new password.
	if rec := e.do("POST", "/api/admin/users/baru%40esb.co.id/password", `{"password":"reset-baru-99"}`, admin); rec.Code != http.StatusNoContent {
		t.Fatalf("set password = %d", rec.Code)
	}
	if rec := e.do("GET", "/api/me", "", baru); rec.Code != http.StatusUnauthorized {
		t.Errorf("old session after admin reset = %d, want 401", rec.Code)
	}
	if rec := e.do("POST", "/api/auth/login", `{"email":"baru@esb.co.id","password":"reset-baru-99"}`, nil); rec.Code != http.StatusOK {
		t.Errorf("login with the reset password = %d", rec.Code)
	}
	if rec := e.do("POST", "/api/admin/users/hantu%40esb.co.id/password", `{"password":"reset-baru-99"}`, admin); rec.Code != http.StatusNotFound {
		t.Errorf("reset for an unknown user = %d, want 404", rec.Code)
	}

	// You can't remove yourself; you can remove others (and their sessions go).
	if rec := e.do("DELETE", "/api/admin/users/admin%40esb.co.id", "", admin); rec.Code != http.StatusConflict {
		t.Errorf("self removal = %d, want 409", rec.Code)
	}
	e.db.Model(&model.User{}).Where("email = ?", "admin@esb.co.id").Count(&n)
	if n != 1 {
		t.Error("the admin removed themselves")
	}
	if rec := e.do("DELETE", "/api/admin/users/baru%40esb.co.id", "", admin); rec.Code != http.StatusNoContent {
		t.Errorf("remove other user = %d, want 204", rec.Code)
	}
}

func TestChangeOwnPassword(t *testing.T) {
	e := newEnv(t, false)
	e.user("andi@esb.co.id", false)
	c := e.login("andi@esb.co.id")

	if rec := e.do("POST", "/api/auth/password", `{"current_password":"salah-banget","new_password":"baru-12345"}`, c); rec.Code != http.StatusBadRequest {
		t.Errorf("wrong current password = %d, want 400", rec.Code)
	}
	if rec := e.do("POST", "/api/auth/password", `{"current_password":"`+testPassword+`","new_password":"pendek"}`, c); rec.Code != http.StatusBadRequest {
		t.Errorf("weak new password = %d, want 400", rec.Code)
	}
	if rec := e.do("POST", "/api/auth/password", `{"current_password":"`+testPassword+`","new_password":"baru-12345"}`, c); rec.Code != http.StatusNoContent {
		t.Fatalf("change password = %d", rec.Code)
	}
	if rec := e.do("GET", "/api/me", "", c); rec.Code != http.StatusOK {
		t.Errorf("the session that changed the password should stay valid, got %d", rec.Code)
	}
	if rec := e.do("POST", "/api/auth/password", `{"current_password":"x","new_password":"yyyyyyyy"}`, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("change password without a session = %d, want 401", rec.Code)
	}
}
