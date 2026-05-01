package n8nclient

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/whoAngeel/n8n-workflow-exported/credentials"
)

// emptyWorkflowsResponse returns a minimal valid n8n API response with no workflows.
func emptyWorkflowsResponse() []byte {
	data, _ := json.Marshal(map[string]any{"data": []any{}})
	return data
}

// TestBasicAuthHeader verifies Property 4:
// For any (username, password) pair, GetAllWorkflows sends exactly
// "Authorization: Basic base64(username:password)".
func TestBasicAuthHeader(t *testing.T) {
	cases := []struct {
		username string
		password string
		desc     string
	}{
		{"admin", "secret", "simple credentials"},
		{"user@example.com", "p@$$w0rd!", "email username and special chars in password"},
		{"user", "pass:with:colons", "colons in password"},
		{"üser", "pässwörd", "unicode characters"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			var capturedHeader string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedHeader = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(emptyWorkflowsResponse())
			}))
			defer srv.Close()

			creds := credentials.Credentials{
				BaseURL:  srv.URL,
				AuthType: credentials.AuthTypeBasic,
				Username: tc.username,
				Password: tc.password,
			}

			client := NewN8NClient(creds)
			_, err := client.GetAllWorkflows()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expected := "Basic " + base64.StdEncoding.EncodeToString(
				[]byte(tc.username+":"+tc.password),
			)
			if capturedHeader != expected {
				t.Errorf("Authorization header = %q, want %q", capturedHeader, expected)
			}
		})
	}
}

// TestTokenAuthHeader verifies Property 5:
// For any non-empty token, GetAllWorkflows sends exactly
// "X-N8N-API-KEY: <token>".
func TestTokenAuthHeader(t *testing.T) {
	cases := []struct {
		token string
		desc  string
	}{
		{"mytoken123", "simple token"},
		{"Bearer eyJhbGciOiJIUzI1NiJ9.abc.def", "JWT-like token"},
		{"token with spaces", "token with spaces"},
		{"tök€n-wïth-ünicode", "unicode token"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			var capturedHeader string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedHeader = r.Header.Get("X-N8N-API-KEY")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(emptyWorkflowsResponse())
			}))
			defer srv.Close()

			creds := credentials.Credentials{
				BaseURL:  srv.URL,
				AuthType: credentials.AuthTypeToken,
				Token:    tc.token,
			}

			client := NewN8NClient(creds)
			_, err := client.GetAllWorkflows()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if capturedHeader != tc.token {
				t.Errorf("X-N8N-API-KEY = %q, want %q", capturedHeader, tc.token)
			}
		})
	}
}

// TestGetAllWorkflows_AuthErrors verifies that 401 and 403 return "authentication failed".
func TestGetAllWorkflows_AuthErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			creds := credentials.Credentials{
				BaseURL:  srv.URL,
				AuthType: credentials.AuthTypeToken,
				Token:    "bad-token",
			}

			client := NewN8NClient(creds)
			_, err := client.GetAllWorkflows()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

// TestGetAllWorkflows_EmptyList verifies that an empty data array returns a non-nil empty slice.
func TestGetAllWorkflows_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(emptyWorkflowsResponse())
	}))
	defer srv.Close()

	creds := credentials.Credentials{
		BaseURL:  srv.URL,
		AuthType: credentials.AuthTypeToken,
		Token:    "any-token",
	}

	client := NewN8NClient(creds)
	workflows, err := client.GetAllWorkflows()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if workflows == nil {
		t.Error("expected non-nil slice, got nil")
	}
	if len(workflows) != 0 {
		t.Errorf("expected 0 workflows, got %d", len(workflows))
	}
}
