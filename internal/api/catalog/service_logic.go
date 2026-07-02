package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	catalogpkg "github.com/uigraph/app/internal/catalog"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/specparser"
	"github.com/uigraph/app/internal/storage"
	storepkg "github.com/uigraph/app/internal/store"
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

func (h *Handler) downloadSpec(ctx context.Context, key string) (string, error) {
	rc, err := h.storage.Download(ctx, key)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// publishParams drives publishAPIGroupVersion.
type publishParams struct {
	group     *catalogpkg.APIGroup
	serviceID string
	// spec is the spec content written for both the working-copy blob and the new
	// version snapshot. When empty, the current working-copy blob is reused (the
	// working copy is snapshotted as-is).
	spec string
	// endpoints, when non-nil, become the new working-copy endpoints. When nil and
	// spec is non-empty, endpoints are parsed from spec.
	endpoints  []catalogpkg.APIEndpoint
	label      *string
	isAuto     bool
	actorID    string
	commitHash *string
}

// publishAPIGroupVersion uploads the spec blobs and atomically snapshots a new
// version (see store.PublishAPIGroupVersion). It mutates p.group in place with the
// resolved spec key/hash and version label. Any failure surfaces as an error so
// the working copy is never left mutated without a matching snapshot.
func (h *Handler) publishAPIGroupVersion(ctx context.Context, p publishParams) (catalogpkg.APIGroupVersion, error) {
	g := p.group
	now := time.Now().UTC()
	versionID := uuid.NewString()
	vKey := storage.APIGroupVersionSpecKey(g.OrgID, p.serviceID, g.ID, versionID)

	in := catalogpkg.PublishAPIGroupVersionInput{ActorID: p.actorID}
	in.Version = catalogpkg.APIGroupVersion{
		ID: versionID, APIGroupID: g.ID, Label: p.label,
		IsAutoVersion: p.isAuto, CreatedBy: p.actorID,
		CreatedByCommitHash: p.commitHash, CreatedAt: now,
	}

	if p.spec != "" {
		hash := specHash(p.spec)
		wcKey := storage.APIGroupSpecKey(g.OrgID, p.serviceID, g.ID)
		if err := h.uploadSpec(ctx, wcKey, p.spec); err != nil {
			return in.Version, err
		}
		if err := h.uploadSpec(ctx, vKey, p.spec); err != nil {
			return in.Version, err
		}
		g.SpecKey, g.SpecHash = &wcKey, &hash
		in.Version.SpecKey, in.Version.SpecHash = vKey, hash

		eps := p.endpoints
		if eps == nil {
			parsed, err := parseSpecEndpointsForProtocol(p.spec, g.Protocol, g.ID, p.serviceID, g.OrgID, p.actorID, now)
			if err != nil {
				return in.Version, err
			}
			eps = parsed
		}
		in.ReplaceEndpoints = true
		in.NewEndpoints = eps
	} else {
		if g.SpecKey == nil || g.SpecHash == nil {
			return in.Version, fmt.Errorf("api group %s has no spec to version", g.ID)
		}
		content, err := h.downloadSpec(ctx, *g.SpecKey)
		if err != nil {
			return in.Version, err
		}
		if err := h.uploadSpec(ctx, vKey, content); err != nil {
			return in.Version, err
		}
		in.Version.SpecKey, in.Version.SpecHash = vKey, *g.SpecHash
		if p.endpoints != nil {
			in.ReplaceEndpoints = true
			in.NewEndpoints = p.endpoints
		}
	}

	for i := range in.NewEndpoints {
		in.NewEndpoints[i].CreatedByCommitHash = p.commitHash
	}
	in.Group = *g
	v, err := h.store.PublishAPIGroupVersion(ctx, in)
	if err != nil {
		return v, err
	}
	g.Version = *v.Label
	return v, nil
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

// parseSpecEndpointsForProtocol parses a spec into working-copy endpoints for the
// given protocol. It performs no I/O so callers can snapshot the result inside a
// transaction.
func parseSpecEndpointsForProtocol(spec, protocol, apiGroupID, serviceID, orgID, actorID string, now time.Time) ([]catalogpkg.APIEndpoint, error) {
	switch normalizeProtocol(protocol) {
	case "graphql":
		return parseGraphQLSpecEndpoints(spec, apiGroupID, serviceID, orgID, actorID, now)
	case "grpc":
		return parseGrpcSpecEndpoints(spec, apiGroupID, serviceID, orgID, actorID, now)
	default:
		return parseSpecEndpoints(spec, apiGroupID, serviceID, orgID, actorID, now)
	}
}

// normalizeProtocol maps a free-form protocol label (as stored on the API group)
// to the lowercase token used to pick a spec parser.
func normalizeProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "graphql":
		return "graphql"
	case "grpc":
		return "grpc"
	default:
		return "openapi"
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
	if rawPaths == nil {
		return nil, nil
	}

	pathOrder, methodsByPath, orderErr := parseOpenAPIDocumentOrder(spec)
	if orderErr != nil {
		pathOrder = sortedPathKeys(rawPaths)
		methodsByPath = map[string][]string{}
	} else {
		seen := make(map[string]bool, len(pathOrder))
		for _, path := range pathOrder {
			seen[path] = true
		}
		var extras []string
		for path := range rawPaths {
			if !seen[path] {
				extras = append(extras, path)
			}
		}
		sort.Strings(extras)
		pathOrder = append(pathOrder, extras...)
	}

	var endpoints []catalogpkg.APIEndpoint
	order := 0.0
	for _, path := range pathOrder {
		pathItem, ok := rawPaths[path]
		if !ok {
			continue
		}
		item, ok := pathItem.(map[string]interface{})
		if !ok {
			continue
		}
		for _, method := range methodsForPathItem(path, item, methodsByPath) {
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
			resolvedReqBody := resolveRefsInDoc(doc, op["requestBody"])
			resolvedResponses := resolveRefsInDoc(doc, op["responses"])
			normalizedReqBody := normalizeRequestBodyForStorage(resolvedReqBody)
			normalizedResponses := normalizeResponsesForStorage(resolvedResponses)
			reqBody, _ := json.Marshal(normalizedReqBody)
			responses, _ := json.Marshal(normalizedResponses)

			if operationID == "" {
				operationID = strings.ToUpper(method) + " " + path
			}

			endpoints = append(endpoints, catalogpkg.APIEndpoint{
				ID:               uuid.NewString(),
				APIGroupID:       apiGroupID,
				ServiceID:        serviceID,
				OrgID:            orgID,
				OperationID:      operationID,
				Method:           strings.ToUpper(method),
				Path:             path,
				Summary:          summary,
				Description:      description,
				Tags:             tags,
				Parameters:       params,
				RequestBody:      reqBody,
				Responses:        responses,
				ExampleRequests:  seedExampleSamplesJSON(reqBody),
				ExampleResponses: seedExampleSamplesJSON(responses),
				Order:            order,
				CreatedBy:        actorID,
				CreatedAt:        now,
				UpdatedAt:        now,
			})
			order++
		}
	}
	return endpoints, nil
}

// parseGraphQLSpecEndpoints parses a GraphQL SDL document and returns one
// APIEndpoint per root Query/Mutation/Subscription field.
func parseGraphQLSpecEndpoints(spec, apiGroupID, serviceID, orgID, actorID string, now time.Time) ([]catalogpkg.APIEndpoint, error) {
	ops, err := specparser.ParseGraphQL([]byte(spec))
	if err != nil {
		return nil, err
	}

	endpoints := make([]catalogpkg.APIEndpoint, 0, len(ops))
	for i, op := range ops {
		requestBody, _ := json.Marshal(map[string]string{"query": op.RequestExample})

		responses := json.RawMessage(op.ResponseExample)
		if !json.Valid(responses) {
			responses, _ = json.Marshal(map[string]string{})
		}
		parameters := json.RawMessage(op.Variables)
		if !json.Valid(parameters) {
			parameters, _ = json.Marshal(map[string]string{})
		}

		endpoints = append(endpoints, catalogpkg.APIEndpoint{
			ID:               uuid.NewString(),
			APIGroupID:       apiGroupID,
			ServiceID:        serviceID,
			OrgID:            orgID,
			OperationID:      op.OperationID,
			Method:           op.Kind,
			Path:             op.Signature,
			Description:      op.Description,
			Tags:             op.Tags,
			Parameters:       parameters,
			RequestBody:      requestBody,
			Responses:        responses,
			ExampleRequests:  seedExampleSamplesJSON(requestBody),
			ExampleResponses: seedExampleSamplesJSON(responses),
			Order:            float64(i),
			CreatedBy:        actorID,
			CreatedAt:        now,
			UpdatedAt:        now,
		})
	}
	return endpoints, nil
}

// parseGrpcSpecEndpoints parses a .proto document and returns one APIEndpoint
// per RPC method. Package/service/request/response type names are packed into
// the Parameters column as a small JSON object since the generic APIEndpoint
// shape only has a handful of scalar text columns.
func parseGrpcSpecEndpoints(spec, apiGroupID, serviceID, orgID, actorID string, now time.Time) ([]catalogpkg.APIEndpoint, error) {
	methods, err := specparser.ParseGrpc([]byte(spec))
	if err != nil {
		return nil, err
	}

	endpoints := make([]catalogpkg.APIEndpoint, 0, len(methods))
	for i, m := range methods {
		grpcMeta, _ := json.Marshal(map[string]string{
			"packageName":  m.PackageName,
			"serviceName":  m.ServiceName,
			"requestType":  m.RequestType,
			"responseType": m.ResponseType,
		})

		requestExample := json.RawMessage(m.RequestExample)
		if !json.Valid(requestExample) {
			requestExample, _ = json.Marshal(map[string]string{})
		}
		responseExample := json.RawMessage(m.ResponseExample)
		if !json.Valid(responseExample) {
			responseExample, _ = json.Marshal(map[string]string{})
		}

		path := m.MethodID
		if path != "" && !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		endpoints = append(endpoints, catalogpkg.APIEndpoint{
			ID:               uuid.NewString(),
			APIGroupID:       apiGroupID,
			ServiceID:        serviceID,
			OrgID:            orgID,
			OperationID:      m.MethodName,
			Method:           m.StreamingType,
			Path:             path,
			Summary:          m.ProtoSnippet,
			Description:      m.Description,
			Tags:             m.Tags,
			Parameters:       grpcMeta,
			RequestBody:      requestExample,
			Responses:        responseExample,
			ExampleRequests:  seedExampleSamplesJSON(requestExample),
			ExampleResponses: seedExampleSamplesJSON(responseExample),
			Order:            float64(i),
			CreatedBy:        actorID,
			CreatedAt:        now,
			UpdatedAt:        now,
		})
	}
	return endpoints, nil
}
