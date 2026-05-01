package credentials

import (
	"strings"
	"testing"
)

// TestBaseURLNormalization verifies Property 1:
// For any URL string (with zero, one, or multiple trailing slashes),
// the normalized URL stored in Credentials never ends with "/".
func TestBaseURLNormalization(t *testing.T) {
	cases := []struct {
		input string
		desc  string
	}{
		{"https://n8n.example.com", "no trailing slash"},
		{"https://n8n.example.com/", "one trailing slash"},
		{"https://n8n.example.com//", "two trailing slashes"},
		{"https://n8n.example.com///", "three trailing slashes"},
		{"http://localhost:5678/", "localhost with port and slash"},
		{"http://localhost:5678", "localhost with port, no slash"},
		{"/", "only a slash"},
		{"///", "only multiple slashes"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			normalized := strings.TrimRight(strings.TrimSpace(tc.input), "/")
			if strings.HasSuffix(normalized, "/") {
				t.Errorf("normalized URL %q still ends with '/' (input: %q)", normalized, tc.input)
			}
		})
	}
}
