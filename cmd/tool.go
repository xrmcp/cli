package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var toolURL string
var toolLsDesc bool

var toolCmd = &cobra.Command{
	Use:   "tool",
	Short: "Manage xrMCP tools",
}

var toolLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List installed tools",
	RunE:  runToolLs,
}

var toolInstallCmd = &cobra.Command{
	Use:   "install <manifest>",
	Short: "Register a tool from a manifest JSON file",
	Args:  cobra.ExactArgs(1),
	RunE:  runToolInstall,
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
	toolInstallCmd.Flags().StringVar(&toolURL, "url", "", "xrMCP server base URL (default: XRMCP_SERVER_URL or http://localhost:7373)")
	toolUninstallCmd.Flags().StringVar(&toolURL, "url", "", "xrMCP server base URL (default: XRMCP_SERVER_URL or http://localhost:7373)")

	toolCmd.AddCommand(toolLsCmd, toolInstallCmd, toolUninstallCmd)
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

func runToolLs(cmd *cobra.Command, args []string) error {
	url := serverURL(toolURL) + "/tools/list-installed"
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
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
	manifestPath := args[0]
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		log.Fatalf("cannot read manifest: %v", err)
	}

	url := serverURL(toolURL) + "/tools/register"
	resp, err := http.Post(url, "application/json", bytes.NewReader(data)) //nolint:noctx
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Name   string   `json:"name"`
		Status string   `json:"status"`
		Errors []string `json:"errors"`
	}
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

func runToolUninstall(cmd *cobra.Command, args []string) error {
	name := args[0]
	url := serverURL(toolURL) + "/tools/" + name

	req, err := http.NewRequest(http.MethodDelete, url, nil)
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
	default:
		fmt.Fprintf(os.Stderr, "✗ Unexpected status %d for: %s\n", resp.StatusCode, name)
		os.Exit(1)
	}
	return nil
}
