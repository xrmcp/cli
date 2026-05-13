package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var versionFlag bool

var rootCmd = &cobra.Command{
	Use:           "xrmcp",
	Short:         "xrMCP — Extended Reality of MCP",
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if versionFlag {
			writeVersionInfo(cmd.OutOrStdout())
			return nil
		}
		return cmd.Help()
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Print version information")
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command.
func Execute() {
	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		if shouldShowUsage(err) {
			_ = cmd.Usage()
		}
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func shouldShowUsage(err error) bool {
	text := err.Error()
	patterns := []string{
		"unknown command",
		"unknown flag",
		"required flag(s)",
		"accepts 0 arg(s)",
		"accepts 1 arg(s)",
		"requires at least",
		"requires at most",
		"requires exactly",
		"argument",
	}
	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
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
