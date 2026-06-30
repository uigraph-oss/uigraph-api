package catalog

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	catalogpkg "github.com/uigraph/app/internal/catalog"
)

// ── toSlug ────────────────────────────────────────────────────────────────────

func TestToSlug_lowercasesInput(t *testing.T) {
	if got := toSlug("Hello World"); got != "hello-world" {
		t.Fatalf("expected hello-world, got %q", got)
	}
}

func TestToSlug_replacesSpacesWithHyphens(t *testing.T) {
	if got := toSlug("my service name"); got != "my-service-name" {
		t.Fatalf("expected my-service-name, got %q", got)
	}
}

func TestToSlug_replacesSpecialChars(t *testing.T) {
	if got := toSlug("order-service@v2"); got != "order-service-v2" {
		t.Fatalf("expected order-service-v2, got %q", got)
	}
}

func TestToSlug_preservesDotsAndHyphens(t *testing.T) {
	if got := toSlug("api.v1-service"); got != "api.v1-service" {
		t.Fatalf("expected api.v1-service, got %q", got)
	}
}

func TestToSlug_preservesNumbers(t *testing.T) {
	if got := toSlug("service123"); got != "service123" {
		t.Fatalf("expected service123, got %q", got)
	}
}

func TestToSlug_collapsesConsecutiveHyphens(t *testing.T) {
	if got := toSlug("foo  bar"); got != "foo-bar" {
		t.Fatalf("expected foo-bar (collapsed hyphens), got %q", got)
	}
}

func TestToSlug_trimsLeadingAndTrailingHyphens(t *testing.T) {
	if got := toSlug("@service@"); got != "service" {
		t.Fatalf("expected service (trimmed), got %q", got)
	}
}

func TestToSlug_emptyStringIsEmpty(t *testing.T) {
	if got := toSlug(""); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

// ── specHash / sha256Bytes ────────────────────────────────────────────────────

func TestSpecHash_isDeterministic(t *testing.T) {
	input := `{"openapi":"3.0.0"}`
	h1 := specHash(input)
	h2 := specHash(input)
	if h1 != h2 {
		t.Fatalf("specHash is not deterministic: %q vs %q", h1, h2)
	}
}

func TestSpecHash_isHexString(t *testing.T) {
	h := specHash("anything")
	const hexChars = "0123456789abcdef"
	for _, c := range h {
		if !strings.ContainsRune(hexChars, c) {
			t.Fatalf("specHash contains non-hex character %q in %q", c, h)
		}
	}
	// SHA-256 produces 64 hex characters
	if len(h) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(h))
	}
}

func TestSpecHash_differentInputsProduceDifferentHashes(t *testing.T) {
	h1 := specHash("version: 1")
	h2 := specHash("version: 2")
	if h1 == h2 {
		t.Fatal("different inputs must not produce the same hash")
	}
}

func TestSha256Bytes_matchesSpecHashForSameContent(t *testing.T) {
	content := "hello world"
	fromString := specHash(content)
	fromBytes := sha256Bytes([]byte(content))
	if fromString != fromBytes {
		t.Fatalf("specHash and sha256Bytes disagree: %q vs %q", fromString, fromBytes)
	}
}

func TestSha256Bytes_emptyInput(t *testing.T) {
	h := sha256Bytes([]byte{})
	if len(h) != 64 {
		t.Fatalf("expected 64 hex chars for empty input, got %d", len(h))
	}
}

// ── normalizeProtocol ────────────────────────────────────────────────────────

func TestNormalizeProtocol_recognizesGraphQLCaseInsensitively(t *testing.T) {
	for _, in := range []string{"GraphQL", "graphql", "GRAPHQL", "  GraphQL  "} {
		if got := normalizeProtocol(in); got != "graphql" {
			t.Fatalf("normalizeProtocol(%q) = %q, want graphql", in, got)
		}
	}
}

func TestNormalizeProtocol_recognizesGrpcCaseInsensitively(t *testing.T) {
	for _, in := range []string{"gRPC", "grpc", "GRPC"} {
		if got := normalizeProtocol(in); got != "grpc" {
			t.Fatalf("normalizeProtocol(%q) = %q, want grpc", in, got)
		}
	}
}

func TestNormalizeProtocol_defaultsToOpenAPI(t *testing.T) {
	for _, in := range []string{"REST", "OpenAPI", "Swagger", "", "anything-else"} {
		if got := normalizeProtocol(in); got != "openapi" {
			t.Fatalf("normalizeProtocol(%q) = %q, want openapi", in, got)
		}
	}
}

// ── parseGraphQLSpecEndpoints ────────────────────────────────────────────────

const testGraphQLSchema = `
type Project {
	id: ID!
	name: String!
}

type Query {
	v1GetProject(orgId: String!): Project
}

type Mutation {
	v1CreateProject(name: String!): Project
}
`

