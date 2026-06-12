// Package httputil provides shared HTTP response helpers used across all handlers.
package httputil

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/uigraph/app/internal/store"
)

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		JSON(w, http.StatusNotFound, apiError("not_found", err.Error()))
	case errors.Is(err, store.ErrConflict):
		JSON(w, http.StatusConflict, apiError("conflict", err.Error()))
	default:
		slog.ErrorContext(r.Context(), "internal error", "err", err, "path", r.URL.Path)
		JSON(w, http.StatusInternalServerError, apiError("internal_error", "an unexpected error occurred"))
	}
}

func BadRequest(w http.ResponseWriter, msg string) {
	JSON(w, http.StatusBadRequest, apiError("bad_request", msg))
}

func Forbidden(w http.ResponseWriter) {
	JSON(w, http.StatusForbidden, apiError("forbidden", "insufficient permissions"))
}

func Unauthorized(w http.ResponseWriter) {
	JSON(w, http.StatusUnauthorized, apiError("unauthenticated", "authentication required"))
}

func NotImplemented(w http.ResponseWriter) {
	JSON(w, http.StatusNotImplemented, apiError("not_implemented", "not yet implemented"))
}

func Decode(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func apiError(code, msg string) errorBody {
	return errorBody{Code: code, Message: msg}
}
