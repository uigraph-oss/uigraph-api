package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	catalogpkg "github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/httputil"
	storepkg "github.com/uigraph/app/internal/store"
)

func (h *Handler) uploadSpec(ctx context.Context, key, content string) error {
	r := strings.NewReader(content)
	return h.storage.Upload(ctx, key, "application/octet-stream", r, int64(r.Len()))
}

func specHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

func sha256Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)
}

func (h *Handler) ensureServiceInOrg(w http.ResponseWriter, r *http.Request, serviceID string) bool {
	svc, err := h.store.GetService(r.Context(), serviceID)
	if err != nil {
		httputil.Error(w, r, err)
		return false
	}
	if svc == nil || svc.DeletedAt != nil || svc.OrgID != r.PathValue("orgID") {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return false
	}
	return true
}

func (h *Handler) readServiceDocPayload(r *http.Request) (string, string, string, []byte, error) {
	fileName, fileType, description, fileBytes, err := h.readOptionalServiceDocPayload(r, nil)
	if err != nil {
		return "", "", "", nil, err
	}
	if len(fileBytes) == 0 {
		return "", "", "", nil, fmt.Errorf("file content is required")
	}
	return fileName, fileType, description, fileBytes, nil
}

func (h *Handler) readOptionalServiceDocPayload(r *http.Request, existing *catalogpkg.ServiceDoc) (string, string, string, []byte, error) {
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		return readServiceDocFromMultipart(r, existing)
	}
	return readServiceDocFromJSON(r, existing)
}

func readServiceDocFromJSON(r *http.Request, existing *catalogpkg.ServiceDoc) (string, string, string, []byte, error) {
	var body struct {
		FileName      *string `json:"fileName"`
		FileType      *string `json:"fileType"`
		Description   *string `json:"description"`
		ContentBase64 *string `json:"contentBase64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return "", "", "", nil, fmt.Errorf("invalid request body")
	}

	fileName, fileType, description := "", "", ""
	if existing != nil {
		fileName, fileType, description = existing.FileName, existing.FileType, existing.Description
	}
	if body.FileName != nil {
		fileName = strings.TrimSpace(*body.FileName)
	}
	if body.FileType != nil {
		fileType = strings.TrimSpace(*body.FileType)
	}
	if body.Description != nil {
		description = strings.TrimSpace(*body.Description)
	}
	if fileType == "" {
		fileType = "application/octet-stream"
	}
	if fileName == "" {
		return "", "", "", nil, fmt.Errorf("fileName is required")
	}
	var out []byte
	if body.ContentBase64 != nil && strings.TrimSpace(*body.ContentBase64) != "" {
		raw, err := base64.StdEncoding.DecodeString(*body.ContentBase64)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("contentBase64 must be valid base64")
		}
		out = raw
	}
	return fileName, fileType, description, out, nil
}

func readServiceDocFromMultipart(r *http.Request, existing *catalogpkg.ServiceDoc) (string, string, string, []byte, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return "", "", "", nil, fmt.Errorf("invalid multipart form")
	}
	fileName, fileType, description := "", "", ""
	if existing != nil {
		fileName, fileType, description = existing.FileName, existing.FileType, existing.Description
	}
	if v := strings.TrimSpace(r.FormValue("fileName")); v != "" {
		fileName = v
	}
	if v := strings.TrimSpace(r.FormValue("fileType")); v != "" {
		fileType = v
	}
	if v := r.FormValue("description"); v != "" {
		description = strings.TrimSpace(v)
	}
	if fileType == "" {
		fileType = "application/octet-stream"
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		if existing != nil {
			if fileName == "" {
				return "", "", "", nil, fmt.Errorf("fileName is required")
			}
			return fileName, fileType, description, nil, nil
		}
		return "", "", "", nil, fmt.Errorf("file is required")
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed to read file")
	}
	if fileName == "" {
		fileName = strings.TrimSpace(header.Filename)
	}
	if fileName == "" {
		return "", "", "", nil, fmt.Errorf("fileName is required")
	}
	if fileType == "application/octet-stream" {
		if headerType := strings.TrimSpace(header.Header.Get("Content-Type")); headerType != "" {
			fileType = headerType
		}
	}
	return fileName, fileType, description, content, nil
}

// toSlug converts a name to a URL-safe slug (lowercase, hyphens).
func toSlug(name string) string {
	s := strings.ToLower(name)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			return r
		}
		return '-'
	}, s)
	// Collapse consecutive hyphens.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
