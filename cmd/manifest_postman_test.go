package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xrmcp/go-sdk/xrmcp"
)

func TestGeneratePostmanManifests(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "collection.json")
	outputDir := filepath.Join(t.TempDir(), "out")

	const collection = `{
  "info": {
    "name": "Acme Platform"
  },
  "variable": [
    { "key": "baseUrl", "value": "https://api.example.com", "type": "string" },
    { "key": "apiToken", "value": "super-secret-token-value", "type": "string" }
  ],
  "item": [
    {
      "name": "Users",
      "item": [
        {
          "name": "Get User",
          "request": {
            "method": "GET",
            "url": "{{baseUrl}}/users/:userId",
            "header": [
              { "key": "Authorization", "value": "Bearer {{apiToken}}" }
            ]
          }
        }
      ]
    },
    {
      "name": "Projects",
      "item": [
        {
          "name": "List Projects",
          "request": {
            "method": "GET",
            "description": "List projects for a workspace",
            "url": "{{baseUrl}}/projects?workspace={{workspaceId}}"
          }
        }
      ]
    }
  ]
}`

	if err := os.WriteFile(inputPath, []byte(collection), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	summary, err := generatePostmanManifests(inputPath, outputDir, false)
	if err != nil {
		t.Fatalf("generatePostmanManifests returned error: %v", err)
	}

	if summary.Generated != 2 || summary.Failed != 0 || summary.Discovered != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	firstPath := filepath.Join(outputDir, "users", "get_user.xrmcp.json")
	secondPath := filepath.Join(outputDir, "projects", "list_projects.xrmcp.json")

	for _, path := range []string{firstPath, secondPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated file %s: %v", path, err)
		}
	}

	firstData, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read first manifest: %v", err)
	}
	if strings.Contains(string(firstData), "super-secret-token-value") {
		t.Fatal("generated manifest leaked raw secret")
	}

	var payload map[string]any
	if err := json.Unmarshal(firstData, &payload); err != nil {
		t.Fatalf("decode generated manifest: %v", err)
	}

	validator := xrmcp.NewSchemaValidator()
	result := validator.ValidateRegistration(payload)
	if !result.Valid {
		t.Fatalf("generated manifest is not schema-valid: %v", result.Errors)
	}

	config, _ := payload["config"].(map[string]any)
	if got := config["baseUrl"]; got != "https://api.example.com" {
		t.Fatalf("expected config.baseUrl default, got %#v", got)
	}

	tool, _ := payload["tool"].(map[string]any)
	executions, _ := tool["executions"].([]any)
	request := executions[0].(map[string]any)["request"].(map[string]any)
	if got := request["url"]; got != "{{config.baseUrl}}/users/{{input.userId}}" {
		t.Fatalf("unexpected request.url: %#v", got)
	}

	headers, _ := request["header"].([]any)
	if len(headers) != 1 {
		t.Fatalf("expected one generated header, got %d", len(headers))
	}
	authValue := headers[0].(map[string]any)["value"]
	if authValue != "Bearer {{secrets.API_TOKEN}}" {
		t.Fatalf("unexpected Authorization header value: %#v", authValue)
	}

	permissions, _ := tool["permissions"].(map[string]any)
	secrets, _ := permissions["secrets"].([]any)
	if len(secrets) != 1 || secrets[0] != "API_TOKEN" {
		t.Fatalf("unexpected permissions.secrets: %#v", secrets)
	}
}

func TestGeneratePostmanManifestsFailsUnsupportedFileField(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "collection.json")
	outputDir := filepath.Join(t.TempDir(), "out")

	const collection = `{
  "info": { "name": "Uploads" },
  "item": [
    {
      "name": "Files",
      "item": [
        {
          "name": "Upload Asset",
          "request": {
            "method": "POST",
            "url": "https://api.example.com/assets",
            "body": {
              "mode": "formdata",
              "formdata": [
                { "key": "file", "type": "file", "value": "" }
              ]
            }
          }
        }
      ]
    }
  ]
}`

	if err := os.WriteFile(inputPath, []byte(collection), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	summary, err := generatePostmanManifests(inputPath, outputDir, false)
	if err == nil {
		t.Fatal("expected generation error for unsupported file upload")
	}
	if summary.Discovered != 1 || summary.Generated != 0 || summary.Failed != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Failures) != 1 || !strings.Contains(summary.Failures[0], "unsupported form-data file field") {
		t.Fatalf("unexpected failure list: %+v", summary.Failures)
	}
}

