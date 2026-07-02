package authz

import "testing"

func TestValidScope(t *testing.T) {
	if !ValidScope("diagrams:write") {
		t.Fatal("diagrams:write should be valid")
	}
	if !ValidScope("diagrams:*") {
		t.Fatal("diagrams:* wildcard should be valid")
	}
	if ValidScope("diagrams:explode") {
		t.Fatal("diagrams:explode should be invalid")
	}
	if ValidScope("all:*") {
		t.Fatal("all:* should be invalid")
	}
	if ValidScope("manage:x") {
		t.Fatal("manage:x should be invalid")
	}
	if ValidScope("") {
		t.Fatal("empty scope should be invalid")
	}
}

func TestHas(t *testing.T) {
	scopes := []string{"diagrams:read", "maps:write"}
	if !Has(scopes, ScopeDiagramsRead) {
		t.Fatal("expected diagrams:read to be present")
	}
	if Has(scopes, ScopeDiagramsWrite) {
		t.Fatal("did not expect diagrams:write")
	}
	if Has(nil, ScopeDiagramsRead) {
		t.Fatal("nil scopes should grant nothing")
	}
}

func TestHasWildcard(t *testing.T) {
	scopes := []string{"diagrams:*"}
	if !Has(scopes, ScopeDiagramsRead) {
		t.Fatal("diagrams:* should satisfy diagrams:read")
	}
	if !Has(scopes, ScopeDiagramsWrite) {
		t.Fatal("diagrams:* should satisfy diagrams:write")
	}
	if Has(scopes, ScopeMapsWrite) {
		t.Fatal("diagrams:* must not leak into maps")
	}
}

func TestScopesForRole(t *testing.T) {
	viewer := toStrings(ScopesForRole(RoleViewer))
	if !Has(viewer, ScopeDiagramsRead) {
		t.Fatal("viewer should have diagrams:read")
	}
	if Has(viewer, ScopeDiagramsWrite) {
		t.Fatal("viewer must not have diagrams:write")
	}

	editor := toStrings(ScopesForRole(RoleEditor))
	if !Has(editor, ScopeDiagramsWrite) {
		t.Fatal("editor should have diagrams:write")
	}
	if Has(editor, ScopeTeamsCreate) {
		t.Fatal("editor must not have management write scopes")
	}

	admin := toStrings(ScopesForRole(RoleAdmin))
	for _, s := range AllScopes {
		if !Has(admin, s) {
			t.Fatalf("admin should satisfy %s", s)
		}
	}

	if ScopesForRole(Role("ghost")) != nil {
		t.Fatal("unknown role should resolve to nil (deny)")
	}
}

func toStrings(ss []Scope) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = string(s)
	}
	return out
}
