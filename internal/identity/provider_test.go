package identity

import "testing"

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		slug string
		ok   bool
	}{
		{"acme-okta", true},
		{"okta2", true},
		{"a1", true},
		{"acme-okta-eu-west", true},

		{"", false},
		{"a", false},
		{"Acme-Okta", false},
		{"ACME", false},
		{"-okta", false},
		{"okta-", false},
		{"acme--okta", false},
		{"acme_okta", false},
		{"acme okta", false},
		{"acme.okta", false},
		{"acme/okta", false},
		{"okta ", false},
		{"3f2b9c1e-0a4d-4c8b-9f1e-7d6a5b4c3e2f", true},
	}

	for _, tt := range tests {
		err := ValidateSlug(tt.slug)
		if tt.ok && err != nil {
			t.Errorf("ValidateSlug(%q) = %v, want nil", tt.slug, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ValidateSlug(%q) = nil, want error", tt.slug)
		}
	}

	long := ""
	for i := 0; i < 64; i++ {
		long += "a"
	}
	if err := ValidateSlug(long); err == nil {
		t.Errorf("ValidateSlug(64 chars) = nil, want error")
	}
	if err := ValidateSlug(long[:63]); err != nil {
		t.Errorf("ValidateSlug(63 chars) = %v, want nil", err)
	}
}
