// Package httputil provides shared HTTP response helpers used across all handlers.
package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

func Conflict(w http.ResponseWriter, msg string) {
	JSON(w, http.StatusConflict, apiError("conflict", msg))
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

// StreamObject proxies an object from object storage to an HTTP response.
// It calls download(r.Context(), key) to retrieve the object, detects the content type
// by sniffing the first 512 bytes, and streams the complete object to the response with
// appropriate caching headers.
func StreamObject(w http.ResponseWriter, r *http.Request, download func(context.Context, string) (io.ReadCloser, error), key string) {
	// Download the object from storage
	rc, err := download(r.Context(), key)
	if err != nil {
		Error(w, r, store.ErrNotFound)
		return
	}
	defer rc.Close()

	// Read up to 512 bytes to sniff the content type
	head := make([]byte, 512)
	n, err := rc.Read(head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		slog.ErrorContext(r.Context(), "failed to read object head", "err", err, "key", key)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Detect content type from the bytes read
	contentType := http.DetectContentType(head[:n])

	// Set response headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)

	// Write the sniffed bytes first
	if n > 0 {
		_, _ = w.Write(head[:n])
	}

	// Copy the rest of the body
	_, _ = io.Copy(w, rc)
}

