package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stlimtat/kumokagi/internal/vault"
	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/vipersource"
)

var (
	cfgFile     string
	appCfg      *config.Config
	vaultClient *vault.Client
	source      *vipersource.Source
)

var rootCmd = &cobra.Command{
	Use:           "kumokagi",
	Short:         "Ephemeral secrets management for cloud infrastructure",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", config.FileName, "config file path")
	viper.SetEnvPrefix("KUMOKAGI")
	viper.AutomaticEnv()
}

// loadConfig is called by commands that need the provider (all except init).
func loadConfig(ctx context.Context) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	appCfg = cfg

	client, err := vault.New(ctx)
	if err != nil {
		return fmt.Errorf("connect to vault: %w", err)
	}
	vaultClient = client
	source = vipersource.New(client, cfg)
	return nil
}
