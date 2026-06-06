package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

var getCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Fetch a secret value from the backend",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if err := loadConfig(ctx); err != nil {
			return err
		}
		val, err := vaultClient.Get(ctx, provider.SecretPath{
			Mount: appCfg.Mount,
			Env:   appCfg.Env,
			App:   appCfg.App,
			Key:   args[0],
		})
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), val)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}
