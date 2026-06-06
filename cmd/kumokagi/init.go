package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stlimtat/kumokagi/pkg/config"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a .kumokagi.yaml config file in the current directory",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().String("backend", "vault", "secrets backend (vault, aws, azure, gcp)")
	initCmd.Flags().String("app", "", "application name")
	initCmd.Flags().String("env", "prod", "default environment")
	initCmd.Flags().String("vault-addr", "https://vault.example.com", "Vault address")
	if err := initCmd.MarkFlagRequired("app"); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, _ []string) error {
	backend, _ := cmd.Flags().GetString("backend")
	app, _ := cmd.Flags().GetString("app")
	env, _ := cmd.Flags().GetString("env")
	vaultAddr, _ := cmd.Flags().GetString("vault-addr")

	cfg := config.Config{
		Backend: backend,
		Mount:   config.DefaultMount,
		App:     app,
		Env:     env,
		Keys:    []string{},
		Vault:   config.VaultConfig{Address: vaultAddr},
	}

	if _, err := os.Stat(config.FileName); err == nil {
		return fmt.Errorf("%s already exists", config.FileName)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(config.FileName, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created %s for app %q\n", config.FileName, app)
	return nil
}
