package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

var readCmd = &cobra.Command{
	Use:   "read <uri>",
	Short: "Fetch a secret by kumokagi:// URI (backend encoded in URI)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		backend, path, err := provider.ParseURI(args[0])
		if err != nil {
			return err
		}
		// URI backend takes precedence; set override before loadConfig validates.
		if backendOverride == "" {
			backendOverride = backend
		}
		if err := loadConfig(ctx); err != nil {
			return err
		}
		val, err := secretClient.Get(ctx, path)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), val)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(readCmd)
}
