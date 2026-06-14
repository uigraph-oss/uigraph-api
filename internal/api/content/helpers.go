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

// streamObject proxies an object from storage to the HTTP response. The browser
// fetches blobs (frame screenshots, diagram previews) through the API rather
// than via presigned URLs, so the only origin it ever talks to is the app's own
// — no internal storage host leaks into the browser and nothing has to be
// reachable or signed. The content type is sniffed from the first bytes so PNG,
// JPEG, and WebP all serve correctly. The caller is expected to add a content
// hash / file id to the URL, so responses are cached immutably.
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
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(head)
	_, _ = io.Copy(w, rc)
}
