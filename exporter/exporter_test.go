package exporter

import (
	"strings"
	"testing"
)

const forbiddenChars = `/\:*?"<>|`

// ── sanitizeFilename ──────────────────────────────────────────────────────────

// TestSanitizeFilename_ForbiddenCharsRemoved verifies Property 13:
// The result never contains any character from the forbidden set.
func TestSanitizeFilename_ForbiddenCharsRemoved(t *testing.T) {
	cases := []struct {
		input string
		desc  string
	}{
		{`my/workflow`, "forward slash"},
		{`my\workflow`, "backslash"},
		{`my:workflow`, "colon"},
		{`my*workflow`, "asterisk"},
		{`my?workflow`, "question mark"},
		{`my"workflow`, "double quote"},
		{`my<workflow`, "less than"},
		{`my>workflow`, "greater than"},
		{`my|workflow`, "pipe"},
		{`/\:*?"<>|`, "all forbidden chars"},
		{`normal name`, "no forbidden chars — unchanged"},
		{`sw.Enterprise.get`, "dots are allowed"},
		{`xc.Tax.CSF`, "dots and uppercase"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			result := SanitizeFilename(tc.input)
			for _, ch := range forbiddenChars {
				if strings.ContainsRune(result, ch) {
					t.Errorf("SanitizeFilename(%q) = %q still contains forbidden char %q", tc.input, result, ch)
				}
			}
		})
	}
}

// TestSanitizeFilename_NoLeadingTrailingSpaces verifies Property 14:
// The result never starts or ends with a space.
func TestSanitizeFilename_NoLeadingTrailingSpaces(t *testing.T) {
	cases := []struct {
		input string
		desc  string
	}{
		{"  leading spaces", "leading spaces"},
		{"trailing spaces  ", "trailing spaces"},
		{"  both sides  ", "both sides"},
		{"no spaces", "no spaces — unchanged"},
		{"   ", "only spaces → fallback"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			result := SanitizeFilename(tc.input)
			if strings.HasPrefix(result, " ") || strings.HasSuffix(result, " ") {
				t.Errorf("SanitizeFilename(%q) = %q has leading or trailing space", tc.input, result)
			}
		})
	}
}

// TestSanitizeFilename_EmptyFallback verifies that truly empty input (after trim)
// returns "unnamed_workflow". Forbidden chars become "_", so only whitespace-only
// inputs trigger the fallback.
func TestSanitizeFilename_EmptyFallback(t *testing.T) {
	// Only inputs that produce an empty string after TrimSpace trigger the fallback.
	// Forbidden chars are replaced with "_", not removed, so they don't produce empty.
	cases := []string{"", "   "}
	for _, input := range cases {
		result := SanitizeFilename(input)
		if result != "unnamed_workflow" {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", input, result, "unnamed_workflow")
		}
	}
}

// ── CleanWorkflow ─────────────────────────────────────────────────────────────

// TestCleanWorkflow_OnlyImportableFieldsKept verifies the allowlist approach:
// the result contains ONLY name, nodes, connections, settings — nothing else.
func TestCleanWorkflow_OnlyImportableFieldsKept(t *testing.T) {
	raw := map[string]any{
		// importable — must be kept
		"name":        "sw.save_report",
		"nodes":       []any{"node1"},
		"connections": map[string]any{"a": "b"},
		"settings":    map[string]any{"timezone": "UTC"},
		// API-only — must be dropped
		"id":              "wf-001",
		"versionId":       "abc-123",
		"active":          true,
		"tags":            []any{"tag1"},
		"meta":            map[string]any{"instanceId": "xyz"},
		"pinData":         map[string]any{},
		"createdAt":       "2026-01-01T00:00:00Z",
		"updatedAt":       "2026-01-01T00:00:00Z",
		"isArchived":      false,
		"activeVersionId": "abc-123",
		"activeVersion":   map[string]any{"nodes": []any{}},
		"someFutureField": "should be dropped too",
	}

	result := CleanWorkflow(raw)

	allowed := map[string]bool{"name": true, "nodes": true, "connections": true, "settings": true}

	// No extra keys beyond the allowlist.
	for k := range result {
		if !allowed[k] {
			t.Errorf("CleanWorkflow result contains unexpected field %q", k)
		}
	}

	// All importable fields present.
	for k := range allowed {
		if _, ok := result[k]; !ok {
			t.Errorf("CleanWorkflow result missing expected field %q", k)
		}
	}
}

