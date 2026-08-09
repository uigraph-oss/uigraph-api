package rolemap

import (
	"strings"
	"testing"

	"github.com/uigraph/app/internal/authz"
)

func TestMatchOperators(t *testing.T) {
	attrs := map[string]any{
		"role":           "editor",
		"groups":         []any{"eng", "platform-admin", "oncall"},
		"email":          "ada@example.com",
		"email_verified": true,
		"seats":          float64(12),
		"user": map[string]any{
			"role":   "admin",
			"groups": []any{"nested-eng"},
		},
		"empty": []any{},
		"http://schemas.microsoft.com/ws/2008/06/identity/claims/groups":     "f2028123-ce2d-4a3b-adaa-0970d6863ab0",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress": "ada@example.com",
	}

	tests := []struct {
		name  string
		key   string
		op    string
		value string
		want  bool
	}{
		{"equals scalar hit", "role", OpEquals, "editor", true},
		{"equals scalar miss", "role", OpEquals, "viewer", false},
		{"equals array hit", "groups", OpEquals, "oncall", true},
		{"equals array miss", "groups", OpEquals, "sre", false},
		{"equals absent", "missing", OpEquals, "anything", false},
		{"equals bool", "email_verified", OpEquals, "true", true},
		{"equals number", "seats", OpEquals, "12", true},

		{"notEquals scalar hit", "role", OpNotEquals, "viewer", true},
		{"notEquals scalar miss", "role", OpNotEquals, "editor", false},
		{"notEquals array excludes member", "groups", OpNotEquals, "eng", false},
		{"notEquals array non-member", "groups", OpNotEquals, "sre", true},
		{"notEquals absent is false", "missing", OpNotEquals, "sre", false},

		{"contains scalar", "email", OpContains, "@example.", true},
		{"contains array", "groups", OpContains, "admin", true},
		{"contains miss", "groups", OpContains, "finance", false},
		{"notContains hit", "groups", OpNotContains, "finance", true},
		{"notContains miss", "groups", OpNotContains, "admin", false},
		{"notContains absent is false", "missing", OpNotContains, "finance", false},

		{"startsWith scalar", "email", OpStartsWith, "ada", true},
		{"startsWith array", "groups", OpStartsWith, "platform-", true},
		{"startsWith miss", "email", OpStartsWith, "bob", false},
		{"endsWith scalar", "email", OpEndsWith, "example.com", true},
		{"endsWith array", "groups", OpEndsWith, "-admin", true},
		{"endsWith miss", "email", OpEndsWith, ".org", false},

		{"exists present", "role", OpExists, "", true},
		{"exists absent", "missing", OpExists, "", false},
		{"exists empty array is present", "empty", OpExists, "", true},
		{"notExists absent", "missing", OpNotExists, "", true},
		{"notExists present", "role", OpNotExists, "", false},

		{"regex scalar", "email", OpRegex, `^ada@.*\.com$`, true},
		{"regex array", "groups", OpRegex, `^platform-\w+$`, true},
		{"regex miss", "email", OpRegex, `^bob@`, false},

		{"dot notation equals", "user.role", OpEquals, "admin", true},
		{"dot notation array", "user.groups", OpContains, "nested", true},
		{"dot notation missing leaf", "user.missing", OpExists, "", false},
		{"dot notation through scalar", "role.nope", OpExists, "", false},

		{"saml claim uri equals", "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups", OpEquals, "f2028123-ce2d-4a3b-adaa-0970d6863ab0", true},
		{"saml claim uri equals miss", "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups", OpEquals, "other-group", false},
		{"saml claim uri exists", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress", OpExists, "", true},
		{"saml claim uri contains", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress", OpContains, "@example.", true},
		{"saml claim uri absent", "http://schemas.microsoft.com/ws/2008/06/identity/claims/role", OpExists, "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Match(Rule{AttributeKey: tc.key, Operator: tc.op, AttributeValue: tc.value}, attrs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Match(%s %s %q) = %v, want %v", tc.key, tc.op, tc.value, got, tc.want)
			}
		})
	}
}

func TestMatchUnknownOperatorErrors(t *testing.T) {
	_, err := Match(Rule{ID: "r1", AttributeKey: "role", Operator: "isSortOf", AttributeValue: "admin"}, map[string]any{"role": "admin"})
	if err == nil {
		t.Fatal("expected an error for an unknown operator")
	}
	if !strings.Contains(err.Error(), "isSortOf") {
		t.Fatalf("error should name the operator, got: %v", err)
	}
}

func TestMatchInvalidRegexErrors(t *testing.T) {
	_, err := Match(Rule{AttributeKey: "email", Operator: OpRegex, AttributeValue: "([a-z"}, map[string]any{"email": "ada@example.com"})
	if err == nil {
		t.Fatal("expected an error for an invalid regex")
	}
}

