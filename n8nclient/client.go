// Package n8nclient provides an HTTP client for the n8n REST API.
package n8nclient

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/whoAngeel/n8n-workflow-exported/credentials"
)

// WorkflowSummary is the minimal representation returned by the list endpoint.
type WorkflowSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Workflow holds the full workflow data as returned by the API.
// Raw contains the complete JSON object without any modification.
type Workflow struct {
	ID   string
	Name string
	Raw  map[string]any // full JSON as returned by the API, unmodified
}

// N8NClient wraps an http.Client configured for a specific n8n instance.
type N8NClient struct {
	creds      credentials.Credentials
	httpClient *http.Client
}

// NewN8NClient creates a client with a 30-second timeout. No network calls are made.
func NewN8NClient(creds credentials.Credentials) *N8NClient {
	return &N8NClient{
		creds: creds,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetAllWorkflows fetches the complete list of workflows from the n8n instance.
// Returns an empty (non-nil) slice if the instance has no workflows.
func (c *N8NClient) GetAllWorkflows() ([]Workflow, error) {
	url := c.creds.BaseURL + "/api/v1/workflows"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	// Apply authentication header based on AuthType.
	switch c.creds.AuthType {
	case credentials.AuthTypeBasic:
		encoded := base64.StdEncoding.EncodeToString(
			[]byte(c.creds.Username + ":" + c.creds.Password),
		)
		req.Header.Set("Authorization", "Basic "+encoded)
	case credentials.AuthTypeToken:
		req.Header.Set("X-N8N-API-KEY", c.creds.Token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to n8n: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	case http.StatusOK:
		// handled below
	default:
		return nil, fmt.Errorf("unexpected status from n8n API: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// n8n returns { "data": [ {...}, ... ], "nextCursor": null }
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parsing API response: %w", err)
	}

	workflows := make([]Workflow, 0, len(envelope.Data))
	for _, item := range envelope.Data {
		id, _ := item["id"].(string)
		name, _ := item["name"].(string)
		workflows = append(workflows, Workflow{
			ID:   id,
			Name: name,
			Raw:  item, // unmodified reference to the parsed object
		})
	}

	return workflows, nil
}
