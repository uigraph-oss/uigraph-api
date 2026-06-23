package catalog

import (
	"context"
	"crypto/sha256"
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
