package content

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/uigraph/app/internal/storage"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg, "code": status})
}

// streamObject proxies an object from storage to the HTTP response.
func streamObject(w http.ResponseWriter, r *http.Request, st storage.Client, key string) {
	rc, err := st.Download(r.Context(), key)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	defer rc.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(rc, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		writeErr(w, http.StatusInternalServerError, "failed to read object")
		return
	}
	head = head[:n]

	w.Header().Set("Content-Type", http.DetectContentType(head))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(head)
	_, _ = io.Copy(w, rc)
}
