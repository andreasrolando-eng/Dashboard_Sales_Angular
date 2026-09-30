package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Operations-ESB/dashboard-sales/api/internal/handler"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

// Browsers send "@" as %40 in path segments; chi does not decode it for us.
func TestRemoveUser_PercentEncodedEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	if err := db.Create(&model.User{Email: "tes.uji@esb.co.id"}).Error; err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Delete("/api/admin/users/{email}", handler.RemoveUser(db))

	for _, path := range []string{"/api/admin/users/tes.uji%40esb.co.id"} {
		req := httptest.NewRequest(http.MethodDelete, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("DELETE %s = %d, want 204", path, rec.Code)
		}
	}

	var count int64
	db.Model(&model.User{}).Where("email = ?", "tes.uji@esb.co.id").Count(&count)
	if count != 0 {
		t.Errorf("user still present after DELETE with %%40-encoded email (count=%d)", count)
	}
}
