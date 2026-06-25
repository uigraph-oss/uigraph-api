package specparser

import (
	"encoding/json"
	"testing"
)

const sampleSchema = `
type Project {
	id: ID!
	name: String!
}

input CreateProjectInput {
	name: String!
}

type Query {
	v1GetProject(orgId: String!): Project
}

type Mutation {
	v1CreateProject(input: CreateProjectInput!): Project
}

type Subscription {
	v1ProjectUpdated(orgId: String!): Project
}
`

func TestParseGraphQL_extractsOneOperationPerRootField(t *testing.T) {
	ops, err := ParseGraphQL([]byte(sampleSchema))
	if err != nil {
		t.Fatalf("ParseGraphQL returned error: %v", err)
	}
	if len(ops) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(ops))
	}
}

func TestParseGraphQL_classifiesOperationKind(t *testing.T) {
	ops, err := ParseGraphQL([]byte(sampleSchema))
	if err != nil {
		t.Fatalf("ParseGraphQL returned error: %v", err)
	}

	kinds := map[string]string{}
	for _, op := range ops {
		kinds[op.Name] = op.Kind
	}

	if kinds["v1GetProject"] != "Query" {
		t.Fatalf("expected v1GetProject to be a Query, got %q", kinds["v1GetProject"])
	}
	if kinds["v1CreateProject"] != "Mutation" {
		t.Fatalf("expected v1CreateProject to be a Mutation, got %q", kinds["v1CreateProject"])
	}
	if kinds["v1ProjectUpdated"] != "Subscription" {
		t.Fatalf("expected v1ProjectUpdated to be a Subscription, got %q", kinds["v1ProjectUpdated"])
	}
}

func TestParseGraphQL_buildsReadableSignature(t *testing.T) {
	ops, err := ParseGraphQL([]byte(sampleSchema))
	if err != nil {
		t.Fatalf("ParseGraphQL returned error: %v", err)
	}

	for _, op := range ops {
		if op.Name != "v1GetProject" {
			continue
		}
		want := "v1GetProject(orgId: String!): Project"
		if op.Signature != want {
			t.Fatalf("expected signature %q, got %q", want, op.Signature)
		}
		return
	}
	t.Fatal("v1GetProject operation not found")
}

func TestParseGraphQL_variablesAndExamplesAreValidJSON(t *testing.T) {
	ops, err := ParseGraphQL([]byte(sampleSchema))
	if err != nil {
		t.Fatalf("ParseGraphQL returned error: %v", err)
	}

	for _, op := range ops {
		if !json.Valid([]byte(op.Variables)) {
			t.Fatalf("operation %s: Variables is not valid JSON: %s", op.Name, op.Variables)
		}
		if !json.Valid([]byte(op.ResponseExample)) {
			t.Fatalf("operation %s: ResponseExample is not valid JSON: %s", op.Name, op.ResponseExample)
		}
		if op.RequestExample == "" {
			t.Fatalf("operation %s: expected a non-empty RequestExample", op.Name)
		}
	}
}

func TestParseGraphQL_deprecatedFieldGetsTagAndNote(t *testing.T) {
	schema := `
	type Query {
		legacyField: String @deprecated(reason: "use newField instead")
	}
	`
	ops, err := ParseGraphQL([]byte(schema))
	if err != nil {
		t.Fatalf("ParseGraphQL returned error: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	found := false
	for _, tag := range op.Tags {
		if tag == "deprecated" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'deprecated' tag, got %v", op.Tags)
	}
	if op.Description == "" {
		t.Fatal("expected deprecation note in description")
	}
}

func TestParseGraphQL_invalidSchemaReturnsError(t *testing.T) {
	_, err := ParseGraphQL([]byte("not a graphql schema {{{"))
	if err == nil {
		t.Fatal("expected an error for invalid schema")
	}
}

func TestParseGraphQL_emptyInputReturnsError(t *testing.T) {
	_, err := ParseGraphQLSchemas(nil)
	if err == nil {
		t.Fatal("expected an error for empty schema contents")
	}
}