func TestResolveFirstMatchByPriority(t *testing.T) {
	rules := []Rule{
		{ID: "c", Priority: 30, AttributeKey: "groups", Operator: OpExists, Role: authz.RoleViewer},
		{ID: "a", Priority: 10, AttributeKey: "groups", Operator: OpEquals, AttributeValue: "sre", Role: authz.RoleAdmin},
		{ID: "b", Priority: 20, AttributeKey: "groups", Operator: OpEquals, AttributeValue: "eng", Role: authz.RoleEditor},
	}
	attrs := map[string]any{"groups": []any{"eng", "sre"}}

	got, err := Resolve(rules, attrs, authz.RoleViewer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != authz.RoleAdmin {
		t.Fatalf("expected the priority-10 rule to win, got %q", got)
	}
}

func TestResolveDoesNotMutateInput(t *testing.T) {
	rules := []Rule{
		{ID: "c", Priority: 30, AttributeKey: "groups", Operator: OpExists, Role: authz.RoleViewer},
		{ID: "a", Priority: 10, AttributeKey: "groups", Operator: OpEquals, AttributeValue: "sre", Role: authz.RoleAdmin},
	}
	if _, err := Resolve(rules, map[string]any{"groups": []any{"sre"}}, authz.RoleViewer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules[0].ID != "c" || rules[1].ID != "a" {
		t.Fatal("Resolve must not reorder the caller's slice")
	}
}

func TestResolveFallsBackToDefaultRole(t *testing.T) {
	rules := []Rule{
		{Priority: 10, AttributeKey: "groups", Operator: OpEquals, AttributeValue: "sre", Role: authz.RoleAdmin},
	}

	got, err := Resolve(rules, map[string]any{"groups": []any{"finance"}}, authz.RoleEditor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != authz.RoleEditor {
		t.Fatalf("expected the default role, got %q", got)
	}

	got, err = Resolve(nil, map[string]any{}, authz.RoleViewer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != authz.RoleViewer {
		t.Fatalf("expected the default role with no rules, got %q", got)
	}
}

func TestResolvePropagatesRuleErrors(t *testing.T) {
	rules := []Rule{
		{ID: "bad", Priority: 10, AttributeKey: "groups", Operator: "nonsense", Role: authz.RoleAdmin},
		{ID: "ok", Priority: 20, AttributeKey: "groups", Operator: OpExists, Role: authz.RoleEditor},
	}

	got, err := Resolve(rules, map[string]any{"groups": []any{"eng"}}, authz.RoleViewer)
	if err == nil {
		t.Fatal("expected a malformed rule to fail the resolution")
	}
	if got != "" {
		t.Fatalf("expected no role alongside the error, got %q", got)
	}
}

func TestValidateRule(t *testing.T) {
	valid := Rule{AttributeKey: "groups", Operator: OpEquals, AttributeValue: "eng", Role: authz.RoleEditor}
	if err := ValidateRule(valid); err != nil {
		t.Fatalf("expected a valid rule, got: %v", err)
	}

	existsNoValue := Rule{AttributeKey: "groups", Operator: OpExists, Role: authz.RoleViewer}
	if err := ValidateRule(existsNoValue); err != nil {
		t.Fatalf("exists should not require a value, got: %v", err)
	}

	invalid := []struct {
		name string
		rule Rule
	}{
		{"blank key", Rule{AttributeKey: "  ", Operator: OpEquals, AttributeValue: "eng", Role: authz.RoleEditor}},
		{"unknown operator", Rule{AttributeKey: "groups", Operator: "nonsense", AttributeValue: "eng", Role: authz.RoleEditor}},
		{"missing value", Rule{AttributeKey: "groups", Operator: OpEquals, Role: authz.RoleEditor}},
		{"bad regex", Rule{AttributeKey: "groups", Operator: OpRegex, AttributeValue: "([a-z", Role: authz.RoleEditor}},
		{"unknown role", Rule{AttributeKey: "groups", Operator: OpEquals, AttributeValue: "eng", Role: authz.Role("owner")}},
		{"server_admin is not an org role", Rule{AttributeKey: "groups", Operator: OpEquals, AttributeValue: "eng", Role: authz.Role("server_admin")}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateRule(tc.rule); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestValidOperatorCatalog(t *testing.T) {
	for _, op := range ValidOperators {
		if !ValidOperator(op) {
			t.Fatalf("%q is in the catalog but ValidOperator rejects it", op)
		}
		if _, err := Match(Rule{AttributeKey: "role", Operator: op, AttributeValue: "x"}, map[string]any{"role": "x"}); err != nil {
			t.Fatalf("catalog operator %q is not implemented by Match: %v", op, err)
		}
	}
	if ValidOperator("") || ValidOperator("equalsIgnoreCase") {
		t.Fatal("ValidOperator should reject operators outside the catalog")
	}
	if TakesValue(OpExists) || TakesValue(OpNotExists) {
		t.Fatal("the existence operators do not take a value")
	}
	if !TakesValue(OpEquals) {
		t.Fatal("equals takes a value")
	}
}
