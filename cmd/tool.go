package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var toolURL string
var toolLsDesc bool
var toolToken string

const registryManifestBaseURL = "https://raw.githubusercontent.com/xrmcp/registry/main/xrmcp-registry/tools"

var toolCmd = &cobra.Command{
	Use:     "tool",
	Aliases: []string{"t"},
	Short:   "Manage xrMCP tools",
}

var toolLsCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List installed tools",
	RunE:    runToolLs,
}

var toolInstallCmd = &cobra.Command{
	Use:     "install <arg>",
	Aliases: []string{"i"},
	Short:   "Register a tool from a local manifest or registry identifier",
	Args:    cobra.ExactArgs(1),
	RunE:    runToolInstall,
}

var toolSearchCmd = &cobra.Command{
	Use:     "search <keyword>",
	Short:   "Search registry tools by keyword",
	Aliases: []string{"s"},
	Args:    cobra.MinimumNArgs(1),
	RunE:    runToolSearch,
}

var toolUninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "Uninstall a tool by name",
	Args:  cobra.ExactArgs(1),
	RunE:  runToolUninstall,
}

func init() {
	toolLsCmd.Flags().StringVar(&toolURL, "url", "", "xrMCP server base URL (default: XRMCP_SERVER_URL or http://localhost:7373)")
	toolLsCmd.Flags().BoolVarP(&toolLsDesc, "desc", "d", false, "Show description column")
	toolLsCmd.Flags().StringVar(&toolToken, "token", "", "Bearer token for protected xrMCP admin API endpoints")
	toolInstallCmd.Flags().StringVar(&toolURL, "url", "", "xrMCP server base URL (default: XRMCP_SERVER_URL or http://localhost:7373)")
	toolInstallCmd.Flags().StringVar(&toolToken, "token", "", "Bearer token for protected xrMCP admin API endpoints")
	toolUninstallCmd.Flags().StringVar(&toolURL, "url", "", "xrMCP server base URL (default: XRMCP_SERVER_URL or http://localhost:7373)")
	toolUninstallCmd.Flags().StringVar(&toolToken, "token", "", "Bearer token for protected xrMCP admin API endpoints")

	toolCmd.AddCommand(toolLsCmd, toolInstallCmd, toolSearchCmd, toolUninstallCmd)
	rootCmd.AddCommand(toolCmd)
}

type listResponse struct {
	Tools []struct {
		Name         string         `json:"name"`
		Type         string         `json:"type"`
		Description  string         `json:"description"`
		RegisteredAt string         `json:"registeredAt"`
		Metadata     map[string]any `json:"metadata"`
	} `json:"tools"`
}

type installManifest struct {
	Tool   map[string]any `json:"tool"`
	Config map[string]any `json:"config"`
}

type installResponse struct {
	Name   string   `json:"name"`
	Status string   `json:"status"`
	Errors []string `json:"errors"`
}

type installSource struct {
	Kind   string
	Label  string
	Target string
}

func printUnauthorizedToolError(token string) {
	if strings.TrimSpace(token) != "" {
		fmt.Fprintln(os.Stderr, "error: xrMCP server rejected the provided bearer token")
		fmt.Fprintln(os.Stderr, "hint: verify --token or XRMCP_API_TOKEN")
		return
	}
	fmt.Fprintln(os.Stderr, "error: xrMCP server requires a bearer token")
	fmt.Fprintln(os.Stderr, "hint: provide --token or set XRMCP_API_TOKEN")
}

func runToolLs(cmd *cobra.Command, args []string) error {
	url := serverURL(toolURL) + "/tools/list-installed"
	token := resolveAPIToken(toolToken)
	req, err := newAPIRequest(http.MethodGet, url, token, nil)
	if err != nil {
		log.Fatalf("failed to build request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusUnauthorized {
			printUnauthorizedToolError(token)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "error %d: %s\n", resp.StatusCode, body)
		os.Exit(1)
	}

	var result listResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(string(body))
		return nil
	}

	if len(result.Tools) == 0 {
		fmt.Println("No tools installed.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if toolLsDesc {
		fmt.Fprintln(w, "NAME\tAUTHOR\tREGISTERED AT\tDESCRIPTION")
	} else {
		fmt.Fprintln(w, "NAME\tAUTHOR\tREGISTERED AT")
	}

	for _, t := range result.Tools {
		author := "--"
		if t.Metadata != nil {
			if a, ok := t.Metadata["author"].(string); ok && a != "" {
				author = a
			}
		}

		registeredAt := t.RegisteredAt
		if ts, err := time.Parse(time.RFC3339, registeredAt); err == nil {
			registeredAt = ts.Local().Format("2006-01-02 15:04")
		}

		if toolLsDesc {
			desc := t.Description
			if desc == "" {
				desc = "--"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Name, author, registeredAt, desc)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\n", t.Name, author, registeredAt)
		}
	}
	w.Flush()
	return nil
}

