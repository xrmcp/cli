package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/xrmcp/go-sdk/xrmcp"
	_ "github.com/xrmcp/go-sdk/xrmcp/executors/all"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage the xrMCP server",
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the xrMCP runtime server",
	RunE:  runServerStart,
}

var (
	flagTransport string
	flagPort      int
	flagStore     string
	flagEnv       string
)

func init() {
	serverStartCmd.Flags().StringVarP(&flagTransport, "transport", "t", "stdio", "Transport mode: stdio or http")
	serverStartCmd.Flags().IntVarP(&flagPort, "port", "p", 7373, "Port to listen on")
	serverStartCmd.Flags().StringVarP(&flagStore, "store", "s", "", "Path to the tool registry JSON file")
	serverStartCmd.Flags().StringVar(&flagEnv, "env", ".env", "Path to a .env file to load")

	serverCmd.AddCommand(serverStartCmd)
	rootCmd.AddCommand(serverCmd)
}

func runServerStart(cmd *cobra.Command, args []string) error {
	// 1. Load .env file (silently skip if missing)
	if err := godotenv.Load(flagEnv); err != nil && !os.IsNotExist(err) {
		log.Printf("warning: could not load %s: %v", flagEnv, err)
	}

	// 2. Apply flags as env vars (flags take precedence over .env)
	if cmd.Flags().Changed("transport") || os.Getenv("XRMCP_TRANSPORT") == "" {
		os.Setenv("XRMCP_TRANSPORT", flagTransport)
	}
	if cmd.Flags().Changed("port") || os.Getenv("XRMCP_ADDR") == "" {
		os.Setenv("XRMCP_ADDR", fmt.Sprintf(":%d", flagPort))
	}
	if flagStore != "" {
		os.Setenv("XRMCP_STORE_PATH", flagStore)
	}

	// 3. Graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 4. Start server
	if err := xrmcp.NewXRMCPRuntime().StartServer(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
	return nil
}
