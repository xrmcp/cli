package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/xrmcp/go-sdk/xrmcp"
)

type scaffoldManifest struct {
	Tool   scaffoldTool   `json:"tool"`
	Config map[string]any `json:"config"`
}

type scaffoldTool struct {
	SchemaVersion string               `json:"schemaVersion"`
	Name          string               `json:"name"`
	DisplayName   string               `json:"displayName"`
	Description   string               `json:"description"`
	Type          string               `json:"type"`
	InputSchema   scaffoldInputSchema  `json:"inputSchema"`
	OutputSchema  scaffoldJSONSchema   `json:"outputSchema"`
	ConfigSchema  scaffoldConfigSchema `json:"configSchema"`
	Executions    []scaffoldExecution  `json:"executions"`
	Metadata      scaffoldMetadata     `json:"metadata"`
}

type scaffoldInputSchema struct {
	Type                 string                      `json:"type"`
	Properties           map[string]scaffoldProperty `json:"properties"`
	Required             []string                    `json:"required"`
	AdditionalProperties bool                        `json:"additionalProperties"`
}

type scaffoldJSONSchema struct {
	Type string `json:"type"`
}

type scaffoldConfigSchema struct {
	Type       string                      `json:"type"`
	Properties map[string]scaffoldProperty `json:"properties"`
	Required   []string                    `json:"required"`
}

type scaffoldProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type scaffoldExecution struct {
	Type    string          `json:"type"`
	Request scaffoldRequest `json:"request"`
}

type scaffoldRequest struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

type scaffoldMetadata struct {
	Author string `json:"author"`
}

var (
	manifestFrom        string
	manifestIn          string
	manifestOut         string
	manifestBindingOnly bool
)

var manifestCmd = &cobra.Command{
	Use:     "manifest",
	Aliases: []string{"m"},
	Short:   "Create and generate xrMCP manifest files",
}

var manifestGenerateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"g", "gen"},
	Short:   "Generate xrMCP manifests from external sources",
	RunE:    runManifestGenerate,
}

var manifestNewCmd = &cobra.Command{
	Use:          "new <path>",
	Aliases:      []string{"n"},
	Short:        "Create a new xrMCP manifest scaffold",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runManifestNew,
}

var manifestValidateCmd = &cobra.Command{
	Use:     "validate <path>",
	Aliases: []string{"v"},
	Short:   "Validate a local xrMCP manifest file",
	Args:    cobra.ExactArgs(1),
	RunE:    runValidate,
}

func init() {
	manifestGenerateCmd.Flags().StringVarP(&manifestFrom, "from", "f", "", "Input source format (currently: postman)")
	manifestGenerateCmd.Flags().StringVarP(&manifestIn, "in", "i", "", "Path to the input file")
	manifestGenerateCmd.Flags().StringVarP(&manifestOut, "out", "o", "", "Output directory for generated manifests")
	manifestGenerateCmd.Flags().BoolVarP(&manifestBindingOnly, "binding-only", "b", false, "Inspect Postman bindings without generating manifest files")

	manifestGenerateCmd.MarkFlagRequired("from")
	manifestGenerateCmd.MarkFlagRequired("in")

	manifestCmd.AddCommand(manifestGenerateCmd, manifestNewCmd, manifestValidateCmd)
	rootCmd.AddCommand(manifestCmd)
}

func runManifestGenerate(cmd *cobra.Command, args []string) error {
	if !manifestBindingOnly && manifestOut == "" {
		return fmt.Errorf("--out is required unless --binding-only is used")
	}

	switch manifestFrom {
	case "postman":
		return runManifestGeneratePostman(manifestIn, manifestOut, manifestBindingOnly)
	default:
		return fmt.Errorf("unsupported --from value %q", manifestFrom)
	}
}

func runManifestNew(cmd *cobra.Command, args []string) error {
	finalPath, warning := normalizeManifestPath(args[0])
	if warning != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), warning)
	}

	if err := writeManifestScaffold(finalPath); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Created: %s\n", finalPath)
	return nil
}

func runValidate(cmd *cobra.Command, args []string) error {
	errors, err := validateManifestFile(args[0])
	if err != nil {
		return err
	}
	if len(errors) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "✗ Invalid: %s\n", args[0])
		for _, validationErr := range errors {
			fmt.Fprintf(cmd.ErrOrStderr(), "  • %s\n", validationErr)
		}
		return fmt.Errorf("manifest validation failed")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Valid: %s\n", args[0])
	return nil
}

func normalizeManifestPath(input string) (string, string) {
	if strings.HasSuffix(input, ".xrmcp.json") {
		return input, ""
	}

	ext := filepath.Ext(input)
	if ext == "" {
		return input + ".xrmcp.json", ""
	}

	finalPath := strings.TrimSuffix(input, ext) + ".xrmcp.json"
	return finalPath, fmt.Sprintf("warning: expected .xrmcp.json extension, writing %s instead", finalPath)
}

func manifestScaffoldPayload(path string) scaffoldManifest {
	name := strings.TrimSuffix(filepath.Base(path), ".xrmcp.json")
	displayName := humanizeManifestName(name)
	return scaffoldManifest{
		Tool: scaffoldTool{
			SchemaVersion: xrmcp.SpecVersion,
			Name:          name,
			DisplayName:   displayName,
			Description:   "TODO: add description",
			Type:          "api",
			InputSchema: scaffoldInputSchema{
				Type: "object",
				Properties: map[string]scaffoldProperty{
					"resourceId": {
						Type:        "string",
						Description: "TODO: describe the primary input value",
					},
				},
				Required:             []string{"resourceId"},
				AdditionalProperties: false,
			},
			OutputSchema: scaffoldJSONSchema{
				Type: "object",
			},
			ConfigSchema: scaffoldConfigSchema{
				Type: "object",
				Properties: map[string]scaffoldProperty{
					"baseUrl": {
						Type:        "string",
						Description: "Base URL or host for the target API, e.g. api.example.com",
					},
				},
				Required: []string{"baseUrl"},
			},
			Executions: []scaffoldExecution{
				{
					Type: "api",
					Request: scaffoldRequest{
						Method: "GET",
						URL:    "https://{{config.baseUrl}}/ressources/{{input.resourceId}}",
					},
				},
			},
			Metadata: scaffoldMetadata{
				Author: "xrMCP contributors",
			},
		},
		Config: map[string]any{
			"baseUrl": "api.example.com",
		},
	}
}

func humanizeManifestName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || unicode.IsSpace(r)
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		parts[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(parts, " ")
}

func writeManifestScaffold(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("manifest already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	payload := manifestScaffoldPayload(path)
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0o644)
}

func validateManifestFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	result := xrmcp.NewSchemaValidator().ValidateRegistration(payload)
	return result.Errors, nil
}
