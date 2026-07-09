package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

var setCmd = &cobra.Command{
	Use:   "set <key> [value|-]",
	Short: "Create or update a secret value. Pass '-' or omit value to read from stdin.",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if err := loadConfig(ctx); err != nil {
			return err
		}
		var value string
		if len(args) == 2 && args[1] != "-" {
			value = args[1]
		} else {
			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			value = strings.TrimRight(string(raw), "\n")
		}
		if err := secretClient.Set(ctx, provider.SecretPath{
			Mount: appCfg.Mount,
			Env:   appCfg.Env,
			App:   appCfg.App,
			Key:   args[0],
		}, value); err != nil {
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
