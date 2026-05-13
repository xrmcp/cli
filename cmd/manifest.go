package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	manifestFrom        string
	manifestIn          string
	manifestOut         string
	manifestBindingOnly bool
)

var manifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Generate xrMCP manifest files",
}

var manifestGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate xrMCP manifests from external sources",
	RunE:  runManifestGenerate,
}

func init() {
	manifestGenerateCmd.Flags().StringVarP(&manifestFrom, "from", "f", "", "Input source format (currently: postman)")
	manifestGenerateCmd.Flags().StringVarP(&manifestIn, "in", "i", "", "Path to the input file")
	manifestGenerateCmd.Flags().StringVarP(&manifestOut, "out", "o", "", "Output directory for generated manifests")
	manifestGenerateCmd.Flags().BoolVarP(&manifestBindingOnly, "binding-only", "b", false, "Inspect Postman bindings without generating manifest files")

	manifestGenerateCmd.MarkFlagRequired("from")
	manifestGenerateCmd.MarkFlagRequired("in")

	manifestCmd.AddCommand(manifestGenerateCmd)
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