func TestAnalyzePostmanBindingsReport(t *testing.T) {
	t.Parallel()

	const collection = `{
  "info": { "name": "Acme Platform" },
  "variable": [
    { "key": "baseUrl", "value": "https://api.example.com", "type": "string" },
    { "key": "apiToken", "value": "super-secret-token-value", "type": "string" }
  ],
  "item": [
    {
      "name": "Users",
      "item": [
        {
          "name": "Get User",
          "request": {
            "method": "GET",
            "url": "{{baseUrl}}/users/:userId?workspace={{workspaceId}}",
            "header": [
              { "key": "Authorization", "value": "Bearer {{apiToken}}" }
            ]
          }
        }
      ]
    }
  ]
}`

	var parsed postmanCollection
	if err := json.Unmarshal([]byte(collection), &parsed); err != nil {
		t.Fatalf("decode collection: %v", err)
	}

	analysis := analyzePostmanBindings(parsed)
	if analysis.Discovered != 1 {
		t.Fatalf("expected 1 discovered request, got %d", analysis.Discovered)
	}

	baseURL := analysis.Records["baseUrl"]
	if baseURL == nil || baseURL.Classification != bindingClassConfig {
		t.Fatalf("expected baseUrl to be config, got %#v", baseURL)
	}
	apiToken := analysis.Records["apiToken"]
	if apiToken == nil || apiToken.Classification != bindingClassSecret {
		t.Fatalf("expected apiToken to be secret, got %#v", apiToken)
	}
	userID := analysis.Records["userId"]
	if userID == nil || userID.Classification != bindingClassInput {
		t.Fatalf("expected userId to be input, got %#v", userID)
	}

	report := renderBindingReport(analysis)
	for _, expected := range []string{
		"Binding: baseUrl",
		"Classified as: config",
		"Binding: apiToken",
		"Classified as: secret",
		"Binding: userId",
		"Classified as: input",
		"Binding: workspaceId",
		"Classified as: input",
		"Why:",
	} {
		if !strings.Contains(report, expected) {
			t.Fatalf("expected report to contain %q\n%s", expected, report)
		}
	}
}