func runToolInstall(cmd *cobra.Command, args []string) error {
	manifest, source, err := loadInstallManifest(args[0])
	if err != nil {
		log.Fatalf("cannot load manifest: %v", err)
	}

	if manifest.Tool == nil {
		log.Fatalf("manifest is missing the top-level tool block")
	}

	config, err := collectInstallConfig(manifest, source)
	if err != nil {
		log.Fatalf("cannot collect config: %v", err)
	}

	payload := map[string]any{
		"tool":   manifest.Tool,
		"config": config,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("cannot encode install payload: %v", err)
	}

	url := serverURL(toolURL) + "/tools/register"
	token := resolveAPIToken(toolToken)
	req, err := newAPIRequest(http.MethodPost, url, token, bytes.NewReader(data))
	if err != nil {
		log.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		printUnauthorizedToolError(token)
		os.Exit(1)
	}

	var result installResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(string(body))
		os.Exit(1)
	}

	switch result.Status {
	case "registered":
		fmt.Printf("✓ Installed: %s\n", result.Name)
	case "updated":
		fmt.Printf("✓ Updated: %s\n", result.Name)
	default:
		fmt.Fprintf(os.Stderr, "✗ Rejected: %s\n", result.Name)
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  • %s\n", e)
		}
		os.Exit(1)
	}
	return nil
}

func loadInstallManifest(arg string) (installManifest, installSource, error) {
	if strings.HasSuffix(arg, ".xrmcp.json") {
		data, err := os.ReadFile(arg)
		if err != nil {
			return installManifest{}, installSource{}, err
		}
		return decodeInstallManifest(data, installSource{
			Kind:   "local",
			Label:  "local file",
			Target: arg,
		})
	}

	parts := strings.Split(arg, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return installManifest{}, installSource{}, fmt.Errorf("registry install expects <category>/<tool_name>, got %q", arg)
	}

	manifestURL := fmt.Sprintf("%s/%s/%s.xrmcp.json", registryManifestBaseURL, parts[0], parts[1])
	resp, err := http.Get(manifestURL) //nolint:noctx
	if err != nil {
		return installManifest{}, installSource{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return installManifest{}, installSource{}, fmt.Errorf("registry fetch failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return decodeInstallManifest(body, installSource{
		Kind:   "registry",
		Label:  "registry",
		Target: arg,
	})
}

func decodeInstallManifest(data []byte, source installSource) (installManifest, installSource, error) {
	var manifest installManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return installManifest{}, installSource{}, err
	}
	return manifest, source, nil
}

func collectInstallConfig(manifest installManifest, source installSource) (map[string]any, error) {
	configSchema, _ := manifest.Tool["configSchema"].(map[string]any)
	properties, _ := configSchema["properties"].(map[string]any)
	if len(properties) == 0 {
		if manifest.Config != nil {
			return manifest.Config, nil
		}
		return map[string]any{}, nil
	}

	required := jsonStringSlice(configSchema["required"])
	defaults := manifest.Config
	if defaults == nil {
		defaults = map[string]any{}
	}

	interactive := canPromptInteractively()
	if interactive {
		printInstallHeader(manifest, source)
		printSecretsNote(manifest)
	}

	return promptConfigFields(properties, required, defaults, "", interactive)
}

func hasNestedProperties(prop map[string]any) bool {
	if schemaType(prop) != "object" {
		return false
	}
	return len(childProperties(prop)) > 0
}

func childProperties(prop map[string]any) map[string]any {
	properties, _ := prop["properties"].(map[string]any)
	return properties
}

func schemaType(prop map[string]any) string {
	if value, ok := prop["type"].(string); ok {
		return value
	}
	if values, ok := prop["type"].([]any); ok {
		for _, candidate := range values {
			value, ok := candidate.(string)
			if !ok || value == "null" {
				continue
			}
			return value
		}
	}
	return ""
}

func isSecretConfigField(name string, prop map[string]any) bool {
	if value, ok := prop["format"].(string); ok && strings.EqualFold(value, "password") {
		return true
	}
	if value, ok := prop["x-secret"].(bool); ok && value {
		return true
	}

	desc, _ := prop["description"].(string)
	text := strings.ToLower(name + " " + desc)
	for _, marker := range []string{"secret", "password", "token", "private key", "api key", "apikey"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func canPromptInteractively() bool {
	stdinInfo, err := os.Stdin.Stat()
	if err != nil || (stdinInfo.Mode()&os.ModeCharDevice) == 0 {
		return false
	}

	stdoutInfo, err := os.Stdout.Stat()
	if err != nil || (stdoutInfo.Mode()&os.ModeCharDevice) == 0 {
		return false
	}

	return true
}

func nestedString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func jsonStringSlice(value any) []string {
	rawValues, ok := value.([]any)
	if !ok {
		return nil
	}

	out := make([]string, 0, len(rawValues))
	for _, raw := range rawValues {
		text, ok := raw.(string)
		if !ok || text == "" {
			continue
		}
		out = append(out, text)
	}
	return out
}

func runToolUninstall(cmd *cobra.Command, args []string) error {
	name := args[0]
	url := serverURL(toolURL) + "/tools/" + name
	token := resolveAPIToken(toolToken)

	req, err := newAPIRequest(http.MethodDelete, url, token, nil)
	if err != nil {
		log.Fatalf("failed to build request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	switch resp.StatusCode {
	case http.StatusNoContent:
		fmt.Printf("✓ Uninstalled: %s\n", name)
	case http.StatusNotFound:
		fmt.Fprintf(os.Stderr, "✗ Not found: %s\n", name)
		os.Exit(1)
	case http.StatusUnauthorized:
		printUnauthorizedToolError(token)
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "✗ Unexpected status %d for: %s\n", resp.StatusCode, name)
		os.Exit(1)
	}
	return nil
}