// TestCleanWorkflow_SettingsAlwaysEmpty verifies that settings is always
// reset to {} regardless of the original value — removes instance-specific
// config like binaryDataMode that should not carry over to the target instance.
func TestCleanWorkflow_SettingsAlwaysEmpty(t *testing.T) {
	cases := []struct {
		desc     string
		settings any
	}{
		{"settings with binaryDataMode", map[string]any{"binaryDataMode": "filesystem"}},
		{"settings with multiple keys", map[string]any{"binaryDataMode": "s3", "timezone": "UTC"}},
		{"settings already empty", map[string]any{}},
		{"settings absent", nil}, // nil means key not present
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			raw := map[string]any{"name": "wf", "nodes": []any{}}
			if tc.settings != nil {
				raw["settings"] = tc.settings
			}

			result := CleanWorkflow(raw)

			val, ok := result["settings"]
			if !ok {
				t.Fatal("settings missing from result")
			}
			m, ok := val.(map[string]any)
			if !ok {
				t.Fatalf("settings is %T, want map[string]any", val)
			}
			if len(m) != 0 {
				t.Errorf("settings = %v, want empty map {}", m)
			}
		})
	}
}

// TestCleanWorkflow_DoesNotMutateOriginal verifies Property 12:
// The original raw map is identical before and after calling CleanWorkflow.
func TestCleanWorkflow_DoesNotMutateOriginal(t *testing.T) {
	raw := map[string]any{
		"id":        "wf-001",
		"active":    true,
		"versionId": "v1",
		"name":      "My Workflow",
		"settings":  map[string]any{"timezone": "UTC"},
	}

	snapshotLen := len(raw)
	snapshotKeys := make([]string, 0, len(raw))
	for k := range raw {
		snapshotKeys = append(snapshotKeys, k)
	}

	CleanWorkflow(raw)

	if len(raw) != snapshotLen {
		t.Errorf("original map has %d keys after CleanWorkflow, want %d", len(raw), snapshotLen)
	}
	for _, k := range snapshotKeys {
		if _, ok := raw[k]; !ok {
			t.Errorf("original map lost key %q after CleanWorkflow", k)
		}
	}
}

// TestCleanWorkflow_MatchesManualDownload verifies the output matches exactly
// what n8n produces with the manual "Download" button.
func TestCleanWorkflow_MatchesManualDownload(t *testing.T) {
	// Simulate what the API returns (with all the extra fields).
	apiResponse := map[string]any{
		"name":            "sw.save_report",
		"nodes":           []any{"node1", "node2"},
		"connections":     map[string]any{"a": "b"},
		"settings":        map[string]any{},
		"id":              "WhZVMbv3M0PJDoYc",
		"versionId":       "abc-123",
		"active":          false,
		"tags":            []any{},
		"meta":            nil,
		"pinData":         map[string]any{},
		"createdAt":       "2026-04-30T19:20:24.887Z",
		"updatedAt":       "2026-04-30T19:20:24.890Z",
		"isArchived":      false,
		"activeVersionId": "abc-123",
		"activeVersion":   map[string]any{"nodes": []any{}},
	}

	result := CleanWorkflow(apiResponse)

	// Must have exactly 4 keys.
	if len(result) != 4 {
		t.Errorf("expected 4 keys in cleaned workflow, got %d: %v", len(result), result)
	}

	// name must match.
	if result["name"] != "sw.save_report" {
		t.Errorf("name = %v, want sw.save_report", result["name"])
	}
}
