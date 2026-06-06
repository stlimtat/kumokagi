package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Create or update a secret value in the backend",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if err := loadConfig(ctx); err != nil {
			return err
		}
		if err := vaultClient.Set(ctx, provider.SecretPath{
			Mount: appCfg.Mount,
			Env:   appCfg.Env,
			App:   appCfg.App,
			Key:   args[0],
		}, args[1]); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Set %s/%s/%s/%s\n",
			appCfg.Mount, appCfg.Env, appCfg.App, args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setCmd)
}
