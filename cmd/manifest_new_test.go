package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xrmcp/go-sdk/xrmcp"
)

func TestNormalizeManifestPathAppendsExtension(t *testing.T) {
	got, warning := normalizeManifestPath("./tools/my-tool")
	if got != "./tools/my-tool.xrmcp.json" {
		t.Fatalf("unexpected path: %q", got)
	}
	if warning != "" {
		t.Fatalf("expected no warning, got %q", warning)
	}
}

func TestNormalizeManifestPathCorrectsWrongExtension(t *testing.T) {
	got, warning := normalizeManifestPath("./tools/my-tool.json")
	if got != "./tools/my-tool.xrmcp.json" {
		t.Fatalf("unexpected path: %q", got)
	}
	if !strings.Contains(warning, "warning: expected .xrmcp.json extension") {
		t.Fatalf("expected warning, got %q", warning)
	}
}

func TestWriteManifestScaffoldProducesValidManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jira", "get_ticket.xrmcp.json")

	if err := writeManifestScaffold(path); err != nil {
		t.Fatalf("writeManifestScaffold: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	tool := payload["tool"].(map[string]any)
	if got := tool["schemaVersion"]; got != xrmcp.SpecVersion {
		t.Fatalf("unexpected schemaVersion: %#v", got)
	}
	if got := tool["name"]; got != "get_ticket" {
		t.Fatalf("unexpected name: %#v", got)
	}
	if got := tool["displayName"]; got != "Get Ticket" {
		t.Fatalf("unexpected displayName: %#v", got)
	}
	if got := tool["description"]; got != "TODO: add description" {
		t.Fatalf("unexpected description: %#v", got)
	}
	inputSchema := tool["inputSchema"].(map[string]any)
	properties := inputSchema["properties"].(map[string]any)
	if _, ok := properties["resourceId"]; !ok {
		t.Fatalf("expected resourceId input property, got %#v", properties)
	}
	outputSchema := tool["outputSchema"].(map[string]any)
	if got := outputSchema["type"]; got != "object" {
		t.Fatalf("expected object outputSchema, got %#v", got)
	}
	configSchema := tool["configSchema"].(map[string]any)
	configProperties := configSchema["properties"].(map[string]any)
	if _, ok := configProperties["baseUrl"]; !ok {
		t.Fatalf("expected baseUrl config property, got %#v", configProperties)
	}
	metadata := tool["metadata"].(map[string]any)
	if got := metadata["author"]; got != "xrMCP contributors" {
		t.Fatalf("unexpected metadata.author: %#v", got)
	}
	config := payload["config"].(map[string]any)
	if got := config["baseUrl"]; got != "api.example.com" {
		t.Fatalf("expected baseUrl config default, got %#v", got)
	}

	result := xrmcp.NewSchemaValidator().ValidateRegistration(payload)
	if !result.Valid {
		t.Fatalf("expected valid scaffold, got errors: %v", result.Errors)
	}
}

func TestWriteManifestScaffoldDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool.xrmcp.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := writeManifestScaffold(path)
	if err == nil {
		t.Fatal("expected overwrite protection error")
	}
}

func TestValidateManifestFileSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool.xrmcp.json")
	if err := writeManifestScaffold(path); err != nil {
		t.Fatalf("writeManifestScaffold: %v", err)
	}

	errors, err := validateManifestFile(path)
	if err != nil {
		t.Fatalf("validateManifestFile: %v", err)
	}
	if len(errors) != 0 {
		t.Fatalf("expected no validation errors, got %v", errors)
	}
}

func TestValidateManifestFileFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.xrmcp.json")
	if err := os.WriteFile(path, []byte(`{"tool":{"name":"bad"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	errors, err := validateManifestFile(path)
	if err != nil {
		t.Fatalf("validateManifestFile: %v", err)
	}
	if len(errors) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestRunManifestNewWarnsAndWritesCorrectedPath(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "tool.json")
	finalPath := filepath.Join(dir, "tool.xrmcp.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	manifestNewCmd.SetOut(&stdout)
	manifestNewCmd.SetErr(&stderr)

	if err := runManifestNew(manifestNewCmd, []string{input}); err != nil {
		t.Fatalf("runManifestNew: %v", err)
	}

	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("expected corrected file path to exist: %v", err)
	}
	if !strings.Contains(stderr.String(), "warning: expected .xrmcp.json extension") {
		t.Fatalf("expected warning, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Created: "+finalPath) {
		t.Fatalf("expected success output, got %q", stdout.String())
	}
}
