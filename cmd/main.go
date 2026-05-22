package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"forge/internal/config"
	"forge/internal/hands/sandboxinit"
	"forge/internal/orchestration"

	// Each drivers package activates all built-in implementations for its domain.
	// To add a new driver, add it to the relevant drivers package — not here.
	_ "forge/internal/gateway/session/drivers"
	_ "forge/internal/gateway/store/drivers"
	_ "forge/internal/hands/drivers"
	_ "forge/internal/memory/stores/drivers"
)

var rootCmd = &cobra.Command{
	Use:   "forge",
	Short: "AI coding agent daemon",
	Long:  "forge is a multi-tenant AI coding agent. Run 'forge serve' to start the HTTP API server.",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newToolServerCmd())
	rootCmd.AddCommand(newSandboxInitCmd())
}

// runOptions holds CLI flags for the serve command.
type runOptions struct {
	configFile string
}

// addRunFlags binds the common agent flags to cmd and stores them in opts.
func addRunFlags(cmd *cobra.Command, opts *runOptions) {
	cmd.Flags().StringVarP(&opts.configFile, "config", "c", "", "path to forge.yaml (env vars used as fallback)")
}

func newServeCmd() *cobra.Command {
	var opts runOptions
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the multi-tenant HTTP API server",
		RunE:  func(_ *cobra.Command, _ []string) error { return runServe(opts, addr) },
	}
	addRunFlags(cmd, &opts)
	cmd.Flags().StringVar(&addr, "addr", ":8080", "HTTP listen address")
	return cmd
}

func runServe(opts runOptions, addr string) error {
	ctx := context.Background()
	cfg, err := config.Load(opts.configFile)
	if err != nil {
		return err
	}
	return orchestration.Serve(ctx, cfg, addr)
}

func newSandboxInitCmd() *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:    "sandbox-init",
		Short:  "Install packages declared in FORGE_PACKAGES_SPEC (runs inside container)",
		Hidden: true,
		RunE:   func(_ *cobra.Command, _ []string) error { return sandboxinit.Run(workspace) },
	}
	cmd.Flags().StringVar(&workspace, "workspace", "/workspace", "workspace root directory")
	return cmd
}

