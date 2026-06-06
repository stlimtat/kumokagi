package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secret keys for this app in the backend",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		if err := loadConfig(ctx); err != nil {
			return err
		}
		keys, err := vaultClient.List(ctx, provider.SecretPath{
			Mount: appCfg.Mount,
			Env:   appCfg.Env,
			App:   appCfg.App,
		})
		if err != nil {
			return err
		}
		for _, k := range keys {
			fmt.Fprintln(cmd.OutOrStdout(), k)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
