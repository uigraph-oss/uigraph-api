package httputil_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/store"
)

func TestJSON_setsContentTypeAndStatus(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.JSON(w, http.StatusCreated, map[string]string{"id": "abc"})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
}

func TestJSON_encodesBody(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.JSON(w, http.StatusOK, map[string]any{"count": 3})

	var got map[string]any
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["count"] != float64(3) {
		t.Fatalf("unexpected body: %v", got)
	}
}

func TestError_notFound_returns404(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	httputil.Error(w, r, store.ErrNotFound)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	var body struct{ Code string }
	json.NewDecoder(w.Body).Decode(&body) //nolint:errcheck
	if body.Code != "not_found" {
		t.Fatalf("expected not_found code, got %q", body.Code)
	}
}

func TestError_conflict_returns409(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)

	httputil.Error(w, r, store.ErrConflict)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestError_other_returns500(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	httputil.Error(w, r, errors.New("db exploded"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestBadRequest_returns400WithCode(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.BadRequest(w, "name is required")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var body struct{ Code, Message string }
	json.NewDecoder(w.Body).Decode(&body) //nolint:errcheck
	if body.Code != "bad_request" {
		t.Fatalf("expected bad_request, got %q", body.Code)
	}
	if body.Message != "name is required" {
		t.Fatalf("unexpected message: %q", body.Message)
	}
}

func TestUnauthorized_returns401(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.Unauthorized(w)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestForbidden_returns403(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.Forbidden(w)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
