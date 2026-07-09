package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/factory"
	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stlimtat/kumokagi/pkg/vipersource"
)

var (
	cfgFile         string
	backendOverride string
	appCfg          *config.Config
	secretClient    provider.Provider
	source          *vipersource.Source
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
	rootCmd.PersistentFlags().StringVar(&backendOverride, "backend", "", "override backend from config (vault|aws|azure|gcp|onepassword)")
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
	if backendOverride != "" {
		cfg.Backend = backendOverride
	}
	appCfg = cfg

	client, err := factory.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", cfg.Backend, err)
	}
	secretClient = client
	source = vipersource.New(client, cfg)
	return nil
}