func TestGeneratePostmanManifestsLiftsLiteralValues(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "collection.json")
	outputDir := filepath.Join(t.TempDir(), "out")

	const collection = `{
  "info": { "name": "Repository" },
  "item": [
    {
      "name": "Actions",
      "item": [
        {
          "name": "Add Action",
          "request": {
            "method": "POST",
            "url": "http://172.18.120.59:8083/repository/action?caseTypeCode=calf&profileCode=user",
            "auth": {
              "type": "basic",
              "basic": [
                { "key": "username", "value": "walter.bates", "type": "string" },
                { "key": "password", "value": "bpm", "type": "string" }
              ]
            },
            "body": {
              "mode": "raw",
              "raw": "{\n  \"code\": \"bloquer\",\n  \"label\": \"Bloquer un dossier\",\n  \"automatic\": false,\n  \"implementationVersion\": \"1.0\"\n}"
            }
          }
        }
      ]
    }
  ]
}`

	if err := os.WriteFile(inputPath, []byte(collection), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	summary, err := generatePostmanManifests(inputPath, outputDir, false)
	if err != nil {
		t.Fatalf("generatePostmanManifests returned error: %v", err)
	}
	if summary.Generated != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	manifestPath := filepath.Join(outputDir, "actions", "add_action.xrmcp.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read generated manifest: %v", err)
	}

	text := string(data)
	for _, forbidden := range []string{"\"bpm\""} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("generated manifest leaked literal %q\n%s", forbidden, text)
		}
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode generated manifest: %v", err)
	}

	validator := xrmcp.NewSchemaValidator()
	result := validator.ValidateRegistration(payload)
	if !result.Valid {
		t.Fatalf("generated manifest is not schema-valid: %v", result.Errors)
	}

	config, _ := payload["config"].(map[string]any)
	if got := config["baseUrl"]; got != "http://172.18.120.59:8083" {
		t.Fatalf("expected baseUrl config default, got %#v", got)
	}

	tool := payload["tool"].(map[string]any)
	configSchema := tool["configSchema"].(map[string]any)
	configProps := configSchema["properties"].(map[string]any)
	if _, ok := configProps["authUsername"]; !ok {
		t.Fatalf("expected authUsername in configSchema, got %#v", configProps)
	}

	inputSchema := tool["inputSchema"].(map[string]any)
	inputProps := inputSchema["properties"].(map[string]any)
	for _, expected := range []string{"caseTypeCode", "profileCode", "code", "label", "automatic", "implementationVersion"} {
		if _, ok := inputProps[expected]; !ok {
			t.Fatalf("expected %s in inputSchema, got %#v", expected, inputProps)
		}
	}

	executions := tool["executions"].([]any)
	request := executions[0].(map[string]any)["request"].(map[string]any)
	if got := request["url"]; got != "{{config.baseUrl}}/repository/action?caseTypeCode={{input.caseTypeCode}}&profileCode={{input.profileCode}}" {
		t.Fatalf("unexpected request.url: %#v", got)
	}

	auth := request["auth"].(map[string]any)
	basic := auth["basic"].([]any)
	if basic[0].(map[string]any)["value"] != "{{config.authUsername}}" {
		t.Fatalf("unexpected auth username binding: %#v", basic[0])
	}
	if basic[1].(map[string]any)["value"] != "{{secrets.PASSWORD}}" {
		t.Fatalf("unexpected auth password binding: %#v", basic[1])
	}

	body := request["body"].(map[string]any)
	raw := body["raw"].(string)
	for _, expected := range []string{
		`"code": "{{input.code}}"`,
		`"label": "{{input.label}}"`,
		`"automatic": {{input.automatic}}`,
		`"implementationVersion": "{{input.implementationVersion}}"`,
	} {
		if !strings.Contains(raw, expected) {
			t.Fatalf("expected raw body to contain %q\n%s", expected, raw)
		}
	}

	permissions := tool["permissions"].(map[string]any)
	secrets := permissions["secrets"].([]any)
	if len(secrets) != 1 || secrets[0] != "PASSWORD" {
		t.Fatalf("unexpected permissions.secrets: %#v", secrets)
	}
	network := permissions["network"].([]any)
	if len(network) != 1 || network[0] != "172.18.120.59" {
		t.Fatalf("unexpected permissions.network: %#v", network)
	}
}

func TestGeneratePostmanManifestsTracksLiteralAuthorizationHeaderSecret(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "collection.json")
	outputDir := filepath.Join(t.TempDir(), "out")

	const collection = `{
  "info": { "name": "Gateway" },
  "item": [
    {
      "name": "Req Utina APIM",
      "request": {
        "method": "GET",
        "url": "https://api.example.com/data",
        "header": [
          { "key": "Authorization", "value": "Bearer abcdefghijklmnop" }
        ]
      }
    }
  ]
}`

	if err := os.WriteFile(inputPath, []byte(collection), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	summary, err := generatePostmanManifests(inputPath, outputDir, false)
	if err != nil {
		t.Fatalf("generatePostmanManifests returned error: %v", err)
	}
	if summary.Generated != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	manifestPath := filepath.Join(outputDir, "req_utina_apim.xrmcp.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read generated manifest: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode generated manifest: %v", err)
	}

	validator := xrmcp.NewSchemaValidator()
	result := validator.ValidateRegistration(payload)
	if !result.Valid {
		t.Fatalf("generated manifest is not schema-valid: %v", result.Errors)
	}

	tool := payload["tool"].(map[string]any)
	permissions := tool["permissions"].(map[string]any)
	secrets := permissions["secrets"].([]any)
	if len(secrets) != 1 || secrets[0] != "AUTHORIZATION_TOKEN" {
		t.Fatalf("unexpected permissions.secrets: %#v", secrets)
	}
}
