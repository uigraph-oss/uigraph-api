package identity

import "testing"

func TestNormalizeOrgDomain(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
		valid bool
	}{
		{name: "standard", value: "example.com", want: "example.com", valid: true},
		{name: "normalizes", value: " @Sub.Example.COM ", want: "sub.example.com", valid: true},
		{name: "hyphen", value: "my-org.example.co.uk", want: "my-org.example.co.uk", valid: true},
		{name: "missing suffix", value: "localhost", valid: false},
		{name: "url", value: "https://example.com", valid: false},
		{name: "email", value: "user@example.com", valid: false},
		{name: "leading hyphen", value: "-org.example.com", valid: false},
		{name: "trailing hyphen", value: "org-.example.com", valid: false},
		{name: "empty label", value: "org..com", valid: false},
		{name: "numeric suffix", value: "example.123", valid: false},
		{name: "underscore", value: "my_org.example.com", valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeOrgDomain(test.value)
			if test.valid && err != nil {
				t.Fatalf("NormalizeOrgDomain() error = %v", err)
			}
			if !test.valid && err == nil {
				t.Fatalf("NormalizeOrgDomain() = %q, want error", got)
			}
			if got != test.want {
				t.Fatalf("NormalizeOrgDomain() = %q, want %q", got, test.want)
			}
		})
	}
}
