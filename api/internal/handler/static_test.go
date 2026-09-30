package handler_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Operations-ESB/dashboard-sales/api/internal/handler"
)

func TestSPA(t *testing.T) {
	dir := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "main-ABC123.js"), []byte("console.log(1)"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "icon.png"), []byte("png"), 0o644))
	secret := filepath.Join(filepath.Dir(dir), "secret.txt")
	must(os.WriteFile(secret, []byte("rahasia"), 0o644))
	t.Cleanup(func() { os.Remove(secret) })

	h := handler.SPA(dir)
	get := func(method, p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, p, nil))
		return rec
	}

	cases := []struct {
		name, method, path string
		code               int
		body, cache        string
	}{
		{"root is the app", "GET", "/", 200, "app", "no-cache"},
		{"client route falls back to index", "GET", "/sales", 200, "app", "no-cache"},
		{"nested client route", "GET", "/admin/users", 200, "app", "no-cache"},
		{"hashed bundle is long-cached", "GET", "/main-ABC123.js", 200, "console.log", "immutable"},
		{"static asset", "GET", "/icon.png", 200, "png", "immutable"},
		{"unknown api path stays JSON 404", "GET", "/api/tidak-ada", 404, "endpoint tidak ditemukan", ""},
		{"path traversal is rejected, nothing leaks", "GET", "/../secret.txt", 400, "", ""},
		{"non-GET is refused", "POST", "/sales", 405, "", ""},
	}
	for _, c := range cases {
		rec := get(c.method, c.path)
		if rec.Code != c.code {
			t.Errorf("%s: status %d, want %d", c.name, rec.Code, c.code)
		}
		if c.body != "" && !strings.Contains(rec.Body.String(), c.body) {
			t.Errorf("%s: body %q, want it to contain %q", c.name, rec.Body.String(), c.body)
		}
		if strings.Contains(rec.Body.String(), "rahasia") {
			t.Errorf("%s: leaked a file outside the static dir", c.name)
		}
		if c.cache != "" && !strings.Contains(rec.Header().Get("Cache-Control"), c.cache) {
			t.Errorf("%s: Cache-Control %q, want %q", c.name, rec.Header().Get("Cache-Control"), c.cache)
		}
	}
}
