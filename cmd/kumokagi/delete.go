package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Permanently delete a secret from the backend",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if err := loadConfig(ctx); err != nil {
			return err
		}
		if err := secretClient.Delete(ctx, provider.SecretPath{
			Mount: appCfg.Mount,
			Env:   appCfg.Env,
			App:   appCfg.App,
			Key:   args[0],
		}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s/%s/%s/%s\n",
			appCfg.Mount, appCfg.Env, appCfg.App, args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
