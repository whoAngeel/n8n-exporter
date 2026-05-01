// Package exporter handles workflow cleaning, filename sanitization,
// output directory resolution, and writing JSON files to disk.
package exporter

import (
	"os"
	"path/filepath"
	"strings"
)

// importableFields are the only top-level fields n8n needs to import a workflow.
// This matches exactly what n8n produces when you use the manual "Download" button.
// Using an allowlist (vs a denylist) makes the output immune to new API fields
// that n8n might add in future versions.
var importableFields = []string{"name", "nodes", "connections", "settings"}

// ResolveOutputDir returns the absolute path to use for exports.
// If customPath is non-empty, it is used as-is (already absolute from credentials).
// Otherwise falls back to #n8n_workflows_original in the current working directory.
func ResolveOutputDir(customPath string) string {
	if customPath != "" {
		return customPath
	}
	base, err := os.Getwd()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "#n8n_workflows_original")
}

// SanitizeFilename returns a filename-safe version of name.
// Characters forbidden on Windows, macOS, and Linux are replaced with "_".
// Leading/trailing spaces are trimmed. Empty results fall back to "unnamed_workflow".
func SanitizeFilename(name string) string {
	const forbidden = `/\:*?"<>|`
	var b strings.Builder
	for _, r := range name {
		if strings.ContainsRune(forbidden, r) {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	result := strings.TrimSpace(b.String())
	if result == "" {
		return "unnamed_workflow"
	}
	return result
}

// CleanWorkflow extracts only the fields that n8n needs to import a workflow,
// producing output identical to n8n's manual "Download" button.
//
// Kept:    name, nodes, connections, settings
// Dropped: id, versionId, active, tags, meta, pinData, createdAt, updatedAt,
//
//	isArchived, activeVersionId, activeVersion, and any future API fields.
//
// settings is always included as {} when absent, matching the manual download format.
// The original map is never mutated.
func CleanWorkflow(raw map[string]any) map[string]any {
	cleaned := make(map[string]any, len(importableFields))

	for _, field := range importableFields {
		if v, ok := raw[field]; ok {
			cleaned[field] = v
		}
	}

	// Always reset settings to an empty object, regardless of the original value.
	// This removes instance-specific config (e.g. binaryDataMode) that should not
	// carry over to the target instance — matching the manual cleanup process.
	cleaned["settings"] = map[string]any{}

	return cleaned
}
