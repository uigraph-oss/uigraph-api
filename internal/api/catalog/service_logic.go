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
	"time"

	"github.com/google/uuid"
	catalogpkg "github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/httputil"
	storepkg "github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/storage"
	"gopkg.in/yaml.v3"
)

func (h *Handler) uploadSpec(ctx context.Context, key, content string) error {
	r := strings.NewReader(content)
	return h.storage.Upload(ctx, key, "application/octet-stream", r, int64(r.Len()))
}

// resolveSpec returns spec content from either the raw spec string or a pre-uploaded asset.
// specAssetID takes precedence when present.
func (h *Handler) resolveSpec(ctx context.Context, spec string, specAssetID *string) (string, error) {
	if specAssetID == nil {
		return spec, nil
	}
	rc, err := h.storage.Download(ctx, storage.AssetKey(*specAssetID))
	if err != nil {
		return "", fmt.Errorf("download spec asset: %w", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("read spec asset: %w", err)
	}
	return string(data), nil
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

var httpMethods = []string{"get", "post", "put", "delete", "patch", "head", "options", "trace"}

// importSpecEndpoints replaces the current working-copy endpoints for an API group.
// Versioned snapshot endpoints (api_group_version_id IS NOT NULL) are never touched.
func (h *Handler) importSpecEndpoints(ctx context.Context, spec, apiGroupID, serviceID, orgID, actorID string, now time.Time) {
	endpoints, err := parseSpecEndpoints(spec, apiGroupID, serviceID, orgID, actorID, now)
	if err != nil || len(endpoints) == 0 {
		return
	}
	_ = h.store.SoftDeleteCurrentAPIEndpoints(ctx, apiGroupID, actorID)
	for _, e := range endpoints {
		_ = h.store.CreateAPIEndpoint(ctx, e)
	}
}

// parseSpecEndpoints parses an OpenAPI 3.x or Swagger 2.0 spec (JSON or YAML)
// and returns one APIEndpoint per HTTP operation.
func parseSpecEndpoints(spec, apiGroupID, serviceID, orgID, actorID string, now time.Time) ([]catalogpkg.APIEndpoint, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(spec), &doc); err != nil {
		if err2 := yaml.Unmarshal([]byte(spec), &doc); err2 != nil {
			return nil, fmt.Errorf("spec is not valid JSON or YAML")
		}
	}

	rawPaths, _ := doc["paths"].(map[string]interface{})
	var endpoints []catalogpkg.APIEndpoint
	order := 0.0
	for path, pathItem := range rawPaths {
		item, ok := pathItem.(map[string]interface{})
		if !ok {
			continue
		}
		for _, method := range httpMethods {
			op, ok := item[method].(map[string]interface{})
			if !ok {
				continue
			}

			operationID, _ := op["operationId"].(string)
			summary, _ := op["summary"].(string)
			description, _ := op["description"].(string)

			var tags []string
			if rawTags, ok := op["tags"].([]interface{}); ok {
				for _, t := range rawTags {
					if s, ok := t.(string); ok {
						tags = append(tags, s)
					}
				}
			}

			params, _ := json.Marshal(op["parameters"])
			reqBody, _ := json.Marshal(op["requestBody"])
			responses, _ := json.Marshal(op["responses"])

			if operationID == "" {
				operationID = strings.ToUpper(method) + " " + path
			}

			endpoints = append(endpoints, catalogpkg.APIEndpoint{
				ID:          uuid.NewString(),
				APIGroupID:  apiGroupID,
				ServiceID:   serviceID,
				OrgID:       orgID,
				OperationID: operationID,
				Method:      strings.ToUpper(method),
				Path:        path,
				Summary:     summary,
				Description: description,
				Tags:        tags,
				Parameters:  params,
				RequestBody: reqBody,
				Responses:   responses,
				Order:       order,
				CreatedBy:   actorID,
				CreatedAt:   now,
				UpdatedAt:   now,
			})
			order++
		}
	}
	return endpoints, nil
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
