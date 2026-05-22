package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"forge/internal/config"
	appstore "forge/internal/gateway/store"
)

func init() {
	rootCmd.AddCommand(newAdminCmd())
}

func newAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Platform administration commands",
	}
	cmd.AddCommand(newAdminCreateTenantCmd())
	return cmd
}

func newAdminCreateTenantCmd() *cobra.Command {
	var configFile, tenantID, tenantName, adminUser, adminPass string
	cmd := &cobra.Command{
		Use:   "create-tenant",
		Short: "Create a new tenant",
		Long: `Create a new tenant. Use --admin-username and --admin-password to
bootstrap the first admin user in one step (required to log in via the web UI).`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if tenantID == "" {
				return fmt.Errorf("--id is required")
			}
			if (adminUser == "") != (adminPass == "") {
				return fmt.Errorf("--admin-username and --admin-password must be provided together")
			}

			cfg, err := config.Load(configFile)
			if err != nil {
				return err
			}
			ts, err := appstore.Open(cfg.Store.DriverOrDefault(), cfg.Store.Options)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer ts.Close()

			ctx := context.Background()
			name := tenantName
			if name == "" {
				name = tenantID
			}
			if err := ts.Tenants().Seed(ctx, &appstore.Tenant{
				ID:   tenantID,
				Name: name,
			}); err != nil {
				return fmt.Errorf("create tenant: %w", err)
			}
			fmt.Fprintf(os.Stdout, "tenant created: %s (%s)\n", tenantID, name)

			if adminUser != "" {
				hash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
				if err != nil {
					return fmt.Errorf("hash password: %w", err)
				}
				if err := ts.Users().Seed(ctx, &appstore.User{
					TenantID:     tenantID,
					Username:     adminUser,
					PasswordHash: string(hash),
					Role:         "admin",
				}); err != nil {
					return fmt.Errorf("create admin user: %w", err)
				}
				fmt.Fprintf(os.Stdout, "admin user created: %s\n", adminUser)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&configFile, "config", "c", "", "path to forge.yaml")
	cmd.Flags().StringVar(&tenantID, "id", "", "tenant ID (required)")
	cmd.Flags().StringVar(&tenantName, "name", "", "display name (defaults to ID)")
	cmd.Flags().StringVar(&adminUser, "admin-username", "", "bootstrap an initial admin user")
	cmd.Flags().StringVar(&adminPass, "admin-password", "", "password for the initial admin user")
	return cmd
}
