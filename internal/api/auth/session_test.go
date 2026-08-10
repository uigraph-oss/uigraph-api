package auth

import "testing"

func TestValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "email", input: "sayadenv@hotmail.com", want: "sayadenv@hotmail.com", ok: true},
		{name: "trimmed", input: "  sayadenv@hotmail.com  ", want: "sayadenv@hotmail.com", ok: true},
		{name: "empty", input: "", ok: false},
		{name: "opaque name id", input: "0bb008b0-a328-4cc2-8af4-013dc91b2c63", ok: false},
		{name: "missing local part", input: "@hotmail.com", ok: false},
		{name: "missing domain", input: "sayadenv@", ok: false},
		{name: "display name", input: "Nazmus Sayad <sayadenv@hotmail.com>", ok: false},
		{name: "multiple addresses", input: "one@example.com, two@example.com", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := validEmail(test.input)
			if got != test.want || ok != test.ok {
				t.Fatalf("validEmail(%q) = (%q, %t), want (%q, %t)", test.input, got, ok, test.want, test.ok)
			}
		})
	}
}
