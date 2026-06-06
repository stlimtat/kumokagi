package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Check all declared secrets exist in the backend (no values printed)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		if err := loadConfig(ctx); err != nil {
			return err
		}
		if err := source.Verify(ctx); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "All %d secrets verified\n", len(appCfg.Keys))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}
