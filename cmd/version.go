package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xrmcp/go-sdk/xrmcp"
)

var cliVersion = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show xrmcp version information",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		writeVersionInfo(cmd.OutOrStdout())
		return nil
	},
}

func effectiveCLIVersion() string {
	if strings.TrimSpace(cliVersion) == "" {
		return "dev"
	}
	return cliVersion
}

func versionInfo() string {
	return fmt.Sprintf(
		"spec: %s\nxrmcp/go-sdk: %s\nxrmcp/cli: %s\n",
		xrmcp.SpecVersion,
		xrmcp.SDKVersion,
		effectiveCLIVersion(),
	)
}

func writeVersionInfo(w io.Writer) {
	_, _ = io.WriteString(w, versionInfo())
}
