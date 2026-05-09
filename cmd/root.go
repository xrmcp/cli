package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "xrmcp",
	Short: "xrMCP — Extended Reality of MCP",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// serverURL returns the effective server base URL, resolving flag → env → default.
func serverURL(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv("XRMCP_SERVER_URL"); v != "" {
		return v
	}
	return "http://localhost:7373"
}