func TestParseGraphQLSpecEndpoints_mapsOneEndpointPerOperation(t *testing.T) {
	endpoints, err := parseGraphQLSpecEndpoints(testGraphQLSchema, "group-1", "svc-1", "org-1", "user-1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}
}

func TestParseGraphQLSpecEndpoints_mapsKindToMethodAndSignatureToPath(t *testing.T) {
	endpoints, err := parseGraphQLSpecEndpoints(testGraphQLSchema, "group-1", "svc-1", "org-1", "user-1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byOperationID := map[string]catalogpkg.APIEndpoint{}
	for _, e := range endpoints {
		byOperationID[e.OperationID] = e
	}

	getProject, ok := byOperationID["v1GetProject"]
	if !ok {
		t.Fatal("expected v1GetProject endpoint")
	}
	if getProject.Method != "Query" {
		t.Fatalf("expected Method=Query, got %q", getProject.Method)
	}
	if getProject.Path != "v1GetProject(orgId: String!): Project" {
		t.Fatalf("expected Path to be the field signature, got %q", getProject.Path)
	}

	createProject, ok := byOperationID["v1CreateProject"]
	if !ok {
		t.Fatal("expected v1CreateProject endpoint")
	}
	if createProject.Method != "Mutation" {
		t.Fatalf("expected Method=Mutation, got %q", createProject.Method)
	}
}

func TestParseGraphQLSpecEndpoints_jsonColumnsAreValidJSON(t *testing.T) {
	endpoints, err := parseGraphQLSpecEndpoints(testGraphQLSchema, "group-1", "svc-1", "org-1", "user-1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, e := range endpoints {
		if !json.Valid(e.Parameters) {
			t.Fatalf("endpoint %s: Parameters is not valid JSON: %s", e.OperationID, e.Parameters)
		}
		if !json.Valid(e.RequestBody) {
			t.Fatalf("endpoint %s: RequestBody is not valid JSON: %s", e.OperationID, e.RequestBody)
		}
		if !json.Valid(e.Responses) {
			t.Fatalf("endpoint %s: Responses is not valid JSON: %s", e.OperationID, e.Responses)
		}
	}
}

func TestParseGraphQLSpecEndpoints_invalidSchemaReturnsError(t *testing.T) {
	_, err := parseGraphQLSpecEndpoints("not a graphql schema {{{", "group-1", "svc-1", "org-1", "user-1", time.Now())
	if err == nil {
		t.Fatal("expected an error for an invalid schema")
	}
}

// ── parseGrpcSpecEndpoints ───────────────────────────────────────────────────

const testProtoSchema = `
syntax = "proto3";

package helloworld;

message HelloRequest {
	string name = 1;
}

message HelloReply {
	string message = 1;
}

service Greeter {
	rpc SayHello(HelloRequest) returns (HelloReply);
}
`

func TestParseGrpcSpecEndpoints_mapsOneEndpointPerMethod(t *testing.T) {
	endpoints, err := parseGrpcSpecEndpoints(testProtoSchema, "group-1", "svc-1", "org-1", "user-1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
}

func TestParseGrpcSpecEndpoints_mapsStreamingTypeAndPath(t *testing.T) {
	endpoints, err := parseGrpcSpecEndpoints(testProtoSchema, "group-1", "svc-1", "org-1", "user-1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	e := endpoints[0]
	if e.OperationID != "SayHello" {
		t.Fatalf("expected OperationID=SayHello, got %q", e.OperationID)
	}
	if e.Method != "UNARY" {
		t.Fatalf("expected Method=UNARY, got %q", e.Method)
	}
	if e.Path != "/helloworld.Greeter/SayHello" {
		t.Fatalf("expected gRPC-style path, got %q", e.Path)
	}
}

func TestParseGrpcSpecEndpoints_packsServiceMetadataIntoParameters(t *testing.T) {
	endpoints, err := parseGrpcSpecEndpoints(testProtoSchema, "group-1", "svc-1", "org-1", "user-1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var meta struct {
		PackageName  string `json:"packageName"`
		ServiceName  string `json:"serviceName"`
		RequestType  string `json:"requestType"`
		ResponseType string `json:"responseType"`
	}
	if err := json.Unmarshal(endpoints[0].Parameters, &meta); err != nil {
		t.Fatalf("Parameters is not valid JSON: %v", err)
	}
	if meta.PackageName != "helloworld" || meta.ServiceName != "Greeter" {
		t.Fatalf("unexpected service metadata: %+v", meta)
	}
	if meta.RequestType != "HelloRequest" || meta.ResponseType != "HelloReply" {
		t.Fatalf("unexpected type metadata: %+v", meta)
	}
}

func TestParseGrpcSpecEndpoints_invalidProtoReturnsError(t *testing.T) {
	_, err := parseGrpcSpecEndpoints("not a proto file {{{", "group-1", "svc-1", "org-1", "user-1", time.Now())
	if err == nil {
		t.Fatal("expected an error for an invalid proto file")
	}
}

// ── parseSpecEndpoints / $ref resolution ─────────────────────────────────────

const testOpenAPISpecWithRef = `{
  "openapi": "3.0.0",
  "components": {
    "schemas": {
      "Order": {
        "type": "object",
        "required": ["id"],
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "total_amount": {"type": "number"}
        }
      }
    }
  },
  "paths": {
    "/checkout": {
      "post": {
        "operationId": "createOrder",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/Order"}
            }
          }
        },
        "responses": {
          "201": {
            "description": "Order created",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/Order"}
              }
            }
          }
        }
      }
    }
  }
}`
