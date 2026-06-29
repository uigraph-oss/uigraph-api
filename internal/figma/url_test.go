package figma

import "testing"

func TestParseURL(t *testing.T) {
	fileKey, nodeID, err := ParseURL("https://www.figma.com/design/abc123XYZ/My-File?node-id=12-345&t=x")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if fileKey != "abc123XYZ" {
		t.Errorf("fileKey = %q, want abc123XYZ", fileKey)
	}
	if nodeID != "12:345" {
		t.Errorf("nodeID = %q, want 12:345", nodeID)
	}
}

func TestParseURLInvalid(t *testing.T) {
	cases := []string{
		"https://example.com/file/abc?node-id=1-2",
		"https://www.figma.com/design/abc123",
		"not a url",
	}
	for _, c := range cases {
		if _, _, err := ParseURL(c); err == nil {
			t.Errorf("ParseURL(%q) expected error", c)
		}
	}
}
